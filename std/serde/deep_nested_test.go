package serde

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDeepNestedPointerContainersMatchEncodingJSON(t *testing.T) {
	integer := NamedInt(42)
	inner := &integer
	middle := &inner
	outer := &middle
	want := CompatibilityValues{
		NamedIntTriple:     outer,
		NamedIntDoublePtrs: []**NamedInt{nil, middle},
		NamedIntTripleMap:  map[string]***NamedInt{"nil": nil, "value": outer},
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
		t.Fatalf("deep nested encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}

	var got CompatibilityValues
	if err := got.UnmarshalJSON(gotJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deepNestedFixture(got), deepNestedFixture(want)) {
		t.Fatalf("deep nested round trip differs:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDeepNestedPointerContainersPreserveNullAndAtomicFailure(t *testing.T) {
	integer := NamedInt(7)
	inner := &integer
	middle := &inner
	outer := &middle
	before := CompatibilityValues{
		NamedIntTriple:     outer,
		NamedIntDoublePtrs: []**NamedInt{middle},
		NamedIntTripleMap:  map[string]***NamedInt{"keep": outer},
	}

	for _, input := range []string{
		`{"namedIntTriple":9223372036854775808}`,
		`{"namedIntDoublePtrs":[7,9223372036854775808]}`,
		`{"namedIntTripleMap":{"bad":9223372036854775808}}`,
	} {
		t.Run(input, func(t *testing.T) {
			value := before
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid deep nested value accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("failed decode changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}

	value := before
	if err := value.UnmarshalJSON([]byte(`{"namedIntTriple":null,"namedIntDoublePtrs":[null],"namedIntTripleMap":{"null":null}}`)); err != nil {
		t.Fatal(err)
	}
	if value.NamedIntTriple != nil || len(value.NamedIntDoublePtrs) != 1 || value.NamedIntDoublePtrs[0] != nil || value.NamedIntTripleMap["null"] != nil {
		t.Fatalf("deep nested null semantics differ: %#v", value)
	}
}

func TestDeepNestedPointerContainersAvoidFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"***v.NamedIntTriple",
		"v.NamedIntDoublePtrs = append(v.NamedIntDoublePtrs, &inner)",
		"v.NamedIntTripleMap[mk] = &middle",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("deep nested generated path missing %q", want)
		}
	}
	for _, field := range []string{"NamedIntTriple", "NamedIntDoublePtrs", "NamedIntTripleMap"} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("deep nested field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}

func deepNestedFixture(value CompatibilityValues) any {
	var direct *NamedInt
	if value.NamedIntTriple != nil && *value.NamedIntTriple != nil {
		direct = **value.NamedIntTriple
	}
	slice := make([]*NamedInt, len(value.NamedIntDoublePtrs))
	for i, element := range value.NamedIntDoublePtrs {
		if element != nil {
			slice[i] = *element
		}
	}
	mapped := make(map[string]*NamedInt, len(value.NamedIntTripleMap))
	for key, element := range value.NamedIntTripleMap {
		if element != nil && *element != nil {
			mapped[key] = **element
		} else {
			mapped[key] = nil
		}
	}
	return struct {
		Direct *NamedInt
		Slice  []*NamedInt
		Map    map[string]*NamedInt
	}{direct, slice, mapped}
}
