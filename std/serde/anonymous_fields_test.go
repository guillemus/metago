package serde

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTaggedAnonymousFieldsMatchEncodingJSON(t *testing.T) {
	value := CompatibilityAnonymous{
		CompatibilityAnonymousValue: CompatibilityAnonymousValue{
			Name: "value", Both: "input", JSON: "json-value", Text: "text-value",
		},
		CompatibilityAnonymousPointer: &CompatibilityAnonymousPointer{Count: 7},
		Tail:                          9,
	}
	type plain CompatibilityAnonymous
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(plain(value))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("tagged anonymous encoding differs:\n got: %s\nwant: %s", got, want)
	}
	if !bytes.Contains(got, []byte(`"value":{"name":"value","both":"json","json":"marshaled:json-value","text":"text:text-value"}`)) ||
		!bytes.Contains(got, []byte(`"pointer":{"count":7}`)) {
		t.Fatalf("anonymous fields were promoted instead of using explicit tags: %s", got)
	}

	var decoded CompatibilityAnonymous
	if err := decoded.UnmarshalJSON(got); err != nil {
		t.Fatal(err)
	}
	var standard plain
	if err := json.Unmarshal(got, &standard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain(decoded), standard) {
		t.Fatalf("tagged anonymous decoding differs:\n got: %#v\nwant: %#v", decoded, standard)
	}
	if decoded.Both != "json" || decoded.JSON != "json-value:unmarshaled" || decoded.Text != "text-value:unmarshaled" {
		t.Fatalf("custom interface precedence was not preserved: %#v", decoded.CompatibilityAnonymousValue)
	}
}

func TestAnonymousPointerAllocationNullAndOmission(t *testing.T) {
	value := CompatibilityAnonymous{CompatibilityAnonymousValue: CompatibilityAnonymousValue{Name: "value"}}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"pointer"`)) {
		t.Fatalf("nil tagged anonymous pointer was not omitted: %s", data)
	}

	if err := value.UnmarshalJSON([]byte(`{"pointer":{"count":12}}`)); err != nil {
		t.Fatal(err)
	}
	if value.CompatibilityAnonymousPointer == nil || value.Count != 12 {
		t.Fatalf("tagged anonymous pointer was not allocated: %#v", value)
	}
	if err := value.UnmarshalJSON([]byte(`{"pointer":null}`)); err != nil {
		t.Fatal(err)
	}
	if value.CompatibilityAnonymousPointer != nil {
		t.Fatalf("tagged anonymous pointer null did not clear field: %#v", value)
	}
}

func TestAnonymousPointerPromotionMatchesEncodingJSON(t *testing.T) {
	input := []byte(`{"promoted":"value","conflict":"embedded","outer":"outer"}`)
	var got CompatibilityAnonymousPromotion
	if err := got.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}
	type plain CompatibilityAnonymousPromotion
	var want plain
	if err := json.Unmarshal(input, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain(got), want) {
		t.Fatalf("anonymous pointer promotion differs:\n got: %#v\nwant: %#v", got, want)
	}
	if got.CompatibilityEmbedded == nil || got.Promoted != "value" {
		t.Fatalf("promoted anonymous pointer was not allocated: %#v", got)
	}

	gotJSON, err := got.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("anonymous pointer promotion encoding differs:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestAnonymousPointerFallbackFailuresAreAtomic(t *testing.T) {
	beforeTagged := CompatibilityAnonymous{
		CompatibilityAnonymousValue:   CompatibilityAnonymousValue{Name: "keep"},
		CompatibilityAnonymousPointer: &CompatibilityAnonymousPointer{Count: 1},
		Tail:                          2,
	}
	tagged := beforeTagged
	if err := tagged.UnmarshalJSON([]byte(`{"pointer":{"count":9},"tail":"wrong"}`)); err == nil {
		t.Fatal("invalid tagged anonymous value accepted")
	}
	if !reflect.DeepEqual(tagged, beforeTagged) {
		t.Fatalf("failed tagged anonymous decode changed receiver:\n got: %#v\nwant: %#v", tagged, beforeTagged)
	}

	beforePromoted := CompatibilityAnonymousPromotion{
		CompatibilityEmbedded: &CompatibilityEmbedded{Promoted: "keep", Conflict: "keep"},
		Outer:                 "keep",
	}
	promoted := beforePromoted
	if err := promoted.UnmarshalJSON([]byte(`{"promoted":"changed","outer":1}`)); err == nil {
		t.Fatal("invalid promoted anonymous value accepted")
	}
	if !reflect.DeepEqual(promoted, beforePromoted) {
		t.Fatalf("failed promoted anonymous decode changed receiver:\n got: %#v\nwant: %#v", promoted, beforePromoted)
	}
}

func TestAnonymousFieldsUseCanonicalFallback(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"type jsonAlias CompatibilityAnonymous",
		"clonedCompatibilityAnonymousPointer := *next.CompatibilityAnonymousPointer",
		"type jsonAlias CompatibilityAnonymousPromotion",
		"clonedCompatibilityEmbedded := *next.CompatibilityEmbedded",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("anonymous fallback source missing %q", want)
		}
	}
}
