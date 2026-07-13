package serde

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestAdvancedNativeMapValuesMatchEncodingJSON(t *testing.T) {
	integer := NamedInt(42)
	integerPointer := &integer
	integerNested := &integerPointer
	want := CompatibilityValues{
		NamedIntNestedMap: map[string]**NamedInt{
			"nil": nil, "value": integerNested,
		},
		NamedIntPtrSliceMap: map[string][]*NamedInt{
			"nil": nil, "values": {nil, &integer},
		},
		RawMap: map[string]json.RawMessage{
			"nil": nil, "null": json.RawMessage(`null`), "object": json.RawMessage(`{ "value" : [1, true] }`),
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
		t.Fatalf("advanced map encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	var got CompatibilityValues
	if err := got.UnmarshalJSON(gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(advancedMapFixture(got), advancedMapFixture(want)) {
		t.Fatalf("advanced map round trip differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAdvancedNativeMapFailuresAreAtomic(t *testing.T) {
	integer := NamedInt(7)
	integerPointer := &integer
	before := CompatibilityValues{
		NamedIntNestedMap:   map[string]**NamedInt{"keep": &integerPointer},
		NamedIntPtrSliceMap: map[string][]*NamedInt{"keep": {&integer}},
		RawMap:              map[string]json.RawMessage{"keep": json.RawMessage(`{"value":7}`)},
	}
	inputs := []string{
		`{"namedIntNestedMap":{"bad":9223372036854775808}}`,
		`{"namedIntPtrSliceMap":{"bad":[7,9223372036854775808]}}`,
		`{"rawMap":{"bad":{"truncated":}}}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			value := cloneAdvancedMapFixture(before)
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid advanced map value accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("failed decode changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

func TestRawMapCopiesDecodedValues(t *testing.T) {
	input := []byte(`{"rawMap":{"value":{"nested":[1,2,3]}}}`)
	var value CompatibilityValues
	if err := value.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}
	want := append(json.RawMessage(nil), value.RawMap["value"]...)
	for i := range input {
		input[i] = 'x'
	}
	if !reflect.DeepEqual(value.RawMap["value"], want) {
		t.Fatalf("decoded RawMessage map value aliases input: got %q, want %q", value.RawMap["value"], want)
	}
}

func TestAdvancedNativeMapValuesAvoidFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"v.NamedIntNestedMap[mk] = &inner",
		"decodedValue := make([]*NamedInt, 0, 8)",
		"v.RawMap[mk] = append(v.RawMap[mk][:0:0], raw...)",
		"AppendRaw(b, me)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("advanced generated map path missing %q", want)
		}
	}
	for _, field := range []string{"NamedIntNestedMap", "NamedIntPtrSliceMap", "RawMap"} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("advanced map field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}

func advancedMapFixture(value CompatibilityValues) any {
	raw := make(map[string]any, len(value.RawMap))
	for key, element := range value.RawMap {
		if element == nil {
			raw[key] = nil
			continue
		}
		var decoded any
		if err := json.Unmarshal(element, &decoded); err != nil {
			panic(err)
		}
		raw[key] = decoded
	}
	return struct {
		Nested map[string]**NamedInt
		Slices map[string][]*NamedInt
		Raw    map[string]any
	}{value.NamedIntNestedMap, value.NamedIntPtrSliceMap, raw}
}

func cloneAdvancedMapFixture(value CompatibilityValues) CompatibilityValues {
	clone := value
	clone.NamedIntNestedMap = make(map[string]**NamedInt, len(value.NamedIntNestedMap))
	for key, element := range value.NamedIntNestedMap {
		if element != nil && *element != nil {
			integer := **element
			pointer := &integer
			clone.NamedIntNestedMap[key] = &pointer
		}
	}
	clone.NamedIntPtrSliceMap = make(map[string][]*NamedInt, len(value.NamedIntPtrSliceMap))
	for key, element := range value.NamedIntPtrSliceMap {
		copied := make([]*NamedInt, len(element))
		for i, integer := range element {
			if integer != nil {
				value := *integer
				copied[i] = &value
			}
		}
		clone.NamedIntPtrSliceMap[key] = copied
	}
	clone.RawMap = make(map[string]json.RawMessage, len(value.RawMap))
	for key, element := range value.RawMap {
		clone.RawMap[key] = append(json.RawMessage(nil), element...)
	}
	return clone
}
