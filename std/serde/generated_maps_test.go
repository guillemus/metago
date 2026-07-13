package serde

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedStructAndPointerMapValues(t *testing.T) {
	want := CompatibilityValues{
		AddressMap:        map[string]Address{"b": {City: "second"}, "a": {City: "first"}},
		AddressPointerMap: map[string]*Address{"nil": nil, "set": {City: "value"}},
	}
	data, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got CompatibilityValues
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.AddressMap, want.AddressMap) || !reflect.DeepEqual(got.AddressPointerMap, want.AddressPointerMap) {
		t.Fatalf("generated map-value round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	if strings.Index(string(data), `"a":`) > strings.Index(string(data), `"b":`) {
		t.Fatalf("generated map keys are not sorted: %s", data)
	}
}

func TestGeneratedMapValuesReplaceDuplicateKeysAndFailAtomically(t *testing.T) {
	var value CompatibilityValues
	input := `{"addressMap":{"key":{"street":"first"},"key":{"city":"second"}},"addressPointerMap":{"key":{"street":"first"},"key":{"city":"second"}}}`
	if err := value.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if got := value.AddressMap["key"]; got.Street != "" || got.City != "second" {
		t.Fatalf("duplicate struct map value should replace: %#v", got)
	}
	if got := value.AddressPointerMap["key"]; got == nil || got.Street != "" || got.City != "second" {
		t.Fatalf("duplicate pointer map value should replace: %#v", got)
	}

	before := CompatibilityValues{AddressMap: map[string]Address{"keep": {City: "old"}}, AddressPointerMap: map[string]*Address{"keep": {City: "old"}}}
	value = CompatibilityValues{AddressMap: map[string]Address{"keep": {City: "old"}}, AddressPointerMap: map[string]*Address{"keep": {City: "old"}}}
	if err := value.UnmarshalJSON([]byte(`{"addressMap":{"new":{"city":"changed"}},"addressPointerMap":{"new":{"city":42}}}`)); err == nil {
		t.Fatal("invalid generated map value accepted")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("generated map failure changed receiver:\n got: %#v\nwant: %#v", value, before)
	}
}

func TestGeneratedPointerMapDetectsCycles(t *testing.T) {
	node := &CompatibilityCycle{Value: "cycle"}
	node.Next = node
	if data, err := (CompatibilityValues{CyclePointerMap: map[string]*CompatibilityCycle{"cycle": node}}).MarshalJSON(); err == nil || data != nil {
		t.Fatalf("cyclic pointer map value encoded as %q without error", data)
	}
}

func TestGeneratedMapValuesAvoidFallback(t *testing.T) {
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
	for _, want := range []string{"me.appendJSONState(b, seen)", "decodedValue.unmarshalJSONLexer(l)", "decodedPointer := new(Address)"} {
		if !strings.Contains(block, want) {
			t.Errorf("generated map-value path missing %q", want)
		}
	}
	for _, unwanted := range []string{"json.Marshal(v.AddressMap)", "json.Marshal(v.AddressPointerMap)"} {
		if strings.Contains(block, unwanted) {
			t.Errorf("generated map value unexpectedly uses fallback %q", unwanted)
		}
	}
}
