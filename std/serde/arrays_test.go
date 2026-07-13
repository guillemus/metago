package serde

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeScalarAndGeneratedStructArrays(t *testing.T) {
	want := CompatibilityValues{
		Array:        [3]int{1, 2, 3},
		Int8Array:    [2]int8{math.MinInt8, math.MaxInt8},
		Float64Array: [2]float64{math.SmallestNonzeroFloat64, math.MaxFloat64},
		AddressArray: [2]Address{{City: "first"}, {City: "second"}},
	}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Array, want.Array) ||
		!reflect.DeepEqual(got.Int8Array, want.Int8Array) ||
		!reflect.DeepEqual(got.Float64Array, want.Float64Array) ||
		!reflect.DeepEqual(got.AddressArray, want.AddressArray) {
		t.Fatalf("array round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativeArraysShortLongOverflowAndAtomicFailure(t *testing.T) {
	value := CompatibilityValues{Array: [3]int{9, 9, 9}, AddressArray: [2]Address{{City: "old"}, {City: "old"}}}
	if err := value.UnmarshalJSON([]byte(`{"array":[1,2],"addressArray":[{"city":"new"}]}`)); err != nil {
		t.Fatal(err)
	}
	if value.Array != [3]int{1, 2, 0} || value.AddressArray != [2]Address{{City: "new"}, {}} {
		t.Fatalf("short arrays did not zero remaining elements: %#v", value)
	}
	if err := value.UnmarshalJSON([]byte(`{"array":[1,2,3,4],"addressArray":[{"city":"a"},{"city":"b"},{"city":"ignored"}]}`)); err != nil {
		t.Fatal(err)
	}
	if value.Array != [3]int{1, 2, 3} || value.AddressArray != [2]Address{{City: "a"}, {City: "b"}} {
		t.Fatalf("long arrays did not discard extra elements: %#v", value)
	}

	before := CompatibilityValues{Int8Array: [2]int8{7, 8}, AddressArray: [2]Address{{City: "keep"}}}
	value = before
	if err := value.UnmarshalJSON([]byte(`{"addressArray":[{"city":"changed"}],"int8Array":[128]}`)); err == nil {
		t.Fatal("array element overflow accepted")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("array failure changed receiver:\n got: %#v\nwant: %#v", value, before)
	}
}

func TestNativeFloatArraysRejectNonFiniteValues(t *testing.T) {
	for _, value := range []CompatibilityValues{
		{Float64Array: [2]float64{math.NaN()}},
		{Float64Array: [2]float64{math.Inf(1)}},
	} {
		if data, err := value.MarshalJSON(); err == nil || data != nil {
			t.Fatalf("non-finite array value encoded as %q without error", data)
		}
	}
}

func TestSupportedArraysUseGeneratedPaths(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"for i, e := range v.Array",
		"decoded[i] = int(l.Int(strconv.IntSize))",
		"decoded[i] = int8(l.Int(8))",
		"decoded[i].unmarshalJSONLexer(l)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated array path missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"json.Marshal(v.Array)", "json.Unmarshal(raw, &v.Array)",
		"json.Marshal(v.AddressArray)", "json.Unmarshal(raw, &v.AddressArray)",
	} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("supported array unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}
