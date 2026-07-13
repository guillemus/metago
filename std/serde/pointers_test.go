package serde

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeScalarNestedAndGeneratedPointers(t *testing.T) {
	integer := 42
	inner := &integer
	text := "value"
	unsigned := uint8(math.MaxUint8)
	float := math.SmallestNonzeroFloat64
	want := CompatibilityValues{
		Pointer:       &integer,
		Nested:        &inner,
		StringPointer: &text,
		Uint8Pointer:  &unsigned,
		FloatPointer:  &float,
		NestedStruct:  &CompatibilityTagBehavior{Renamed: "nested", Named: "value"},
	}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Pointer, want.Pointer) ||
		!reflect.DeepEqual(got.Nested, want.Nested) ||
		!reflect.DeepEqual(got.StringPointer, want.StringPointer) ||
		!reflect.DeepEqual(got.Uint8Pointer, want.Uint8Pointer) ||
		!reflect.DeepEqual(got.FloatPointer, want.FloatPointer) ||
		!reflect.DeepEqual(got.NestedStruct, want.NestedStruct) {
		t.Fatalf("pointer round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativePointersRejectInvalidValuesAtomically(t *testing.T) {
	integer := 7
	inner := &integer
	text := "keep"
	unsigned := uint8(8)
	float := 9.0
	before := CompatibilityValues{
		Pointer:       &integer,
		Nested:        &inner,
		StringPointer: &text,
		Uint8Pointer:  &unsigned,
		FloatPointer:  &float,
		NestedStruct:  &CompatibilityTagBehavior{Renamed: "keep"},
	}
	for _, input := range []string{
		`{"uint8Pointer":256}`,
		`{"nested":"wrong"}`,
		`{"nestedStruct":{"renamed":"changed"},"floatPointer":"wrong"}`,
	} {
		value := clonePointerFixture(before)
		if err := value.UnmarshalJSON([]byte(input)); err == nil {
			t.Fatalf("invalid pointer value accepted: %s", input)
		}
		if !reflect.DeepEqual(value, before) {
			t.Fatalf("pointer failure changed receiver:\n got: %#v\nwant: %#v", value, before)
		}
	}

	for _, value := range []CompatibilityValues{
		{FloatPointer: new(math.NaN())},
		{FloatPointer: new(math.Inf(1))},
	} {
		if data, err := value.MarshalJSON(); err == nil || data != nil {
			t.Fatalf("non-finite pointer encoded as %q without error", data)
		}
	}
}

func TestSupportedPointersUseGeneratedPaths(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"decoded := int(l.Int(strconv.IntSize))",
		"inner := &decoded",
		"v.Nested = &inner",
		"decoded := new(CompatibilityTagBehavior)",
		"decoded.unmarshalJSONLexer(l)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated pointer path missing %q", want)
		}
	}
	start := strings.Index(generated, "func (v CompatibilityValues) appendJSONState")
	end := strings.Index(generated[start:], "// MarshalJSON implements json.Marshaler for CompatibilityTagBehavior")
	if start < 0 || end < 0 {
		t.Fatal("generated CompatibilityValues source block not found")
	}
	compatibilitySource := generated[start : start+end]
	for _, unwanted := range []string{
		"json.Marshal(v.Pointer)", "json.Unmarshal(raw, &v.Pointer)",
		"json.Marshal(v.Nested)", "json.Unmarshal(raw, &v.Nested)",
		"json.Marshal(v.NestedStruct)", "json.Unmarshal(raw, &v.NestedStruct)",
	} {
		if strings.Contains(compatibilitySource, unwanted) {
			t.Errorf("supported pointer unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}

func clonePointerFixture(value CompatibilityValues) CompatibilityValues {
	clone := value
	if value.Pointer != nil {
		v := *value.Pointer
		clone.Pointer = &v
	}
	if value.Nested != nil && *value.Nested != nil {
		v := **value.Nested
		inner := &v
		clone.Nested = &inner
	}
	if value.StringPointer != nil {
		v := *value.StringPointer
		clone.StringPointer = &v
	}
	if value.Uint8Pointer != nil {
		v := *value.Uint8Pointer
		clone.Uint8Pointer = &v
	}
	if value.FloatPointer != nil {
		v := *value.FloatPointer
		clone.FloatPointer = &v
	}
	if value.NestedStruct != nil {
		v := *value.NestedStruct
		clone.NestedStruct = &v
	}
	return clone
}
