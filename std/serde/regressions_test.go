package serde

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestCrossImplementationRegressions curates observable cases from the
// implementations listed in compatibility.md. Exact sources and revisions are
// recorded in testdata/PROVENANCE.md.
func TestCrossImplementationRegressions(t *testing.T) {
	t.Run("go issue 7046 string-tagged null", func(t *testing.T) {
		before := CompatibilityTagBehavior{QuotedInt: 7, QuotedUint: 8, QuotedFloat: 9}
		got := before
		if err := got.UnmarshalJSON([]byte(`{"quotedInt":null,"quotedUint":null,"quotedFloat":null}`)); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatalf("string-tagged null changed scalars: got %#v, want %#v", got, before)
		}
	})

	t.Run("serde_json issue 953 trailing decimal", func(t *testing.T) {
		before := CompatibilityNumbers{Float64: 7}
		got := before
		if err := got.UnmarshalJSON([]byte(`{"float64":18446744073709551615.}`)); err == nil {
			t.Fatal("number with decimal point but no fraction digit was accepted")
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatalf("invalid decimal changed receiver: got %#v, want %#v", got, before)
		}
	})

	t.Run("serde_json issue 1004 float32 formatting", func(t *testing.T) {
		got, err := (CompatibilityNumbers{Float32: 5.55}).MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal(got, &object); err != nil {
			t.Fatal(err)
		}
		if object["float32"] != 5.55 {
			t.Fatalf("float32 encoded as %#v, want 5.55", object["float32"])
		}
	})

	t.Run("sonic and Go issue 12921 named uint8 slices", func(t *testing.T) {
		want := CompatibilityValues{
			NamedByteSlice:    []NamedByte("hello"),
			NamedByteSliceMap: map[string][]NamedByte{"value": []NamedByte("world")},
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
			t.Fatalf("named uint8 encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
		}
		var decoded CompatibilityValues
		if err := decoded.UnmarshalJSON(gotJSON); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decoded.NamedByteSlice, want.NamedByteSlice) || !reflect.DeepEqual(decoded.NamedByteSliceMap, want.NamedByteSliceMap) {
			t.Fatalf("named uint8 round trip differs: %#v", decoded)
		}
	})

	t.Run("goccy issue 360 byte slice numeric array", func(t *testing.T) {
		var got CompatibilityValues
		if err := got.UnmarshalJSON([]byte(`{"bytes":[0,1,null,255]}`)); err != nil {
			t.Fatal(err)
		}
		if want := []byte{0, 1, 0, 255}; !reflect.DeepEqual(got.Bytes, want) {
			t.Fatalf("numeric byte array = %v, want %v", got.Bytes, want)
		}
		before := got
		if err := got.UnmarshalJSON([]byte(`{"bytes":[256]}`)); err == nil {
			t.Fatal("byte overflow was accepted")
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatalf("byte overflow changed receiver: got %#v, want %#v", got, before)
		}
	})

	t.Run("jsoniter raw message ownership issue", func(t *testing.T) {
		input := []byte(`{"raw":{"open":true,"list":["a",2,null]}}`)
		var got CompatibilityValues
		if err := got.UnmarshalJSON(input); err != nil {
			t.Fatal(err)
		}
		want := append(json.RawMessage(nil), got.Raw...)
		for i := range input {
			input[i] = 'x'
		}
		if !reflect.DeepEqual(got.Raw, want) {
			t.Fatalf("RawMessage aliases decoder input: got %q, want %q", got.Raw, want)
		}
	})

	t.Run("easyjson escaped solidus base64", func(t *testing.T) {
		var got CompatibilityValues
		if err := got.UnmarshalJSON([]byte(`{"bytes":"c3ViamVjdHM\/X2Q9MQ=="}`)); err != nil {
			t.Fatal(err)
		}
		if string(got.Bytes) != "subjects?_d=1" {
			t.Fatalf("escaped-solidus base64 decoded to %q", got.Bytes)
		}
	})

	t.Run("security alternating nesting depth", func(t *testing.T) {
		const depth = 10001
		var b strings.Builder
		b.Grow(depth*6 + 32)
		b.WriteString(`{"interface":`)
		for i := 0; i < depth; i++ {
			if i%2 == 0 {
				b.WriteByte('[')
			} else {
				b.WriteString(`{"x":`)
			}
		}
		b.WriteString("null")
		for i := depth - 1; i >= 0; i-- {
			if i%2 == 0 {
				b.WriteByte(']')
			} else {
				b.WriteByte('}')
			}
		}
		b.WriteByte('}')

		before := CompatibilityValues{String: "unchanged"}
		got := before
		if err := got.UnmarshalJSON([]byte(b.String())); err == nil {
			t.Fatal("adversarial nesting beyond the configured limit was accepted")
		}
		if !reflect.DeepEqual(got, before) {
			t.Fatalf("depth failure changed receiver: got %#v, want %#v", got, before)
		}
	})
}

func TestRegressionGeneratedPathsAvoidFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"rawBytes := make([]byte, len(v.NamedByteSlice))",
		"decodedBytes := l.Bytes()",
		"rawBytes := make([]byte, len(me))",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("regression generated path missing %q", want)
		}
	}
	for _, field := range []string{"Bytes", "NamedByteSlice", "NamedByteSliceMap"} {
		for _, operation := range []string{"json.Marshal(v." + field + ")", "json.Unmarshal(raw, &v." + field + ")"} {
			if strings.Contains(generated, operation) {
				t.Errorf("regression field %s unexpectedly uses fallback %q", field, operation)
			}
		}
	}
}
