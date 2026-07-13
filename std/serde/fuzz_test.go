package serde

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf8"
)

func FuzzCompatibilityValuesUnmarshal(f *testing.F) {
	addCuratedCorpusSeeds(f)

	// Supplement the file corpus with generated-path bounds, malformed UTF-8,
	// Unicode replacement, and a previously fragile nested-container shape.
	for _, seed := range [][]byte{
		[]byte(`{"int8Slice":[128]}`),
		[]byte(`{"string":"\ud800"}`),
		append([]byte(`{"string":"`), 0xff, '"', '}'),
		[]byte(`{"addressPointerMap":{"home":{"street":"s","city":"c","zip":"z"}}}`),
		[]byte(`{"namedStringPtr":"text","namedIntNested":42,"namedStringSlice":["a"],"namedBoolSlice":[true],"namedIntSlice":[-1],"namedStringPtrs":[null,"b"],"namedUintArray":[0,4294967295],"namedFloatMap":{"x":1.5},"namedIntPtrMap":{"nil":null,"x":7}}`),
		[]byte(`{"namedIntSliceMap":{"x":[null,-1]},"byteSliceMap":{"x":"AAH/"},"namedUintArrayMap":{"x":[1,2]},"nestedScalarMap":{"x":{"a":1,"zero":null}}}`),
		[]byte(`{"addressSliceMap":{"x":[null,{"city":"a"}]},"addressPtrSliceMap":{"x":[null,{"city":"b"}]},"addressArrayMap":{"x":[{"city":"a"}]},"nestedAddressMap":{"x":{"a":{"city":"a"}}},"nestedAddressPtrMap":{"x":{"nil":null,"a":{"city":"b"}}}}`),
		[]byte(`{"namedIntNestedMap":{"nil":null,"x":7},"namedIntPtrSliceMap":{"x":[null,7]},"rawMap":{"nil":null,"x":{"nested":[1,true]}}}`),
		[]byte(`{"bytes":[0,1,null,255],"namedByteSlice":"aGVsbG8=","namedByteSliceMap":{"escaped":"c3ViamVjdHM\/X2Q9MQ=="}}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		pointer := 42
		value := CompatibilityValues{
			String:  "unchanged",
			Pointer: &pointer,
			Slice:   []int{1, 2, 3},
			Map:     map[string]int{"before": 1},
		}
		wantOnFailure := CompatibilityValues{
			String:  "unchanged",
			Pointer: &pointer,
			Slice:   []int{1, 2, 3},
			Map:     map[string]int{"before": 1},
		}

		if err := value.UnmarshalJSON(data); err != nil && !reflect.DeepEqual(value, wantOnFailure) {
			t.Fatalf("failed decode modified receiver: got %#v, want %#v", value, wantOnFailure)
		}
	})
}

func FuzzCompatibilityValuesDifferential(f *testing.F) {
	addCuratedCorpusSeeds(f)
	f.Add([]byte(`{"namedStringPtr":"text","namedIntNested":42,"namedStringSlice":["a"],"namedBoolSlice":[true],"namedIntSlice":[-1],"namedStringPtrs":[null,"b"],"namedUintArray":[0,4294967295],"namedFloatMap":{"x":1.5},"namedIntPtrMap":{"nil":null,"x":7}}`))
	f.Add([]byte(`{"namedIntSliceMap":{"x":[null,-1]},"byteSliceMap":{"x":"AAH/"},"namedUintArrayMap":{"x":[1,2]},"nestedScalarMap":{"x":{"a":1,"zero":null}}}`))
	f.Add([]byte(`{"addressSliceMap":{"x":[null,{"city":"a"}]},"addressPtrSliceMap":{"x":[null,{"city":"b"}]},"addressArrayMap":{"x":[{"city":"a"}]},"nestedAddressMap":{"x":{"a":{"city":"a"}}},"nestedAddressPtrMap":{"x":{"nil":null,"a":{"city":"b"}}}}`))
	f.Add([]byte(`{"namedIntNestedMap":{"nil":null,"x":7},"namedIntPtrSliceMap":{"x":[null,7]},"rawMap":{"nil":null,"x":{"nested":[1,true]}}}`))
	f.Add([]byte(`{"bytes":[0,1,null,255],"namedByteSlice":"aGVsbG8=","namedByteSliceMap":{"escaped":"c3ViamVjdHM\/X2Q9MQ=="}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		type plain CompatibilityValues

		var generated CompatibilityValues
		generatedErr := generated.UnmarshalJSON(data)
		var standard plain
		standardErr := json.Unmarshal(data, &standard)

		if (generatedErr == nil) != (standardErr == nil) {
			t.Fatalf("acceptance differs: generated error=%v, encoding/json error=%v, input=%q", generatedErr, standardErr, data)
		}
		if generatedErr == nil && !reflect.DeepEqual(plain(generated), standard) {
			t.Fatalf("decoded values differ:\n generated: %#v\nencoding/json: %#v\ninput: %q", generated, standard, data)
		}
	})
}

func FuzzCompatibilityNumbersDifferential(f *testing.F) {
	for _, subject := range jsonTestSuiteAmbiguousSubjects() {
		if subject.kind == "number" {
			f.Add(adaptedAmbiguousInput(subject))
		}
	}
	f.Add([]byte(`{"int8":127,"uint8":255,"float32":1.5,"float64":-0}`))
	f.Add([]byte(`{"float64":18446744073709551615.}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		type plain CompatibilityNumbers

		var generated CompatibilityNumbers
		generatedErr := generated.UnmarshalJSON(data)
		var standard plain
		standardErr := json.Unmarshal(data, &standard)
		if (generatedErr == nil) != (standardErr == nil) {
			t.Fatalf("acceptance differs: generated error=%v, encoding/json error=%v, input=%q", generatedErr, standardErr, data)
		}
		if generatedErr == nil && !reflect.DeepEqual(plain(generated), standard) {
			t.Fatalf("decoded values differ:\n generated: %#v\nencoding/json: %#v\ninput: %q", generated, standard, data)
		}
	})
}

func FuzzCompatibilityAnonymousDifferential(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"value":{"name":"value","both":"json","json":"marshaled:x","text":"text:x"},"pointer":{"count":7},"tail":9}`),
		[]byte(`{"value":null,"pointer":null,"tail":"wrong"}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		type plain CompatibilityAnonymous
		var generated CompatibilityAnonymous
		generatedErr := generated.UnmarshalJSON(data)
		var standard plain
		standardErr := json.Unmarshal(data, &standard)
		if (generatedErr == nil) != (standardErr == nil) {
			t.Fatalf("acceptance differs: generated error=%v, encoding/json error=%v, input=%q", generatedErr, standardErr, data)
		}
		if generatedErr == nil && !reflect.DeepEqual(plain(generated), standard) {
			t.Fatalf("decoded anonymous values differ:\n generated: %#v\nencoding/json: %#v\ninput: %q", generated, standard, data)
		}
	})
}

func FuzzCompatibilityAnonymousPromotionDifferential(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"promoted":"value","conflict":"embedded","outer":"outer"}`),
		[]byte(`{"promoted":"changed","outer":1}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		type plain CompatibilityAnonymousPromotion
		var generated CompatibilityAnonymousPromotion
		generatedErr := generated.UnmarshalJSON(data)
		var standard plain
		standardErr := json.Unmarshal(data, &standard)
		if (generatedErr == nil) != (standardErr == nil) {
			t.Fatalf("acceptance differs: generated error=%v, encoding/json error=%v, input=%q", generatedErr, standardErr, data)
		}
		if generatedErr == nil && !reflect.DeepEqual(plain(generated), standard) {
			t.Fatalf("decoded promoted values differ:\n generated: %#v\nencoding/json: %#v\ninput: %q", generated, standard, data)
		}
	})
}

func addCuratedCorpusSeeds(f *testing.F) {
	f.Helper()

	for _, category := range []string{"accepted", "rejected", "ambiguous"} {
		paths, err := filepath.Glob(filepath.Join("testdata", category, "*.json"))
		if err != nil {
			f.Fatal(err)
		}
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(data)
		}
	}
	for _, subject := range jsonTestSuiteAmbiguousSubjects() {
		f.Add(adaptedAmbiguousInput(subject))
	}
}

func FuzzCompatibilityValuesMarshalRoundTrip(f *testing.F) {
	f.Add("hello", true, int64(42))
	f.Add("<>&\u2028\u2029", false, int64(math.MinInt64))
	f.Add("日本語", true, int64(math.MaxInt64))

	f.Fuzz(func(t *testing.T, text string, boolean bool, integer int64) {
		if !utf8.ValidString(text) {
			t.Skip()
		}

		pointer := int(integer)
		namedString := NamedString(text)
		namedInt := NamedInt(integer)
		namedIntPointer := &namedInt
		address := Address{City: text}
		rawText, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		value := CompatibilityValues{
			String:           text,
			Bool:             boolean,
			Pointer:          &pointer,
			Slice:            []int{pointer, 0},
			Map:              map[string]int{text: pointer},
			NamedStringPtr:   &namedString,
			NamedStringSlice: []NamedString{namedString},
			NamedBoolSlice:   []NamedBool{NamedBool(boolean)},
			NamedIntSlice:    []NamedInt{namedInt},
			NamedStringPtrs:  []*NamedString{nil, &namedString},
			NamedUintArray:   [2]NamedUint{0, NamedUint(uint32(integer))},
			NamedFloatMap:    map[string]NamedFloat{text: 1.5},
			NamedIntPtrMap:   map[string]*NamedInt{"value": &namedInt},
			NamedIntNestedMap: map[string]**NamedInt{
				"value": &namedIntPointer,
			},
			NamedIntPtrSliceMap: map[string][]*NamedInt{
				"value": {nil, &namedInt},
			},
			RawMap:           map[string]json.RawMessage{"value": rawText},
			NamedIntSliceMap: map[string][]NamedInt{text: {namedInt}},
			ByteSliceMap:     map[string][]byte{text: []byte(text)},
			NamedUintArrayMap: map[string][2]NamedUint{
				text: {0, NamedUint(uint32(integer))},
			},
			NestedScalarMap: map[string]map[NamedMapKey]int8{
				text: {NamedMapKey(text): int8(integer)},
			},
			AddressSliceMap:    map[string][]Address{text: {address}},
			AddressPtrSliceMap: map[string][]*Address{text: {nil, &address}},
			AddressArrayMap:    map[string][2]Address{text: {address, {}}},
			NestedAddressMap:   map[string]map[string]Address{text: {text: address}},
			NestedAddressPtrMap: map[string]map[string]*Address{
				text: {"nil": nil, text: &address},
			},
		}
		data, err := value.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(data) {
			t.Fatalf("MarshalJSON emitted invalid JSON: %q", data)
		}
		type plain CompatibilityValues
		standard, standardErr := json.Marshal(plain(value))
		if standardErr != nil {
			t.Fatalf("encoding/json rejected supported value: %v", standardErr)
		}
		var generatedJSON any
		var standardJSON any
		if err := json.Unmarshal(data, &generatedJSON); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(standard, &standardJSON); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(generatedJSON, standardJSON) {
			t.Fatalf("semantic encoding differs:\n generated: %s\nencoding/json: %s", data, standard)
		}
		var decoded CompatibilityValues
		if err := decoded.UnmarshalJSON(data); err != nil {
			t.Fatalf("decode encoded value: %v", err)
		}
		if decoded.String != value.String || decoded.Bool != value.Bool ||
			!reflect.DeepEqual(decoded.Pointer, value.Pointer) ||
			!reflect.DeepEqual(decoded.Slice, value.Slice) ||
			!reflect.DeepEqual(decoded.Map, value.Map) ||
			!reflect.DeepEqual(namedScalarFixture(decoded), namedScalarFixture(value)) ||
			!reflect.DeepEqual(nestedMapFixture(decoded), nestedMapFixture(value)) ||
			!reflect.DeepEqual(generatedContainerMapFixture(decoded), generatedContainerMapFixture(value)) ||
			!reflect.DeepEqual(advancedMapFixture(decoded), advancedMapFixture(value)) {
			t.Fatalf("round trip mismatch: got %#v, want %#v", decoded, value)
		}
	})
}
