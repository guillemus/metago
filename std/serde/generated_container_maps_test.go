package serde

import (
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedContainerMapValuesMatchEncodingJSON(t *testing.T) {
	want := CompatibilityValues{
		AddressSliceMap: map[string][]Address{
			"nil": nil, "empty": {}, "values": {{City: "first"}, {City: "second"}},
		},
		AddressPtrSliceMap: map[string][]*Address{
			"nil": nil, "values": {nil, {City: "pointer"}},
		},
		AddressArrayMap: map[string][2]Address{
			"zero": {}, "values": {{City: "first"}, {City: "second"}},
		},
		NestedAddressMap: map[string]map[string]Address{
			"nil": nil, "values": {"b": {City: "second"}, "a": {City: "first"}},
		},
		NestedAddressPtrMap: map[string]map[string]*Address{
			"nil": nil, "values": {"nil": nil, "set": {City: "pointer"}},
		},
	}

	gotJSON, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	type plain CompatibilityValues
	wantJSON, err := json.Marshal(plain(want))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("generated container-map encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	var got CompatibilityValues
	if err := got.UnmarshalJSON(gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(generatedContainerMapFixture(got), generatedContainerMapFixture(want)) {
		t.Fatalf("generated container-map round trip differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGeneratedContainerMapDecodeSemanticsMatchEncodingJSON(t *testing.T) {
	input := []byte(`{
		"addressSliceMap":{"nulls":[null,{"city":"first"}],"replace":[{"street":"old"}]},
		"addressSliceMap":{"replace":[{"city":"new"}]},
		"addressPtrSliceMap":{"values":[null,{"city":"pointer"}]},
		"addressArrayMap":{"short":[{"city":"first"}],"long":[{"city":"first"},{"city":"second"},{"city":"ignored"}],"null":null},
		"nestedAddressMap":{"values":{"same":{"street":"old"},"same":{"city":"new"},"null":null}},
		"nestedAddressPtrMap":{"values":{"nil":null,"set":{"city":"pointer"}}}
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
	if !reflect.DeepEqual(generatedContainerMapFixture(got), generatedContainerMapFixture(CompatibilityValues(want))) {
		t.Fatalf("generated container-map decode semantics differ:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGeneratedContainerMapFailuresAreAtomic(t *testing.T) {
	before := CompatibilityValues{
		AddressSliceMap:     map[string][]Address{"keep": {{City: "old"}}},
		AddressPtrSliceMap:  map[string][]*Address{"keep": {{City: "old"}}},
		AddressArrayMap:     map[string][2]Address{"keep": {{City: "old"}, {City: "second"}}},
		NestedAddressMap:    map[string]map[string]Address{"keep": {"value": {City: "old"}}},
		NestedAddressPtrMap: map[string]map[string]*Address{"keep": {"value": {City: "old"}}},
	}
	inputs := []string{
		`{"addressSliceMap":{"bad":[{"city":1}]}}`,
		`{"addressPtrSliceMap":{"bad":[{"city":1}]}}`,
		`{"addressArrayMap":{"bad":[{"city":1}]}}`,
		`{"nestedAddressMap":{"bad":{"value":{"city":1}}}}`,
		`{"nestedAddressPtrMap":{"bad":{"value":{"city":1}}}}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			value := cloneGeneratedContainerMapFixture(before)
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid generated container-map value accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("failed decode changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

func TestGeneratedContainerMapCyclesReturnErrors(t *testing.T) {
	node := &CompatibilityCycle{Value: "cycle"}
	node.Next = node
	values := []CompatibilityValues{
		{CyclePtrSliceMap: map[string][]*CompatibilityCycle{"cycle": {node}}},
		{NestedCyclePtrMap: map[string]map[string]*CompatibilityCycle{"outer": {"cycle": node}}},
	}
	for _, value := range values {
		if data, err := value.MarshalJSON(); err == nil || data != nil {
			t.Fatalf("cyclic generated container map encoded as %q without error", data)
		}
	}
}

func TestGeneratedContainerMapValuesAvoidFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"decodedValue := make([]Address, 0, 8)",
		"decodedElement := new(Address)",
		"var decodedValue [2]Address",
		"decodedValue := make(map[string]Address, 8)",
		"decodedValue := make(map[string]*Address, 8)",
		"nestedValue.appendJSONState(b, seen)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated container-map path missing %q", want)
		}
	}
	for _, field := range []string{
		"AddressSliceMap", "AddressPtrSliceMap", "CyclePtrSliceMap", "AddressArrayMap",
		"NestedAddressMap", "NestedAddressPtrMap", "NestedCyclePtrMap",
	} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("generated container-map field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}

func generatedContainerMapFixture(value CompatibilityValues) any {
	return struct {
		Slices        map[string][]Address
		PointerSlices map[string][]*Address
		Arrays        map[string][2]Address
		Nested        map[string]map[string]Address
		NestedPtrs    map[string]map[string]*Address
	}{value.AddressSliceMap, value.AddressPtrSliceMap, value.AddressArrayMap, value.NestedAddressMap, value.NestedAddressPtrMap}
}

func cloneGeneratedContainerMapFixture(value CompatibilityValues) CompatibilityValues {
	clone := value
	clone.AddressSliceMap = make(map[string][]Address, len(value.AddressSliceMap))
	for key, element := range value.AddressSliceMap {
		clone.AddressSliceMap[key] = append([]Address(nil), element...)
	}
	clone.AddressPtrSliceMap = make(map[string][]*Address, len(value.AddressPtrSliceMap))
	for key, element := range value.AddressPtrSliceMap {
		copied := make([]*Address, len(element))
		for i, address := range element {
			if address != nil {
				value := *address
				copied[i] = &value
			}
		}
		clone.AddressPtrSliceMap[key] = copied
	}
	clone.AddressArrayMap = make(map[string][2]Address, len(value.AddressArrayMap))
	maps.Copy(clone.AddressArrayMap, value.AddressArrayMap)
	clone.NestedAddressMap = make(map[string]map[string]Address, len(value.NestedAddressMap))
	for key, element := range value.NestedAddressMap {
		inner := make(map[string]Address, len(element))
		maps.Copy(inner, element)
		clone.NestedAddressMap[key] = inner
	}
	clone.NestedAddressPtrMap = make(map[string]map[string]*Address, len(value.NestedAddressPtrMap))
	for key, element := range value.NestedAddressPtrMap {
		inner := make(map[string]*Address, len(element))
		for innerKey, address := range element {
			if address != nil {
				value := *address
				inner[innerKey] = &value
			}
		}
		clone.NestedAddressPtrMap[key] = inner
	}
	return clone
}
