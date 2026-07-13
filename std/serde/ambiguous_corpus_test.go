package serde

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type ambiguousSubject struct {
	name   string
	kind   string
	input  []byte
	accept bool
}

func jsonTestSuiteAmbiguousSubjects() []ambiguousSubject {
	number := func(name, input string, accept bool) ambiguousSubject {
		return ambiguousSubject{name: name, kind: "number", input: []byte(input), accept: accept}
	}
	value := func(name string, input []byte, accept bool) ambiguousSubject {
		return ambiguousSubject{name: name, kind: "value", input: input, accept: accept}
	}
	document := func(name string, input []byte, accept bool) ambiguousSubject {
		return ambiguousSubject{name: name, kind: "document", input: input, accept: accept}
	}

	return []ambiguousSubject{
		number("i_number_double_huge_neg_exp.json", "123.456e-789", true),
		number("i_number_huge_exp.json", "0.4e6699", false),
		number("i_number_neg_int_huge_exp.json", "-1e+9999", false),
		number("i_number_pos_double_huge_exp.json", "1.5e+9999", false),
		number("i_number_real_neg_overflow.json", "-123123e100000", false),
		number("i_number_real_pos_overflow.json", "123123e100000", false),
		number("i_number_real_underflow.json", "123e-10000000", true),
		number("i_number_too_big_neg_int.json", "-123123123123123123123123123123", true),
		number("i_number_too_big_pos_int.json", "100000000000000000000", true),
		number("i_number_very_big_negative_int.json", "-237462374673276894279832749832423479823246327846", true),

		document("i_object_key_lone_2nd_surrogate.json", []byte(`{"\uDFAA":0}`), true),
		value("i_string_1st_surrogate_but_2nd_missing.json", []byte(`["\uDADA"]`), true),
		value("i_string_1st_valid_surrogate_2nd_invalid.json", []byte(`["\uD888\u1234"]`), true),
		document("i_string_UTF-16LE_with_BOM.json", mustHexBytes("fffe5b002200e90022005d00"), false),
		value("i_string_UTF-8_invalid_sequence.json", mustHexBytes("5b22e697a5d188fa225d"), true),
		value("i_string_UTF8_surrogate_U+D800.json", mustHexBytes("5b22eda080225d"), true),
		value("i_string_incomplete_surrogate_and_escape_valid.json", []byte(`["\uD800\n"]`), true),
		value("i_string_incomplete_surrogate_pair.json", []byte(`["\ud1ea"]`), true),
		value("i_string_incomplete_surrogates_escape_valid.json", []byte(`["\uD800\uD800\n"]`), true),
		value("i_string_invalid_lonely_surrogate.json", []byte(`["\ud800"]`), true),
		value("i_string_invalid_surrogate.json", []byte(`["\ud800abc"]`), true),
		value("i_string_invalid_utf-8.json", mustHexBytes("5b22ff225d"), true),
		value("i_string_inverted_surrogates_U+1D11E.json", []byte(`["\uDd1e\uD834"]`), true),
		value("i_string_iso_latin_1.json", mustHexBytes("5b22e9225d"), true),
		value("i_string_lone_second_surrogate.json", []byte(`["\uDFAA"]`), true),
		value("i_string_lone_utf8_continuation_byte.json", mustHexBytes("5b2281225d"), true),
		value("i_string_not_in_unicode_range.json", mustHexBytes("5b22f4bfbfbf225d"), true),
		value("i_string_overlong_sequence_2_bytes.json", mustHexBytes("5b22c0af225d"), true),
		value("i_string_overlong_sequence_6_bytes.json", mustHexBytes("5b22fc83bfbfbf225d"), true),
		value("i_string_overlong_sequence_6_bytes_null.json", mustHexBytes("5b22fc8080808080225d"), true),
		value("i_string_truncated-utf-8.json", mustHexBytes("5b22e0ff225d"), true),
		document("i_string_utf16BE_no_BOM.json", mustHexBytes("005b002200e90022005d"), false),
		document("i_string_utf16LE_no_BOM.json", mustHexBytes("5b002200e90022005d00"), false),
		value("i_structure_500_nested_arrays.json", []byte(strings.Repeat("[", 500)+strings.Repeat("]", 500)), true),
		document("i_structure_UTF-8_BOM_empty_object.json", mustHexBytes("efbbbf7b7d"), false),
	}
}

func TestJSONTestSuiteAmbiguousPolicies(t *testing.T) {
	subjects := jsonTestSuiteAmbiguousSubjects()

	if len(subjects) != 35 {
		t.Fatalf("ambiguous policy table contains %d subjects, want 35", len(subjects))
	}

	for _, tc := range subjects {
		t.Run(tc.name, func(t *testing.T) {
			input := adaptedAmbiguousInput(tc)
			switch tc.kind {
			case "number":
				var generated CompatibilityNumbers
				generatedErr := generated.UnmarshalJSON(input)
				type plain CompatibilityNumbers
				var standard plain
				standardErr := json.Unmarshal(input, &standard)
				assertAmbiguousPolicy(t, tc, generatedErr, standardErr)
				if generatedErr == nil && generated.Float64 != standard.Float64 {
					t.Fatalf("decoded float64 = %v, encoding/json = %v", generated.Float64, standard.Float64)
				}
			case "value":
				var generated CompatibilityValues
				generatedErr := generated.UnmarshalJSON(input)
				type plain CompatibilityValues
				var standard plain
				standardErr := json.Unmarshal(input, &standard)
				assertAmbiguousPolicy(t, tc, generatedErr, standardErr)
				if generatedErr == nil && !reflect.DeepEqual(generated.Interface, standard.Interface) {
					t.Fatalf("decoded value = %#v, encoding/json = %#v", generated.Interface, standard.Interface)
				}
			case "document":
				var generated CompatibilityValues
				generatedErr := generated.UnmarshalJSON(input)
				type plain CompatibilityValues
				var standard plain
				standardErr := json.Unmarshal(input, &standard)
				assertAmbiguousPolicy(t, tc, generatedErr, standardErr)
			default:
				t.Fatalf("unknown ambiguous subject kind %q", tc.kind)
			}
		})
	}
}

func adaptedAmbiguousInput(tc ambiguousSubject) []byte {
	switch tc.kind {
	case "number":
		return []byte(`{"float64":` + string(tc.input) + `}`)
	case "value":
		input := append([]byte(`{"interface":`), tc.input...)
		return append(input, '}')
	case "document":
		return tc.input
	default:
		panic("unknown ambiguous subject kind " + tc.kind)
	}
}

func assertAmbiguousPolicy(t *testing.T, tc ambiguousSubject, generatedErr, standardErr error) {
	t.Helper()

	if (generatedErr == nil) != tc.accept {
		t.Fatalf("serde policy accept=%t, error=%v", tc.accept, generatedErr)
	}
	if (standardErr == nil) != tc.accept {
		t.Fatalf("encoding/json does not support recorded policy accept=%t, error=%v", tc.accept, standardErr)
	}
}

func mustHexBytes(value string) []byte {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return decoded
}
