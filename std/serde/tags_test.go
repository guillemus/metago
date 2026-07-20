package serde

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestStringTagScalarsMatchEncodingJSON(t *testing.T) {
	value := CompatibilityTagBehavior{
		QuotedString: "quote \" slash \\ line\n日本語",
		QuotedBool:   true,
		QuotedInt:    math.MinInt,
		QuotedUint:   math.MaxUint64,
		QuotedFloat:  math.SmallestNonzeroFloat64,
		QuotedNamed:  NamedInt(math.MaxInt64),
	}
	type plain CompatibilityTagBehavior
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(plain(value))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf(",string encoding differs:\n got: %s\nwant: %s", got, want)
	}

	var decoded CompatibilityTagBehavior
	if err := decoded.UnmarshalJSON(want); err != nil {
		t.Fatal(err)
	}
	var standard plain
	if err := json.Unmarshal(want, &standard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain(decoded), standard) {
		t.Fatalf(",string decoding differs:\n got: %#v\nwant: %#v", decoded, standard)
	}
}

func TestStringTagScalarsRejectInvalidForms(t *testing.T) {
	type plain CompatibilityTagBehavior
	inputs := []string{
		`{"quotedString":"plain"}`,
		`{"quotedBool":true}`,
		`{"quotedBool":"False"}`,
		`{"quotedInt":"+1"}`,
		`{"quotedInt":"1.0"}`,
		`{"quotedInt":"999999999999999999999999"}`,
		`{"quotedUint":"-1"}`,
		`{"quotedUint":"18446744073709551616"}`,
		`{"quotedFloat":"NaN"}`,
		`{"quotedFloat":"Infinity"}`,
		`{"quotedFloat":"1e1000"}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			if err := json.Unmarshal([]byte(input), new(plain)); err == nil {
				t.Fatalf("test case is accepted by encoding/json: %s", input)
			}
			before := CompatibilityTagBehavior{QuotedString: "keep", QuotedBool: true, QuotedInt: 7, QuotedUint: 8, QuotedFloat: 9, QuotedNamed: 10}
			value := before
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid ,string value accepted: %s", input)
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatalf("invalid ,string value changed receiver:\n got: %#v\nwant: %#v", value, before)
			}
		})
	}
}

// Covers Go issue 7046; see testdata/PROVENANCE.md.
func TestStringTagNullFormsPreserveScalars(t *testing.T) {
	before := CompatibilityTagBehavior{QuotedString: "keep", QuotedBool: true, QuotedInt: 7, QuotedUint: 8, QuotedFloat: 9, QuotedNamed: 10}
	for _, input := range []string{
		`{"quotedString":null,"quotedBool":null,"quotedInt":null,"quotedUint":null,"quotedFloat":null,"quotedNamed":null}`,
		`{"quotedString":"null","quotedBool":"null","quotedInt":"null","quotedUint":"null","quotedFloat":"null","quotedNamed":"null"}`,
	} {
		value := before
		if err := value.UnmarshalJSON([]byte(input)); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value, before) {
			t.Fatalf(",string null changed scalars:\n got: %#v\nwant: %#v", value, before)
		}
	}
}

func TestOmitZeroMatchesEncodingJSON(t *testing.T) {
	zeroMethod := ZeroByMethod("zero")
	valueMethod := ZeroByMethod("value")
	values := []CompatibilityTagBehavior{
		{},
		{
			ZeroString:    "value",
			ZeroSlice:     []int{},
			ZeroArray:     [2]int{0, 1},
			ZeroMethod:    "value",
			ZeroMethodPtr: &valueMethod,
			ZeroNamed:     1,
			ZeroComposite: ZeroComposite{Values: []int{}},
			OmitBoth:      []int{1},
		},
		{
			ZeroMethod:    "zero",
			ZeroMethodPtr: &zeroMethod,
			ZeroSlice:     []int{1},
			OmitBoth:      []int{},
		},
	}

	type plain CompatibilityTagBehavior
	for _, value := range values {
		got, err := value.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(plain(value))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("omitzero encoding differs:\n got: %s\nwant: %s", got, want)
		}
	}
}
