package serde

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCompleteJSONTestSuiteAcceptedRejectedCorpus(t *testing.T) {
	subjects := loadJSONTestSuiteSubjects(t)
	accepted, rejected := 0, 0
	for _, subject := range subjects {
		subject := subject
		t.Run(subject.name, func(t *testing.T) {
			adapted := make([]byte, 0, len(subject.data)+16)
			adapted = append(adapted, `{"interface":`...)
			adapted = append(adapted, subject.data...)
			adapted = append(adapted, '}')

			var got CompatibilityValues
			gotErr := got.UnmarshalJSON(adapted)
			var want struct {
				Interface any `json:"interface"`
			}
			wantErr := json.Unmarshal(adapted, &want)

			switch subject.name[0] {
			case 'y':
				if gotErr != nil || wantErr != nil {
					t.Fatalf("accepted subject rejected: serde=%v encoding/json=%v", gotErr, wantErr)
				}
				if !reflect.DeepEqual(got.Interface, want.Interface) {
					t.Fatalf("accepted value differs:\n serde: %#v\nencoding/json: %#v", got.Interface, want.Interface)
				}
			case 'n':
				if gotErr == nil || wantErr == nil {
					t.Fatalf("rejected subject acceptance differs: serde=%v encoding/json=%v", gotErr, wantErr)
				}
			default:
				t.Fatalf("unexpected corpus subject %q", subject.name)
			}
		})
		if subject.name[0] == 'y' {
			accepted++
		} else {
			rejected++
		}
	}
	if accepted != 95 || rejected != 188 {
		t.Fatalf("pinned corpus count changed: accepted=%d rejected=%d", accepted, rejected)
	}
}

type jsonTestSuiteSubject struct {
	name string
	data []byte
}

func loadJSONTestSuiteSubjects(t *testing.T) []jsonTestSuiteSubject {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join("testdata", "JSONTestSuite-yn.tar.gz.base64"))
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(compressed)
	if got, want := hex.EncodeToString(digest[:]), "165b1ef91bb128d8fec3ffa329f171795e12fc6e3c2609be05257d92ad68b320"; got != want {
		t.Fatalf("pinned JSONTestSuite archive digest = %s, want %s", got, want)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	var subjects []jsonTestSuiteSubject
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg || (!strings.HasPrefix(header.Name, "y_") && !strings.HasPrefix(header.Name, "n_")) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		subjects = append(subjects, jsonTestSuiteSubject{filepath.Base(header.Name), data})
	}
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].name < subjects[j].name })
	return subjects
}

func TestCuratedAcceptedCorpus(t *testing.T) {
	for _, path := range curatedCorpusFiles(t, "accepted") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			var value CompatibilityValues
			if err := value.UnmarshalJSON(data); err != nil {
				t.Fatalf("accepted corpus subject was rejected: %v", err)
			}
		})
	}
}

func TestCuratedRejectedCorpus(t *testing.T) {
	for _, path := range curatedCorpusFiles(t, "rejected") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			var value CompatibilityValues
			if err := value.UnmarshalJSON(data); err == nil {
				t.Fatal("rejected corpus subject was accepted")
			}
		})
	}
}

func TestCuratedAmbiguousCorpusPolicy(t *testing.T) {
	for _, path := range curatedCorpusFiles(t, "ambiguous") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			var value CompatibilityNumbers
			err = value.UnmarshalJSON(data)
			name := filepath.Base(path)
			switch {
			case strings.HasPrefix(name, "accept_"):
				if err != nil {
					t.Fatalf("policy accepts subject, but decoder rejected it: %v", err)
				}
				if value.Float64 != 0 {
					t.Fatalf("finite exponent underflow = %v, want 0", value.Float64)
				}
			case strings.HasPrefix(name, "reject_"):
				if err == nil {
					t.Fatal("policy rejects subject, but decoder accepted it")
				}
			default:
				t.Fatalf("ambiguous corpus fixture must start with accept_ or reject_: %s", name)
			}
		})
	}
}

func curatedCorpusFiles(t *testing.T, category string) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", category, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("curated corpus category %q has no fixtures", category)
	}
	return paths
}
