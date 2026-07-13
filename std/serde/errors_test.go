package serde

import (
	"strings"
	"testing"
)

func TestWrongJSONKindsReportRootFieldAndOffset(t *testing.T) {
	tests := []struct {
		name   string
		decode func([]byte) error
		input  string
		root   string
		field  string
		goType string
		kind   string
	}{
		{"string", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"string":1}`, "CompatibilityValues", "string", "string", "number"},
		{"bool", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"bool":"true"}`, "CompatibilityValues", "bool", "bool", "string"},
		{"integer", func(data []byte) error { return new(CompatibilityNumbers).UnmarshalJSON(data) }, `{"int64":true}`, "CompatibilityNumbers", "int64", "int64", "boolean"},
		{"float", func(data []byte) error { return new(CompatibilityNumbers).UnmarshalJSON(data) }, `{"float64":[]}`, "CompatibilityNumbers", "float64", "float64", "array"},
		{"pointer", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"pointer":{}}`, "CompatibilityValues", "pointer", "*int", "object"},
		{"slice", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"slice":{}}`, "CompatibilityValues", "slice", "[]int", "object"},
		{"array", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"array":{}}`, "CompatibilityValues", "array", "[3]int", "object"},
		{"bytes", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"bytes":{}}`, "CompatibilityValues", "bytes", "[]byte", "object"},
		{"map", func(data []byte) error { return new(CompatibilityValues).UnmarshalJSON(data) }, `{"map":[]}`, "CompatibilityValues", "map", "map[string]int", "array"},
		{"quoted integer", func(data []byte) error { return new(CompatibilityTagBehavior).UnmarshalJSON(data) }, `{"quotedInt":42}`, "CompatibilityTagBehavior", "quotedInt", "int", "number"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode([]byte(tc.input))
			if err == nil {
				t.Fatalf("wrong JSON kind accepted: %s", tc.input)
			}
			message := err.Error()
			for _, want := range []string{tc.root, `field "` + tc.field + `"`, "Go type " + tc.goType, "JSON " + tc.kind, "offset"} {
				if !strings.Contains(message, want) {
					t.Fatalf("error %q does not contain %q", message, want)
				}
			}
		})
	}
}

func TestNestedGeneratedErrorReportsFullFieldPath(t *testing.T) {
	err := new(User).UnmarshalJSON([]byte(`{"address":{"city":42}}`))
	if err == nil {
		t.Fatal("wrong nested field kind accepted")
	}
	message := err.Error()
	for _, want := range []string{"User", `field "address"`, `field "city"`, "Go type string", "JSON number", "offset"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}
