package serde

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRawMessageUsesGeneratedValidatedPath(t *testing.T) {
	values := []json.RawMessage{
		nil,
		json.RawMessage("null"),
		json.RawMessage(` {"nested":[true,false,null,"<&>"]} `),
	}
	for _, raw := range values {
		value := CompatibilityValues{Raw: raw}
		data, err := value.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(data) {
			t.Fatalf("raw message produced invalid containing JSON: %s", data)
		}
		type plain CompatibilityValues
		standard, err := json.Marshal(plain(value))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, standard) {
			t.Fatalf("RawMessage encoding differs from encoding/json:\n got: %s\nwant: %s", data, standard)
		}
		var decoded CompatibilityValues
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatal(err)
		}
		want := raw
		if want == nil {
			want = json.RawMessage("null")
		}
		if !json.Valid(decoded.Raw) || !jsonSemanticallyEqual(decoded.Raw, want) {
			t.Fatalf("raw message round trip mismatch:\n got: %s\nwant: %s", decoded.Raw, want)
		}
	}

	for _, raw := range []json.RawMessage{json.RawMessage(""), json.RawMessage("{"), json.RawMessage("true false")} {
		if data, err := (CompatibilityValues{Raw: raw}).MarshalJSON(); err == nil || data != nil {
			t.Fatalf("invalid raw message encoded as %q without error", data)
		}
	}

	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"serdejsonruntime.AppendRaw(b, v.Raw)",
		"v.Raw = append(v.Raw[:0:0], raw...)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated RawMessage path missing %q", want)
		}
	}
	for _, unwanted := range []string{"json.Marshal(v.Raw)", "json.Unmarshal(raw, &v.Raw)"} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("RawMessage unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}

// Covers json-iterator/go's raw-message ownership regression; see testdata/PROVENANCE.md.
func TestRawMessageDecodeCopiesAndIsAtomic(t *testing.T) {
	input := []byte(`{"raw":{"copy":true}}`)
	var value CompatibilityValues
	if err := value.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}
	copy(input, bytes.Repeat([]byte{' '}, len(input)))
	if string(value.Raw) != `{"copy":true}` {
		t.Fatalf("decoded RawMessage aliases input: %s", value.Raw)
	}

	before := CompatibilityValues{Raw: json.RawMessage(`{"keep":true}`)}
	value = CompatibilityValues{Raw: append(json.RawMessage(nil), before.Raw...)}
	if err := value.UnmarshalJSON([]byte(`{"raw":{"changed":true},"bool":"wrong"}`)); err == nil {
		t.Fatal("wrong later field kind accepted")
	}
	if !bytes.Equal(value.Raw, before.Raw) {
		t.Fatalf("failed decode changed RawMessage: %s", value.Raw)
	}
}

func jsonSemanticallyEqual(a, b []byte) bool {
	var av, bv any
	return json.Unmarshal(a, &av) == nil && json.Unmarshal(b, &bv) == nil && valuesEqual(av, bv)
}

func valuesEqual(a, b any) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	return err == nil && bytes.Equal(ab, bb)
}
