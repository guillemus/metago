package serde

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// These cases adapt subjects covered by RFC 8259, Go's encoding/json tests, JSONTestSuite,
// serde_json, Sonic, goccy/go-json, jsoniter, and easyjson. They intentionally assert the behavior
// std.serde promises rather than running another implementation as an oracle.

func TestCorpusDocumentFramingAndGrammar(t *testing.T) {
	valid := []string{
		`{}`,
		" \t\r\n{} \t\r\n",
		`{"string":"value"}`,
		`{"string":"value","bool":true,"slice":[1,2,3],"map":{"a":1}}`,
		`{"interface":{"nested":[true,false,null,{"x":1}]}}`,
	}
	for _, input := range valid {
		t.Run("accept_"+testName(input), func(t *testing.T) {
			var value CompatibilityValues
			if err := value.UnmarshalJSON([]byte(input)); err != nil {
				t.Fatalf("valid JSON rejected: %v", err)
			}
		})
	}

	invalid := []string{
		``, ` `, `null null`, `{]`, `[}`, `{`, `{"string"}`, `{"string":}`, `{:1}`,
		`{"string":"x",}`, `{"string":"x" "bool":true}`, `{"slice":[1,]}`,
		`{"slice":[,1]}`, `{"slice":[1 2]}`, `{"slice":[1;2]}`, `{"map":{"a":1,}}`,
		`{"string":"x"} trailing`, `{"string":"x"}{}`, `{'string':'x'}`,
		`{"bool":True}`, `{"bool":false // comment
}`, "{\"bool\":true\v}",
	}
	for _, input := range invalid {
		t.Run("reject_"+testName(input), func(t *testing.T) {
			var value CompatibilityValues
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid JSON accepted: %q", input)
			}
		})
	}
}

func TestCorpusNestingDepthAndMalformedInputSafety(t *testing.T) {
	accepted := `{"interface":` + strings.Repeat("[", 128) + `null` + strings.Repeat("]", 128) + `}`
	var value CompatibilityValues
	if err := value.UnmarshalJSON([]byte(accepted)); err != nil {
		t.Fatalf("reasonable nesting depth rejected: %v", err)
	}

	rejected := `{"interface":` + strings.Repeat("[", 10001) + `null` + strings.Repeat("]", 10001) + `}`
	if err := value.UnmarshalJSON([]byte(rejected)); err == nil {
		t.Fatal("document beyond the nesting limit was accepted")
	}

	malformed := [][]byte{
		nil,
		{0},
		bytes.Repeat([]byte{'['}, 1000),
		append([]byte(`{"interface":"`), bytes.Repeat([]byte{'\\'}, 1000)...),
	}
	for i, input := range malformed {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var target CompatibilityValues
			_ = target.UnmarshalJSON(input)
		})
	}
}

func TestCorpusStrings(t *testing.T) {
	valid := []struct {
		input string
		want  string
	}{
		{`{"string":""}`, ""},
		{`{"string":"quote: \" slash: \\ solidus: \/"}`, `quote: " slash: \ solidus: /`},
		{`{"string":"\b\f\n\r\t"}`, "\b\f\n\r\t"},
		{`{"string":"\u0041\u00e9"}`, "Aé"},
		{`{"string":"\ud834\udd1e"}`, "𝄞"},
		{`{"string":"日本語"}`, "日本語"},
	}
	for _, tc := range valid {
		t.Run(testName(tc.input), func(t *testing.T) {
			var value CompatibilityValues
			if err := value.UnmarshalJSON([]byte(tc.input)); err != nil {
				t.Fatal(err)
			}
			if value.String != tc.want {
				t.Fatalf("decoded string = %q, want %q", value.String, tc.want)
			}
		})
	}

	invalid := []string{
		`{"string":"unterminated}`, `{"string":"\x20"}`, `{"string":"\u12"}`,
		`{"string":"\uZZZZ"}`, "{\"string\":\"line\nbreak\"}",
	}
	for _, input := range invalid {
		t.Run("reject_"+testName(input), func(t *testing.T) {
			var value CompatibilityValues
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid string accepted: %q", input)
			}
		})
	}
}

func TestCorpusUnicodeReplacementPolicy(t *testing.T) {
	for _, input := range []string{`{"string":"\ud800"}`, `{"string":"\udc00"}`, `{"string":"\udc00\ud800"}`} {
		var value CompatibilityValues
		if err := value.UnmarshalJSON([]byte(input)); err != nil {
			t.Fatalf("decode %s: %v", input, err)
		}
		if !strings.ContainsRune(value.String, '\uFFFD') {
			t.Fatalf("decode %s = %q, want Unicode replacement rune", input, value.String)
		}
	}

	invalidUTF8 := [][]byte{
		{0x80},                   // unexpected continuation
		{0xc0, 0xaf},             // overlong encoding
		{0xe2, 0x82},             // truncated sequence
		{0xed, 0xa0, 0x80},       // UTF-8 encoding of a surrogate
		{0xf4, 0x90, 0x80, 0x80}, // above Unicode's maximum code point
	}
	for i, sequence := range invalidUTF8 {
		input := append([]byte(`{"string":"`), sequence...)
		input = append(input, []byte(`"}`)...)
		var value CompatibilityValues
		if err := value.UnmarshalJSON(input); err != nil {
			t.Fatalf("sequence %d: %v", i, err)
		}
		if !strings.ContainsRune(value.String, '\uFFFD') || !utf8.ValidString(value.String) {
			t.Fatalf("malformed UTF-8 sequence %d decoded incorrectly: %q", i, value.String)
		}
	}
}

func TestCorpusStringEncoding(t *testing.T) {
	value := CompatibilityValues{String: "\"\\\b\f\n\r\t\x00<>&\u2028\u2029"}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("encoder emitted invalid JSON: %q", data)
	}
	for _, escaped := range []string{`\"`, `\\`, `\b`, `\f`, `\n`, `\r`, `\t`, `\u0000`, `\u003c`, `\u003e`, `\u0026`, `\u2028`, `\u2029`} {
		if !bytes.Contains(data, []byte(escaped)) {
			t.Errorf("encoded string missing %q: %s", escaped, data)
		}
	}

	value.String = string([]byte{'a', 0xff, 'b'})
	data, err = value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !bytes.Contains(data, []byte(`\ufffd`)) {
		t.Fatalf("malformed Go string was not encoded as valid JSON with replacement: %q", data)
	}
}

func TestCorpusObjectKeyEscaping(t *testing.T) {
	input := `{"map":{"":0,"a\nb":1,"quote\"":2,"日本語":3}}`
	var value CompatibilityValues
	if err := value.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"": 0, "a\nb": 1, `quote"`: 2, "日本語": 3}
	if !reflect.DeepEqual(value.Map, want) {
		t.Fatalf("decoded keys = %#v, want %#v", value.Map, want)
	}
	encoded, err := value.MarshalJSON()
	if err != nil || !json.Valid(encoded) {
		t.Fatalf("key encoder emitted invalid JSON: %s, %v", encoded, err)
	}
}

func TestCorpusNumberGrammar(t *testing.T) {
	valid := []string{
		`0`, `-0`, `1`, `-1`, `1.5`, `-0.25`, `1e2`, `1E2`, `1e+2`, `1e-2`,
		`2.638344616030823e-256`, `5e-324`, `1.7976931348623157e308`,
	}
	for _, number := range valid {
		input := `{"float64":` + number + `}`
		var value CompatibilityNumbers
		if err := value.UnmarshalJSON([]byte(input)); err != nil {
			t.Errorf("valid number %q rejected: %v", number, err)
		}
	}

	invalid := []string{`00`, `01`, `-01`, `+1`, `.1`, `1.`, `1e`, `1e+`, `1e-`, `--1`, `0x10`, `NaN`, `Infinity`, `-Infinity`}
	for _, number := range invalid {
		t.Run("reject_"+testName(number), func(t *testing.T) {
			input := `{"float64":` + number + `}`
			var value CompatibilityNumbers
			if err := value.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("invalid number %q accepted as %v", number, value.Float64)
			}
		})
	}
}

func TestCorpusIntegerBoundsAndKinds(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"int8 overflow", `{"int8":128}`},
		{"int8 underflow", `{"int8":-129}`},
		{"uint8 overflow", `{"uint8":256}`},
		{"uint negative", `{"uint64":-1}`},
		{"int64 overflow", `{"int64":9223372036854775808}`},
		{"int64 underflow", `{"int64":-9223372036854775809}`},
		{"uint64 overflow", `{"uint64":18446744073709551616}`},
		{"integer fraction", `{"int64":1.5}`},
		{"integer exponent", `{"int64":1e2}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var value CompatibilityNumbers
			if err := value.UnmarshalJSON([]byte(tc.input)); err == nil {
				t.Fatalf("out-of-range or wrong-kind number accepted: %s", tc.input)
			}
		})
	}

	input := `{"int8":-128,"int16":-32768,"int32":-2147483648,"int64":-9223372036854775808,"uint8":255,"uint16":65535,"uint32":4294967295,"uint64":18446744073709551615}`
	var value CompatibilityNumbers
	if err := value.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatalf("valid integer boundaries rejected: %v", err)
	}
}

func TestCorpusFloatRoundTripsAndNonFiniteValues(t *testing.T) {
	values := []float64{0, math.Copysign(0, -1), math.SmallestNonzeroFloat64, math.MaxFloat64, 0.1, 2.638344616030823e-256}
	for _, want := range values {
		original := CompatibilityNumbers{Float64: want}
		data, err := original.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal %g: %v", want, err)
		}
		var decoded CompatibilityNumbers
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("unmarshal %g from %s: %v", want, data, err)
		}
		if math.Float64bits(decoded.Float64) != math.Float64bits(want) {
			t.Errorf("float round trip changed bits: %x => %x", math.Float64bits(want), math.Float64bits(decoded.Float64))
		}
	}

	float32Values := []float32{0, float32(math.Copysign(0, -1)), math.SmallestNonzeroFloat32, math.MaxFloat32, 0.1}
	for _, want := range float32Values {
		original := CompatibilityNumbers{Float32: want}
		data, err := original.MarshalJSON()
		if err != nil {
			t.Fatalf("marshal float32 %g: %v", want, err)
		}
		var decoded CompatibilityNumbers
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("unmarshal float32 %g from %s: %v", want, data, err)
		}
		if math.Float32bits(decoded.Float32) != math.Float32bits(want) {
			t.Errorf("float32 round trip changed bits: %x => %x", math.Float32bits(want), math.Float32bits(decoded.Float32))
		}
	}

	var underflow CompatibilityNumbers
	if err := underflow.UnmarshalJSON([]byte(`{"float64":1e-1000,"float32":1e-100}`)); err != nil {
		t.Fatalf("finite exponent underflow should decode to zero: %v", err)
	}
	if underflow.Float64 != 0 || underflow.Float32 != 0 {
		t.Fatalf("exponent underflow = (%g, %g), want zeroes", underflow.Float64, underflow.Float32)
	}
	for _, input := range []string{`{"float64":1e1000}`, `{"float64":-1e1000}`, `{"float32":1e100}`} {
		if err := new(CompatibilityNumbers).UnmarshalJSON([]byte(input)); err == nil {
			t.Errorf("exponent overflow accepted: %s", input)
		}
	}

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if data, err := (CompatibilityNumbers{Float64: value}).MarshalJSON(); err == nil {
			t.Errorf("non-finite value encoded without error as %q", data)
		}
	}
}

func TestCorpusIntegerEncodingAndNamedNumbers(t *testing.T) {
	original := CompatibilityNumbers{
		Int64: math.MinInt64, Uint64: math.MaxUint64,
		Named: NamedInt(math.MaxInt64), NamedUint: NamedUint(math.MaxUint32),
		NamedFloat: NamedFloat(math.SmallestNonzeroFloat32),
	}
	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, exact := range []string{
		`"int64":-9223372036854775808`, `"uint64":18446744073709551615`,
		`"named":9223372036854775807`, `"namedUint":4294967295`,
	} {
		if !bytes.Contains(data, []byte(exact)) {
			t.Errorf("encoded output missing exact integer %s: %s", exact, data)
		}
	}
	var decoded CompatibilityNumbers
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if decoded != original {
		t.Fatalf("named number round trip mismatch:\n got: %#v\nwant: %#v", decoded, original)
	}
}

func TestCorpusStructTagsAndVisibility(t *testing.T) {
	value := CompatibilityTagBehavior{
		Renamed: "yes", Ignored: "secret", EmptyName: "", OmitString: "", OmitInt: 0,
		QuotedInt: 42, Named: "named", Unexported: "visible", hidden: "secret",
	}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("invalid tagged output: %s", data)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"Ignored", "ignored", "EmptyName", "omitString", "omitInt", "hidden"} {
		if _, ok := object[absent]; ok {
			t.Errorf("field %q should have been omitted: %s", absent, data)
		}
	}
	if object["renamed"] != "yes" || object["named"] != "named" || object["unexported"] != "visible" {
		t.Errorf("tagged fields encoded incorrectly: %s", data)
	}
	if object["quotedInt"] != "42" {
		t.Errorf("string-tagged integer = %#v, want string 42", object["quotedInt"])
	}
}

func TestCorpusStructTagsMatchEncodingJSON(t *testing.T) {
	pointer := 7
	values := []CompatibilityTagBehavior{
		{},
		{
			Renamed: "renamed", EmptyName: "empty", Punctuation: "punctuation", EscapedName: "escaped",
			OmitString: "string", OmitBool: true, OmitInt: 1, OmitFloat: 1.5,
			OmitPointer: &pointer, OmitSlice: []int{1}, OmitMap: map[string]int{"a": 1},
			OmitInterface: []int{}, QuotedInt: 42, Named: "named", Unexported: "exported",
			Ignored: "ignored", hidden: "hidden",
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
		var gotObject, wantObject map[string]any
		if err := json.Unmarshal(got, &gotObject); err != nil {
			t.Fatalf("generated output is invalid: %s: %v", got, err)
		}
		if err := json.Unmarshal(want, &wantObject); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotObject, wantObject) {
			t.Fatalf("tag behavior differs from encoding/json:\n got: %s\nwant: %s", got, want)
		}
		var gotDecoded CompatibilityTagBehavior
		if err := gotDecoded.UnmarshalJSON(want); err != nil {
			t.Fatal(err)
		}
		var wantDecoded plain
		if err := json.Unmarshal(want, &wantDecoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plain(gotDecoded), wantDecoded) {
			t.Fatalf("tagged decode differs from encoding/json:\n got: %#v\nwant: %#v", gotDecoded, wantDecoded)
		}
		if value.EscapedName != "" && !bytes.Contains(got, []byte(`"key\u003c\u0026\u003e"`)) {
			t.Fatalf("generated field name was not HTML escaped: %s", got)
		}
	}
}

func TestCorpusCaseInsensitiveAndDuplicateFields(t *testing.T) {
	var value CompatibilityTagBehavior
	if err := value.UnmarshalJSON([]byte(`{"RENAMED":"upper"}`)); err != nil {
		t.Fatal(err)
	}
	if value.Renamed != "upper" {
		t.Fatalf("case-insensitive field resolution = %q, want upper", value.Renamed)
	}
	if err := value.UnmarshalJSON([]byte(`{"RENAMED":"upper","renamed":"exact","renamed":"last"}`)); err != nil {
		t.Fatal(err)
	}
	if value.Renamed != "last" {
		t.Fatalf("duplicate/exact field resolution = %q, want last", value.Renamed)
	}
	if err := value.UnmarshalJSON([]byte(`{"unknown":{"deep":[1,true,null]},"renamed":"known"}`)); err != nil {
		t.Fatalf("unknown field should be ignored after syntax validation: %v", err)
	}
	if value.Renamed != "known" {
		t.Fatalf("known field after unknown field = %q, want known", value.Renamed)
	}

	var containers CompatibilityValues
	if err := containers.UnmarshalJSON([]byte(`{"map":{"a":1},"map":{"b":2}}`)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(containers.Map, map[string]int{"a": 1, "b": 2}) {
		t.Fatalf("duplicate map fields did not merge in input order: %#v", containers.Map)
	}
}

func TestCorpusEmbeddedFieldResolution(t *testing.T) {
	value := CompatibilityEmbedding{CompatibilityEmbedded: CompatibilityEmbedded{Promoted: "p", Conflict: "embedded"}, Conflict: "outer"}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	if object["promoted"] != "p" || object["conflict"] != "outer" || len(object) != 2 {
		t.Fatalf("embedded field resolution incorrect: %s", data)
	}
}

func TestCorpusEmbeddedConflictsAndTaggedDominance(t *testing.T) {
	value := CompatibilityDominance{
		CompatibilityConflictA: CompatibilityConflictA{Conflict: "a", Plain: "plain"},
		CompatibilityConflictB: CompatibilityConflictB{Conflict: "b", Tagged: "tagged"},
	}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	if _, exists := object["conflict"]; exists {
		t.Fatalf("same-depth conflict should be omitted: %s", data)
	}
	if !reflect.DeepEqual(object, map[string]any{"Plain": "tagged"}) {
		t.Fatalf("tagged field did not dominate untagged field: %s", data)
	}

	var decoded CompatibilityDominance
	if err := decoded.UnmarshalJSON([]byte(`{"conflict":"ignored","Plain":"winner"}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.CompatibilityConflictA.Plain != "" || decoded.CompatibilityConflictB.Tagged != "winner" {
		t.Fatalf("decode dominance mismatch: %#v", decoded)
	}
}

func TestCorpusMissingNullAndPointers(t *testing.T) {
	a, b := 1, 2
	pointer := &a
	nested := &pointer
	value := CompatibilityValues{String: "keep", Pointer: &b, Nested: nested, Slice: []int{1}, Map: map[string]int{"a": 1}, Interface: "keep"}
	if err := value.UnmarshalJSON([]byte(`{"bool":true}`)); err != nil {
		t.Fatal(err)
	}
	if value.String != "keep" || value.Pointer == nil || *value.Pointer != 2 || value.Slice == nil || value.Map == nil || value.Interface != "keep" {
		t.Fatalf("missing fields did not preserve values: %#v", value)
	}

	if err := value.UnmarshalJSON([]byte(`{"string":null,"pointer":null,"nested":null,"slice":null,"map":null,"interface":null}`)); err != nil {
		t.Fatal(err)
	}
	if value.String != "keep" || value.Pointer != nil || value.Nested != nil || value.Slice != nil || value.Map != nil || value.Interface != nil {
		t.Fatalf("null semantics incorrect: %#v", value)
	}

	if err := value.UnmarshalJSON([]byte(`{"pointer":7,"nested":8}`)); err != nil {
		t.Fatal(err)
	}
	if value.Pointer == nil || *value.Pointer != 7 || value.Nested == nil || *value.Nested == nil || **value.Nested != 8 {
		t.Fatalf("present pointers were not allocated: %#v", value)
	}
}

func TestCorpusSlicesArraysBytesRawMessagesAndMaps(t *testing.T) {
	raw := json.RawMessage(`{"x":1}`)
	values := []CompatibilityValues{
		{},
		{Slice: []int{}, Map: map[string]int{}, Bytes: []byte{}},
		{Slice: []int{1, 2}, Array: [3]int{1, 2, 3}, Bytes: []byte{0, 1, 2, 255}, Raw: raw, Map: map[string]int{"b": 2, "a": 1}},
	}
	for _, original := range values {
		data, err := original.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(data) {
			t.Fatalf("container output is invalid JSON: %s", data)
		}
		if original.Bytes != nil && !bytes.Contains(data, []byte(`"`+base64.StdEncoding.EncodeToString(original.Bytes)+`"`)) {
			t.Errorf("[]byte is not base64 encoded: %s", data)
		}
		var decoded CompatibilityValues
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatal(err)
		}
		wantRaw := original.Raw
		if wantRaw == nil {
			wantRaw = json.RawMessage("null")
		}
		if !reflect.DeepEqual(decoded.Array, original.Array) || !bytes.Equal(decoded.Bytes, original.Bytes) || !bytes.Equal(decoded.Raw, wantRaw) {
			t.Errorf("container round trip mismatch:\n got: %#v\nwant array: %#v, bytes: %#v, raw: %s", decoded, original.Array, original.Bytes, wantRaw)
		}
	}

	first, err := (CompatibilityValues{Map: map[string]int{"z": 1, "a": 2, "m": 3}}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		next, err := (CompatibilityValues{Map: map[string]int{"z": 1, "a": 2, "m": 3}}).MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("map output is nondeterministic:\n%s\n%s", first, next)
		}
	}
}

func TestCorpusContainerSemanticsMatchEncodingJSON(t *testing.T) {
	type plain CompatibilityValues
	values := []CompatibilityValues{
		{},
		{Slice: []int{}, Map: map[string]int{}, NamedKeyMap: map[NamedMapKey]string{}, Raw: json.RawMessage("null")},
		{Slice: []int{1, 2}, Array: [3]int{1, 2, 3}, Map: map[string]int{"b": 2, "a": 1}, NamedKeyMap: map[NamedMapKey]string{"b": "2", "a": "1"}, Interface: []any{1.0, "x"}},
	}
	for _, value := range values {
		got, err := value.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(plain(value))
		if err != nil {
			t.Fatal(err)
		}
		var gotObject, wantObject any
		if err := json.Unmarshal(got, &gotObject); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(want, &wantObject); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotObject, wantObject) {
			t.Fatalf("container encoding differs:\n got: %s\nwant: %s", got, want)
		}
	}

	value := CompatibilityValues{Slice: []int{9}, Array: [3]int{9, 9, 9}, Map: map[string]int{"keep": 1}}
	if err := value.UnmarshalJSON([]byte(`{"slice":[],"array":[1,2],"map":{"new":2}}`)); err != nil {
		t.Fatal(err)
	}
	if value.Slice == nil || len(value.Slice) != 0 {
		t.Fatalf("empty array should decode to a non-nil empty slice: %#v", value.Slice)
	}
	if value.Array != [3]int{1, 2, 0} {
		t.Fatalf("short array decode = %#v, want [1 2 0]", value.Array)
	}
	if !reflect.DeepEqual(value.Map, map[string]int{"keep": 1, "new": 2}) {
		t.Fatalf("map decode should merge existing entries: %#v", value.Map)
	}
	if err := value.UnmarshalJSON([]byte(`{"array":[1,2,3,4]}`)); err != nil {
		t.Fatal(err)
	}
	if value.Array != [3]int{1, 2, 3} {
		t.Fatalf("long array decode = %#v, want first three elements", value.Array)
	}

}

func TestCorpusDecodeFailureDoesNotPartiallyModifyReceiver(t *testing.T) {
	before := CompatibilityValues{String: "before", Bool: false, Slice: []int{9}, Map: map[string]int{"keep": 1}}
	value := before
	if err := value.UnmarshalJSON([]byte(`{"string":"changed","bool":"wrong"}`)); err == nil {
		t.Fatal("wrong JSON kind accepted")
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("receiver changed after failure:\n got: %#v\nwant: %#v", value, before)
	}

	pointee := 5
	value = CompatibilityValues{
		Pointer: &pointee,
		Slice:   []int{9, 8},
		Map:     map[string]int{"keep": 1},
	}
	if err := value.UnmarshalJSON([]byte(`{"slice":[1,2,3],"map":{"new":2},"pointer":7,"bool":"wrong"}`)); err == nil {
		t.Fatal("wrong JSON kind accepted after mutable fields")
	}
	if pointee != 5 || value.Pointer == nil || *value.Pointer != 5 ||
		!reflect.DeepEqual(value.Slice, []int{9, 8}) ||
		!reflect.DeepEqual(value.Map, map[string]int{"keep": 1}) {
		t.Fatalf("mutable receiver state changed after failure: %#v (original pointee %d)", value, pointee)
	}

	value = CompatibilityValues{
		NamedKeyMap: map[NamedMapKey]string{"keep": "1"},
		Interface:   map[string]any{"keep": true},
	}
	if err := value.UnmarshalJSON([]byte(`{"namedKeyMap":{"new":"2"},"interface":{"new":true},"bool":"wrong"}`)); err == nil {
		t.Fatal("wrong JSON kind accepted after fallback containers")
	}
	if !reflect.DeepEqual(value.NamedKeyMap, map[NamedMapKey]string{"keep": "1"}) ||
		!reflect.DeepEqual(value.Interface, map[string]any{"keep": true}) {
		t.Fatalf("fallback container state changed after failure: %#v", value)
	}
}

func TestCorpusCustomInterfacePrecedenceAndNilPointers(t *testing.T) {
	pointer := CustomBoth("original")
	value := CustomBothEnvelope{Value: "original", Pointer: &pointer}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"value":"json","pointer":"json"}` {
		t.Fatalf("json.Marshaler did not take precedence over TextMarshaler: %s", data)
	}

	var decoded CustomBothEnvelope
	if err := decoded.UnmarshalJSON([]byte(`{"value":"input","pointer":"input"}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Value != "json" || decoded.Pointer == nil || *decoded.Pointer != "json" {
		t.Fatalf("json.Unmarshaler did not take precedence over TextUnmarshaler: %#v", decoded)
	}
	decoded.Pointer = &pointer
	if err := decoded.UnmarshalJSON([]byte(`{"pointer":null}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Pointer != nil {
		t.Fatalf("null custom pointer = %#v, want nil", decoded.Pointer)
	}
	data, err = (CustomBothEnvelope{}).MarshalJSON()
	if err != nil || !bytes.Contains(data, []byte(`"pointer":null`)) {
		t.Fatalf("nil custom pointer should encode as null: %s, %v", data, err)
	}
}

func TestCorpusCustomInterfaceErrorsAreAtomic(t *testing.T) {
	shared := "original"
	value := CustomFailureEnvelope{Before: "original", Failure: CustomFailure{Shared: &shared}}
	if data, err := value.MarshalJSON(); err == nil || data != nil {
		t.Fatalf("custom marshal failure = (%q, %v), want nil output and error", data, err)
	}
	if err := value.UnmarshalJSON([]byte(`{"before":"changed","failure":{}}`)); err == nil {
		t.Fatal("custom unmarshal failure was ignored")
	}
	if value.Before != "original" || value.Failure.Shared == nil || *value.Failure.Shared != "original" || shared != "original" {
		t.Fatalf("custom failure corrupted receiver state: %#v, shared=%q", value, shared)
	}
}

func TestCorpusCyclesReturnErrors(t *testing.T) {
	if os.Getenv("METAGO_SERDE_CYCLE_CHILD") == "1" {
		// Bound the failure in a subprocess so a missing cycle guard cannot exhaust the test runner's
		// entire stack. A correct implementation returns an error and lets the child exit normally.
		debug.SetMaxStack(1 << 20)
		value := &CompatibilityCycle{Value: "root"}
		value.Next = value
		if _, err := value.MarshalJSON(); err == nil {
			os.Exit(2)
		}
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestCorpusCyclesReturnErrors$")
	command.Env = append(os.Environ(), "METAGO_SERDE_CYCLE_CHILD=1")
	if err := command.Run(); err != nil {
		t.Fatalf("cyclic value did not return an error safely: %v", err)
	}
}

func testName(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return "empty"
	}
	var b strings.Builder
	for _, r := range input {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() == 0 || b.String()[b.Len()-1] != '_' {
			b.WriteByte('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	return b.String()
}
