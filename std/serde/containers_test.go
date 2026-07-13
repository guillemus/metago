package serde

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeNarrowScalarContainers(t *testing.T) {
	want := CompatibilityValues{
		Int8Slice: []int8{math.MinInt8, 0, math.MaxInt8},
		Int8Map:   map[string]int8{"min": math.MinInt8, "max": math.MaxInt8},
		Uint8Map:  map[string]uint8{"zero": 0, "max": math.MaxUint8},
	}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Int8Slice, want.Int8Slice) ||
		!reflect.DeepEqual(got.Int8Map, want.Int8Map) ||
		!reflect.DeepEqual(got.Uint8Map, want.Uint8Map) {
		t.Fatalf("narrow container round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNullScalarContainerElementsMatchEncodingJSON(t *testing.T) {
	input := []byte(`{
		"slice":[null,1],
		"int8Array":[null,2],
		"map":{"zero":null,"one":1},
		"namedStringSlice":[null,"value"],
		"namedUintArray":[null,2],
		"namedFloatMap":{"zero":null,"one":1.5}
	}`)

	var got CompatibilityValues
	if err := got.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}
	type plain CompatibilityValues
	var want plain
	if err := json.Unmarshal(input, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Slice, want.Slice) ||
		got.Int8Array != want.Int8Array ||
		!reflect.DeepEqual(got.Map, want.Map) ||
		!reflect.DeepEqual(got.NamedStringSlice, want.NamedStringSlice) ||
		got.NamedUintArray != want.NamedUintArray ||
		!reflect.DeepEqual(got.NamedFloatMap, want.NamedFloatMap) {
		t.Fatalf("null scalar container semantics differ:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativeNarrowScalarContainersRejectOverflowAtomically(t *testing.T) {
	tests := []string{
		`{"int8Slice":[128]}`,
		`{"int8Slice":[-129]}`,
		`{"int8Map":{"bad":128}}`,
		`{"int8Map":{"bad":-129}}`,
		`{"uint8Map":{"bad":-1}}`,
		`{"uint8Map":{"bad":256}}`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			before := CompatibilityValues{
				Int8Slice: []int8{7},
				Int8Map:   map[string]int8{"keep": 7},
				Uint8Map:  map[string]uint8{"keep": 7},
			}
			value := CompatibilityValues{
				Int8Slice: append([]int8(nil), before.Int8Slice...),
				Int8Map:   map[string]int8{"keep": 7},
				Uint8Map:  map[string]uint8{"keep": 7},
			}
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("overflow accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("overflow changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

func TestNativeFloatContainersRejectNonFiniteValues(t *testing.T) {
	values := []CompatibilityValues{
		{Float32Slice: []float32{float32(math.NaN())}},
		{Float32Slice: []float32{float32(math.Inf(1))}},
		{Float64Map: map[string]float64{"bad": math.Inf(-1)}},
	}
	for _, value := range values {
		if data, err := value.MarshalJSON(); err == nil || data != nil {
			t.Fatalf("non-finite container value encoded as %q without error", data)
		}
	}
}

func TestNarrowScalarContainersUseGeneratedPaths(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"v.Int8Slice = append(v.Int8Slice, int8(l.Int(8)))",
		"v.Int8Map[mk] = int8(l.Int(8))",
		"v.Uint8Map[mk] = uint8(l.Uint(8))",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated narrow-container path missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"json.Marshal(v.Int8Map)", "json.Unmarshal(raw, &v.Int8Map)",
		"json.Marshal(v.Uint8Map)", "json.Unmarshal(raw, &v.Uint8Map)",
	} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("narrow container unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}

func TestDuplicateContainerAndNestedFieldsMatchEncodingJSON(t *testing.T) {
	var user User
	if err := user.UnmarshalJSON([]byte(`{"address":{"city":"first"},"address":{"zip":"second"}}`)); err != nil {
		t.Fatal(err)
	}
	if user.Address.City != "first" || user.Address.Zip != "second" {
		t.Fatalf("duplicate nested struct did not merge: %#v", user.Address)
	}

	var value CompatibilityValues
	input := `{
		"slice":[1],"slice":[2,3],
		"map":{"first":1},"map":{"second":2},
		"pointer":1,"pointer":2,
		"nestedStruct":{"renamed":"first"},
		"nestedStruct":{"named":"second"}
	}`
	if err := value.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value.Slice, []int{2, 3}) {
		t.Fatalf("duplicate slice should replace with later value: %#v", value.Slice)
	}
	if !reflect.DeepEqual(value.Map, map[string]int{"first": 1, "second": 2}) {
		t.Fatalf("duplicate map should merge: %#v", value.Map)
	}
	if value.Pointer == nil || *value.Pointer != 2 {
		t.Fatalf("duplicate pointer scalar should use later value: %#v", value.Pointer)
	}
	if value.NestedStruct == nil || value.NestedStruct.Renamed != "first" || value.NestedStruct.Named != "second" {
		t.Fatalf("duplicate pointer-to-struct should merge: %#v", value.NestedStruct)
	}
}

func TestNamedStringKeyMapUsesGeneratedPath(t *testing.T) {
	want := CompatibilityValues{NamedKeyMap: map[NamedMapKey]string{
		"z":  "last",
		"a":  "first",
		"<&": "escaped",
	}}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"namedKeyMap":{"\u003c\u0026":"escaped","a":"first","z":"last"}`) {
		t.Fatalf("named-key map is not sorted and escaped canonically: %s", data)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.NamedKeyMap, want.NamedKeyMap) {
		t.Fatalf("named-key map round trip mismatch: %#v", got.NamedKeyMap)
	}

	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"mapKeysNamedKeyMap = append(mapKeysNamedKeyMap, string(mk))",
		"me := v.NamedKeyMap[NamedMapKey(mk)]",
		"mk := NamedMapKey(l.KeyString())",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated named-key map path missing %q", want)
		}
	}
	for _, unwanted := range []string{"json.Marshal(v.NamedKeyMap)", "json.Unmarshal(raw, &v.NamedKeyMap)"} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("named-key map unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}

func TestTextInterfaceMapKeysUseCanonicalFallback(t *testing.T) {
	want := CompatibilityValues{TextKeyMap: map[TextMapKey]int{{Value: "b"}: 2, {Value: "a"}: 1}}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"textKeyMap":{"key:a":1,"key:b":2}`) {
		t.Fatalf("text-marshaled map keys encoded incorrectly: %s", data)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.TextKeyMap, want.TextKeyMap) {
		t.Fatalf("text-unmarshaled map keys = %#v, want %#v", got.TextKeyMap, want.TextKeyMap)
	}

	before := CompatibilityValues{TextKeyMap: map[TextMapKey]int{{Value: "keep"}: 1}}
	value := CompatibilityValues{TextKeyMap: map[TextMapKey]int{{Value: "keep"}: 1}}
	if err := value.UnmarshalJSON([]byte(`{"textKeyMap":{"key:new":2},"bool":"wrong"}`)); err == nil {
		t.Fatal("wrong later field kind accepted")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("text-key fallback changed receiver after failure: %#v", value)
	}

	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"json.Marshal(v.TextKeyMap)",
		"decoded := make(map[TextMapKey]int",
		"json.Unmarshal(raw, &decoded)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("text-key map fallback missing %q", want)
		}
	}
}
