package serde

import (
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNativeNestedMapValuesMatchEncodingJSON(t *testing.T) {
	want := CompatibilityValues{
		NamedIntSliceMap: map[string][]NamedInt{
			"nil": nil, "empty": {}, "values": {1, 0, -2},
		},
		ByteSliceMap: map[string][]byte{
			"nil": nil, "empty": {}, "bytes": {0, 1, 255},
		},
		NamedUintArrayMap: map[string][2]NamedUint{
			"zero": {}, "values": {1, 2},
		},
		NestedScalarMap: map[string]map[NamedMapKey]int8{
			"nil": nil, "empty": {}, "values": {"z": -1, "a": 2},
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
		t.Fatalf("nested map encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	var got CompatibilityValues
	if err := got.UnmarshalJSON(gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.NamedIntSliceMap, want.NamedIntSliceMap) ||
		!reflect.DeepEqual(got.ByteSliceMap, want.ByteSliceMap) ||
		!reflect.DeepEqual(got.NamedUintArrayMap, want.NamedUintArrayMap) ||
		!reflect.DeepEqual(got.NestedScalarMap, want.NestedScalarMap) {
		t.Fatalf("nested map round trip differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativeNestedMapDecodeSemanticsMatchEncodingJSON(t *testing.T) {
	input := []byte(`{
		"namedIntSliceMap":{"nulls":[null,1],"replace":[1]},
		"namedIntSliceMap":{"replace":[2,3]},
		"byteSliceMap":{"bytes":"AAH/"},
		"namedUintArrayMap":{"short":[1],"long":[1,2,3],"null":null},
		"nestedScalarMap":{"values":{"a":1,"a":2,"zero":null}},
		"nestedScalarMap":{"replace":{"b":3}}
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
	if !reflect.DeepEqual(got.NamedIntSliceMap, want.NamedIntSliceMap) ||
		!reflect.DeepEqual(got.ByteSliceMap, want.ByteSliceMap) ||
		!reflect.DeepEqual(got.NamedUintArrayMap, want.NamedUintArrayMap) ||
		!reflect.DeepEqual(got.NestedScalarMap, want.NestedScalarMap) {
		t.Fatalf("nested map decode semantics differ:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNativeNestedMapFailuresAreAtomic(t *testing.T) {
	before := CompatibilityValues{
		NamedIntSliceMap:  map[string][]NamedInt{"keep": {7}},
		ByteSliceMap:      map[string][]byte{"keep": {7}},
		NamedUintArrayMap: map[string][2]NamedUint{"keep": {7, 8}},
		NestedScalarMap:   map[string]map[NamedMapKey]int8{"keep": {"value": 7}},
	}
	inputs := []string{
		`{"namedIntSliceMap":{"bad":[9223372036854775808]}}`,
		`{"byteSliceMap":{"bad":"%%%"}}`,
		`{"namedUintArrayMap":{"bad":[4294967296]}}`,
		`{"nestedScalarMap":{"bad":{"value":128}}}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			value := cloneNestedMapFixture(before)
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid nested map value accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("failed nested map decode changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

func TestNativeNestedMapValuesAvoidFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"for j, se := range me",
		"rawBytes := make([]byte, len(me))",
		"AppendBytes(b, rawBytes)",
		"for j, ae := range me",
		"nestedKeys := make([]string, 0, len(me))",
		"decodedValue := make([]NamedInt, 0, 8)",
		"decodedBytes := l.Bytes()",
		"v.ByteSliceMap[mk] = decodedValue",
		"var decodedValue [2]NamedUint",
		"decodedValue := make(map[NamedMapKey]int8, 8)",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated nested-map path missing %q", want)
		}
	}
	for _, field := range []string{"NamedIntSliceMap", "ByteSliceMap", "NamedUintArrayMap", "NestedScalarMap"} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("nested map field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}

func cloneNestedMapFixture(value CompatibilityValues) CompatibilityValues {
	clone := value
	clone.NamedIntSliceMap = make(map[string][]NamedInt, len(value.NamedIntSliceMap))
	for key, element := range value.NamedIntSliceMap {
		clone.NamedIntSliceMap[key] = append([]NamedInt(nil), element...)
	}
	clone.ByteSliceMap = make(map[string][]byte, len(value.ByteSliceMap))
	for key, element := range value.ByteSliceMap {
		clone.ByteSliceMap[key] = append([]byte(nil), element...)
	}
	clone.NamedUintArrayMap = make(map[string][2]NamedUint, len(value.NamedUintArrayMap))
	maps.Copy(clone.NamedUintArrayMap, value.NamedUintArrayMap)
	clone.NestedScalarMap = make(map[string]map[NamedMapKey]int8, len(value.NestedScalarMap))
	for key, element := range value.NestedScalarMap {
		inner := make(map[NamedMapKey]int8, len(element))
		maps.Copy(inner, element)
		clone.NestedScalarMap[key] = inner
	}
	return clone
}

func nestedMapFixture(value CompatibilityValues) any {
	return struct {
		Slices map[string][]NamedInt
		Bytes  map[string][]byte
		Arrays map[string][2]NamedUint
		Nested map[string]map[NamedMapKey]int8
	}{
		value.NamedIntSliceMap,
		value.ByteSliceMap,
		value.NamedUintArrayMap,
		value.NestedScalarMap,
	}
}
