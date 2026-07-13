package serde

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func TestGeneratedSourceIsDeterministicAndCompilesMixedShapes(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeSerdeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/serdefixture\n\ngo 1.26.0\n")
	writeSerdeFixture(t, filepath.Join(dir, "model.go"), `package fixture

import "encoding/json"

//mgo:gen std.serde.jsonruntime
type Runtime struct{}

type NamedInt int64
type NamedString string

type MethodZero string

func (v MethodZero) IsZero() bool { return v == "zero" }

type CompositeZero struct {
	Values []int
}


//mgo:gen std.serde
type Child struct {
	Name string `+"`json:\"name\"`"+`
}

//mgo:gen std.serde
type Mixed struct {
	String    string         `+"`json:\"string\"`"+`
	Bool      bool           `+"`json:\"bool\"`"+`
	Int8      int8           `+"`json:\"int8\"`"+`
	Uint8     uint8          `+"`json:\"uint8\"`"+`
	Float32   float32        `+"`json:\"float32\"`"+`
	Named     NamedInt       `+"`json:\"named\"`"+`
	NamedPtr  *NamedInt      `+"`json:\"namedPtr\"`"+`
	NamedList []NamedString  `+"`json:\"namedList\"`"+`
	NamedMap  map[string]NamedInt `+"`json:\"namedMap\"`"+`
	SliceMap  map[string][]NamedInt `+"`json:\"sliceMap\"`"+`
	ArrayMap  map[string][2]NamedInt `+"`json:\"arrayMap\"`"+`
	NestedMap map[string]map[NamedString]NamedInt `+"`json:\"nestedMap\"`"+`
	ChildSliceMap map[string][]Child `+"`json:\"childSliceMap\"`"+`
	ChildArrayMap map[string][2]Child `+"`json:\"childArrayMap\"`"+`
	NestedChildMap map[string]map[string]*Child `+"`json:\"nestedChildMap\"`"+`
	NestedPointerMap map[string]**NamedInt `+"`json:\"nestedPointerMap\"`"+`
	PointerSliceMap map[string][]*NamedInt `+"`json:\"pointerSliceMap\"`"+`
	RawMap map[string]json.RawMessage `+"`json:\"rawMap\"`"+`
	Bytes     []byte         `+"`json:\"bytes\"`"+`
	Pointer   *int           `+"`json:\"pointer\"`"+`
	Slice     []int          `+"`json:\"slice\"`"+`
	Array     [2]int         `+"`json:\"array\"`"+`
	Map       map[string]int `+"`json:\"map\"`"+`
	Interface any            `+"`json:\"interface\"`"+`
	Child     Child          `+"`json:\"child\"`"+`
	Children  []Child        `+"`json:\"children\"`"+`
	ZeroText  string         `+"`json:\"zeroText,omitzero\"`"+`
	ZeroArray [2]int         `+"`json:\"zeroArray,omitzero\"`"+`
	ZeroCheck MethodZero     `+"`json:\"zeroCheck,omitzero\"`"+`
	ZeroValue CompositeZero  `+"`json:\"zeroValue,omitzero\"`"+`
}
`)

	runMetago := func() []byte {
		t.Helper()
		command := exec.Command("go", "run", root, dir)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("generate fixture: %v\n%s", err, output)
		}
		generated, err := os.ReadFile(filepath.Join(dir, "meta.go"))
		if err != nil {
			t.Fatal(err)
		}
		return generated
	}

	first := runMetago()
	second := runMetago()
	if !bytes.Equal(first, second) {
		t.Fatal("identical serde inputs produced different generated source")
	}

	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated mixed-shape fixture does not compile: %v\n%s", err, output)
	}
}

func TestGeneratedImportsAreMinimalAndStable(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	const runtimeImport = "example.com/importfixture/runtime"
	tests := []struct {
		name    string
		model   string
		imports []string
	}{
		{"empty", "//mgo:gen std.serde\ntype Value struct{}\n", []string{"fmt", runtimeImport}},
		{"string_only", "//mgo:gen std.serde\ntype Value struct { Name string `json:\"name\"` }\n", []string{"fmt", runtimeImport, "strings"}},
		{"float_only", "//mgo:gen std.serde\ntype Value struct { Number float64 `json:\"number\"` }\n", []string{"fmt", runtimeImport, "strings"}},
		{"named_uint8_slice", "type Byte uint8\n\n//mgo:gen std.serde\ntype Value struct { Values []Byte `json:\"values\"` }\n", []string{"fmt", runtimeImport, "strings"}},
		{"bool_and_integer", "//mgo:gen std.serde\ntype Value struct {\nEnabled bool `json:\"enabled\"`\nCount int `json:\"count\"`\n}\n", []string{"fmt", runtimeImport, "strconv", "strings"}},
		{"string_map", "//mgo:gen std.serde\ntype Value struct { Values map[string]string `json:\"values\"` }\n", []string{"fmt", "maps", runtimeImport, "sort", "strings"}},
		{"numeric_map", "//mgo:gen std.serde\ntype Value struct { Values map[string]int8 `json:\"values\"` }\n", []string{"fmt", "maps", runtimeImport, "sort", "strconv", "strings"}},
		{"byte_map", "//mgo:gen std.serde\ntype Value struct { Values map[string][]byte `json:\"values\"` }\n", []string{"fmt", "maps", runtimeImport, "sort", "strings"}},
		{"named_uint8_map", "type Byte uint8\n\n//mgo:gen std.serde\ntype Value struct { Values map[string][]Byte `json:\"values\"` }\n", []string{"fmt", "maps", runtimeImport, "sort", "strings"}},
		{"unsupported_fallback", "import \"time\"\n\n//mgo:gen std.serde\ntype Value struct { Time time.Time `json:\"time\"` }\n", []string{"encoding/json", "fmt", runtimeImport, "strings"}},
		{"embedded_fallback", "type Base struct { Name string `json:\"name\"` }\n\n//mgo:gen std.serde\ntype Value struct { Base }\n", []string{"encoding/json", "fmt", runtimeImport}},
		{"strict_string", "//mgo:gen std.serde strict=true\ntype Value struct { Name string `json:\"name\"` }\n", []string{"fmt", runtimeImport, "strconv", "strings"}},
		{"configured_limits", "//mgo:gen std.serde maxinput=64 maxdepth=8\ntype Value struct{}\n", []string{"fmt", runtimeImport}},
		{"raw_message", "import \"encoding/json\"\n\n//mgo:gen std.serde\ntype Value struct { Raw json.RawMessage `json:\"raw\"` }\n", []string{"fmt", runtimeImport, "strings"}},
		{"composite_omitzero", "type Composite struct { Values []int }\n\n//mgo:gen std.serde\ntype Value struct { Composite Composite `json:\"composite,omitzero\"` }\n", []string{"encoding/json", "fmt", runtimeImport, "strings"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSerdeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/importfixture\n\ngo 1.26.0\n")
			writeSerdeFixture(t, filepath.Join(dir, "metago.toml"), "[templates.\"std.serde\".args]\nruntime = \""+runtimeImport+"\"\n")
			writeSerdeFixture(t, filepath.Join(dir, "model.go"), "package fixture\n\n"+tc.model)
			writeSerdeFixture(t, filepath.Join(dir, "runtime", "model.go"), "package runtime\n\n//mgo:gen std.serde.jsonruntime\ntype Runtime struct{}\n")

			command := exec.Command("go", "run", root, dir)
			command.Dir = root
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("generate import fixture: %v\n%s", err, output)
			}
			command = exec.Command("go", "test", "./...")
			command.Dir = dir
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("generated import fixture does not compile: %v\n%s", err, output)
			}

			got := generatedImportPaths(t, filepath.Join(dir, "meta.go"))
			want := append([]string(nil), tc.imports...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("generated imports = %q, want %q", got, want)
			}
		})
	}
}

func generatedImportPaths(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		imports = append(imports, path)
	}
	sort.Strings(imports)
	return imports
}

func TestInvalidStrictArgumentFailsGeneration(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeSerdeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/strictfixture\n\ngo 1.26.0\n")
	writeSerdeFixture(t, filepath.Join(dir, "model.go"), `package fixture

//mgo:gen std.serde.jsonruntime
type Runtime struct{}

//mgo:gen std.serde strict=maybe
type Value struct {
	Name string `+"`json:\"name\"`"+`
}

`)
	command := exec.Command("go", "run", root, dir)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("invalid strict argument generated successfully")
	}
	if !bytes.Contains(output, []byte("std.serde strict must be true or false")) {
		t.Fatalf("invalid strict error missing actionable message:\n%s", output)
	}
}

func TestInvalidLimitArgumentsFailGeneration(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		argument  string
		wantError string
	}{
		{"negative_input", "maxinput=-1", "maxinput must be an unsigned 64-bit integer"},
		{"invalid_input", "maxinput=12x", "maxinput must be an unsigned 64-bit integer"},
		{"overflow_input", "maxinput=18446744073709551616", "maxinput must be an unsigned 64-bit integer"},
		{"negative_depth", "maxdepth=-1", "maxdepth must be an unsigned 64-bit integer"},
		{"invalid_depth", "maxdepth=deep", "maxdepth must be an unsigned 64-bit integer"},
		{"overflow_depth", "maxdepth=18446744073709551616", "maxdepth must be an unsigned 64-bit integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSerdeFixture(t, filepath.Join(dir, "go.mod"), "module example.com/limitfixture\n\ngo 1.26.0\n")
			writeSerdeFixture(t, filepath.Join(dir, "model.go"), `package fixture

//mgo:gen std.serde.jsonruntime
type Runtime struct{}

//mgo:gen std.serde `+tc.argument+`
type Value struct {
	Name string `+"`json:\"name\"`"+`
}
`)
			command := exec.Command("go", "run", root, dir)
			command.Dir = root
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("invalid limit argument %q generated successfully", tc.argument)
			}
			if !bytes.Contains(output, []byte(tc.wantError)) {
				t.Fatalf("invalid limit error missing %q:\n%s", tc.wantError, output)
			}
		})
	}
}

func writeSerdeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
