package serde

import (
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeSlicesOfScalarAndGeneratedPointers(t *testing.T) {
	integer := 42
	unsigned := uint8(math.MaxUint8)
	float := math.SmallestNonzeroFloat64
	want := CompatibilityValues{
		PointerSlice:    []*int{&integer, nil},
		Uint8Pointers:   []*uint8{nil, &unsigned},
		FloatPointers:   []*float64{&float, nil},
		AddressPointers: []*Address{{City: "first"}, nil, {City: "third"}},
	}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.PointerSlice, want.PointerSlice) ||
		!reflect.DeepEqual(got.Uint8Pointers, want.Uint8Pointers) ||
		!reflect.DeepEqual(got.FloatPointers, want.FloatPointers) ||
		!reflect.DeepEqual(got.AddressPointers, want.AddressPointers) {
		t.Fatalf("pointer-slice round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativePointerSlicesRejectInvalidValuesAtomically(t *testing.T) {
	integer := 7
	before := CompatibilityValues{PointerSlice: []*int{&integer}, AddressPointers: []*Address{{City: "keep"}}}
	for _, input := range []string{
		`{"uint8Pointers":[256]}`,
		`{"addressPointers":[{"city":"changed"}],"pointerSlice":["wrong"]}`,
	} {
		integerCopy := 7
		value := CompatibilityValues{PointerSlice: []*int{&integerCopy}, AddressPointers: []*Address{{City: "keep"}}}
		if err := value.UnmarshalJSON([]byte(input)); err == nil {
			t.Fatalf("invalid pointer-slice value accepted: %s", input)
		}
		if !reflect.DeepEqual(value, before) {
			t.Fatalf("pointer-slice failure changed receiver:\n got: %#v\nwant: %#v", value, before)
		}
	}

	for _, value := range []CompatibilityValues{
		{FloatPointers: []*float64{new(math.NaN())}},
		{FloatPointers: []*float64{new(math.Inf(-1))}},
	} {
		if data, err := value.MarshalJSON(); err == nil || data != nil {
			t.Fatalf("non-finite pointer-slice element encoded as %q without error", data)
		}
	}
}

func TestGeneratedPointerSliceDetectsCycles(t *testing.T) {
	node := &CompatibilityCycle{Value: "cycle"}
	node.Next = node
	if data, err := (CompatibilityValues{CyclePointers: []*CompatibilityCycle{node}}).MarshalJSON(); err == nil || data != nil {
		t.Fatalf("cyclic pointer-slice element encoded as %q without error", data)
	}
}

func TestSupportedPointerSlicesUseGeneratedPaths(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	start := strings.Index(generated, "func (v CompatibilityValues) appendJSONState")
	end := strings.Index(generated[start:], "// MarshalJSON implements json.Marshaler for CompatibilityTagBehavior")
	if start < 0 || end < 0 {
		t.Fatal("generated CompatibilityValues source block not found")
	}
	block := generated[start : start+end]
	for _, want := range []string{
		"v.PointerSlice = append(v.PointerSlice, &decoded)",
		"decoded := new(Address)",
		"v.AddressPointers = append(v.AddressPointers, decoded)",
		"if _, exists := seen[e]; exists",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("generated pointer-slice path missing %q", want)
		}
	}
}
