package serde

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestNamedScalarContainersMatchEncodingJSON(t *testing.T) {
	text := NamedString("pointer")
	nestedInt := NamedInt(math.MinInt64)
	nestedIntPointer := &nestedInt
	pointerText := NamedString("slice pointer")
	mapInt := NamedInt(math.MaxInt64)
	want := CompatibilityValues{
		NamedStringPtr:   &text,
		NamedIntNested:   &nestedIntPointer,
		NamedStringSlice: []NamedString{"first", "<&>"},
		NamedBoolSlice:   []NamedBool{true, false},
		NamedIntSlice:    []NamedInt{math.MinInt64, 0, math.MaxInt64},
		NamedStringPtrs:  []*NamedString{nil, &pointerText},
		NamedUintArray:   [2]NamedUint{0, NamedUint(math.MaxUint32)},
		NamedFloatMap:    map[string]NamedFloat{"small": NamedFloat(math.SmallestNonzeroFloat32), "value": 1.5},
		NamedIntPtrMap:   map[string]*NamedInt{"nil": nil, "value": &mapInt},
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
		t.Fatalf("named scalar encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	var got CompatibilityValues
	if err := got.UnmarshalJSON(gotJSON); err != nil {
		t.Fatal(err)
	}
	gotNamed := namedScalarFixture(got)
	wantNamed := namedScalarFixture(want)
	if !reflect.DeepEqual(gotNamed, wantNamed) {
		t.Fatalf("named scalar round trip differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func namedScalarFixture(value CompatibilityValues) any {
	return struct {
		StringPtr  *NamedString
		IntNested  **NamedInt
		Strings    []NamedString
		Bools      []NamedBool
		Ints       []NamedInt
		StringPtrs []*NamedString
		Uints      [2]NamedUint
		Floats     map[string]NamedFloat
		IntPtrs    map[string]*NamedInt
	}{
		value.NamedStringPtr,
		value.NamedIntNested,
		value.NamedStringSlice,
		value.NamedBoolSlice,
		value.NamedIntSlice,
		value.NamedStringPtrs,
		value.NamedUintArray,
		value.NamedFloatMap,
		value.NamedIntPtrMap,
	}
}

func TestNamedScalarContainerFailuresAreAtomic(t *testing.T) {
	keepText := NamedString("keep")
	keepInt := NamedInt(7)
	before := CompatibilityValues{
		NamedStringPtr:   &keepText,
		NamedStringSlice: []NamedString{"keep"},
		NamedBoolSlice:   []NamedBool{true},
		NamedIntSlice:    []NamedInt{7},
		NamedStringPtrs:  []*NamedString{&keepText},
		NamedUintArray:   [2]NamedUint{7, 8},
		NamedFloatMap:    map[string]NamedFloat{"keep": 7},
		NamedIntPtrMap:   map[string]*NamedInt{"keep": &keepInt},
	}

	inputs := []string{
		`{"namedStringPtr":1}`,
		`{"namedStringSlice":["changed",1]}`,
		`{"namedBoolSlice":[true,0]}`,
		`{"namedIntSlice":[9223372036854775808]}`,
		`{"namedStringPtrs":["changed",1]}`,
		`{"namedUintArray":[4294967296]}`,
		`{"namedFloatMap":{"bad":1e1000}}`,
		`{"namedIntPtrMap":{"bad":9223372036854775808}}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			value := cloneNamedScalarFixture(before)
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid named scalar container accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("failed decode changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

func TestNamedScalarContainersUseGeneratedPaths(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"decoded := NamedString(l.String())",
		"decoded := NamedInt(l.Int(64))",
		"append(v.NamedStringSlice, NamedString(l.String()))",
		"append(v.NamedBoolSlice, NamedBool(l.Bool()))",
		"append(v.NamedIntSlice, NamedInt(l.Int(64)))",
		"decoded[i] = NamedUint(l.Uint(32))",
		"v.NamedFloatMap[mk] = NamedFloat(l.Float32())",
		"decodedValue := NamedInt(l.Int(64))",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated named-scalar path missing %q", want)
		}
	}
	for _, field := range []string{
		"NamedStringPtr", "NamedIntNested", "NamedStringSlice", "NamedBoolSlice", "NamedIntSlice",
		"NamedStringPtrs", "NamedUintArray", "NamedFloatMap", "NamedIntPtrMap",
	} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("named scalar field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}

func cloneNamedScalarFixture(value CompatibilityValues) CompatibilityValues {
	clone := value
	if value.NamedStringPtr != nil {
		copyValue := *value.NamedStringPtr
		clone.NamedStringPtr = &copyValue
	}
	clone.NamedStringSlice = append([]NamedString(nil), value.NamedStringSlice...)
	clone.NamedBoolSlice = append([]NamedBool(nil), value.NamedBoolSlice...)
	clone.NamedIntSlice = append([]NamedInt(nil), value.NamedIntSlice...)
	clone.NamedStringPtrs = append([]*NamedString(nil), value.NamedStringPtrs...)
	clone.NamedFloatMap = make(map[string]NamedFloat, len(value.NamedFloatMap))
	for key, element := range value.NamedFloatMap {
		clone.NamedFloatMap[key] = element
	}
	clone.NamedIntPtrMap = make(map[string]*NamedInt, len(value.NamedIntPtrMap))
	for key, element := range value.NamedIntPtrMap {
		if element == nil {
			clone.NamedIntPtrMap[key] = nil
			continue
		}
		copyValue := *element
		clone.NamedIntPtrMap[key] = &copyValue
	}
	return clone
}
