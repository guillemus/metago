package serde

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func sampleUser() User {
	return User{
		ID:     9007199254740993,
		Name:   `Guillem "G" O'Neill \ 日本語 🚀`,
		Email:  "g@example.com",
		Age:    30,
		Active: true,
		Score:  -12.75,
		Tags:   []string{"a", "tab\there", "line\nbreak", ""},
		Address: Address{
			Street: "C/ Major 1",
			City:   "Barcelona",
			Zip:    "08001",
		},
		Items: []Item{
			{SKU: "sku-1", Qty: 2, Price: 9.99},
			{SKU: "sku-2", Qty: 0, Price: 0},
		},
		Metadata: map[string]string{"k1": "v1", "k2": "v2"},
	}
}

func toStd(t *testing.T, u User) StdUser {
	t.Helper()
	var s StdUser
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// Our Marshal output must decode with encoding/json to the same value.
func TestMarshalMatchesStdlib(t *testing.T) {
	u := sampleUser()
	data, err := u.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got StdUser
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("stdlib cannot parse our output: %v\noutput: %s", err, data)
	}
	if want := toStd(t, u); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// Our Unmarshal must decode stdlib output to the same value.
func TestUnmarshalMatchesStdlib(t *testing.T) {
	want := sampleUser()
	data, err := json.Marshal(toStd(t, want))
	if err != nil {
		t.Fatal(err)
	}
	var got User
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatalf("cannot parse stdlib output: %v\ninput: %s", err, data)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decode mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestUnmarshalHandlesWhitespaceEscapesAndUnknownKeys(t *testing.T) {
	input := `{
		"unknown_object": {"deep": [1, "two", {"three": 3}]},
		"name" : "line\nbreak é 🚀 \\" ,
		"id": -42,
		"unknown_array": [ [1,2], "x" ],
		"tags": [ "a" , "b" ],
		"address": { "city": "BCN", "unknown": null },
		"score": 1.5e2
	}`
	var got User
	if err := got.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if got.Name != "line\nbreak é 🚀 \\" {
		t.Fatalf("escape decoding mismatch: %q", got.Name)
	}
	if got.ID != -42 || got.Score != 150 || got.Address.City != "BCN" {
		t.Fatalf("scalar mismatch: %+v", got)
	}
	if !reflect.DeepEqual(got.Tags, []string{"a", "b"}) {
		t.Fatalf("tags mismatch: %#v", got.Tags)
	}
}

func TestUnmarshalNullSemantics(t *testing.T) {
	got := sampleUser()
	input := `{"name": null, "tags": null, "metadata": null, "address": null}`
	if err := got.UnmarshalJSON([]byte(input)); err != nil {
		t.Fatal(err)
	}
	want := sampleUser()
	// Like encoding/json: null is a no-op for strings and structs, nils
	// slices and maps.
	want.Tags = nil
	want.Metadata = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("null semantics mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	cases := map[string]string{
		"trailing data":    `{"id": 1} extra`,
		"trailing comma":   `{"id": 1,}`,
		"missing colon":    `{"id" 1}`,
		"unterminated":     `{"name": "abc`,
		"float into int":   `{"id": 1.5}`,
		"integer overflow": `{"id": 99999999999999999999}`,
		"bad escape":       `{"name": "\x"}`,
		"bare garbage":     `{"id": nope}`,
		"missing comma":    `{"id": 1 "name": "x"}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var u User
			if err := u.UnmarshalJSON([]byte(input)); err == nil {
				t.Fatalf("expected error for %s", input)
			}
		})
	}
}

func TestNestedSerdeUsesGeneratedCodec(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{"v.Address.appendJSON(b)", "v.Address.unmarshalJSONLexer(l)"} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated nested serde path missing %q", want)
		}
	}
	for _, unwanted := range []string{"json.Marshal(v.Address)", "json.Unmarshal(raw, &v.Address)"} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("nested serde type unexpectedly uses encoding/json fallback %q", unwanted)
		}
	}
}

func TestCustomJSONMarshalerAndUnmarshalerFields(t *testing.T) {
	pointer := CustomJSON("pointer")
	value := CustomJSONEnvelope{Value: "value", Pointer: &pointer}

	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	const wantJSON = `{"value":"marshaled:value","pointer":"marshaled:pointer"}`
	if string(data) != wantJSON {
		t.Fatalf("MarshalJSON() = %s, want %s", data, wantJSON)
	}

	var decoded CustomJSONEnvelope
	if err := decoded.UnmarshalJSON([]byte(`{"value":"first","pointer":"second"}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Value != "first:unmarshaled" {
		t.Fatalf("custom value UnmarshalJSON was not used: %q", decoded.Value)
	}
	if decoded.Pointer == nil || *decoded.Pointer != "second:unmarshaled" {
		t.Fatalf("custom pointer UnmarshalJSON was not used: %#v", decoded.Pointer)
	}

	if err := decoded.UnmarshalJSON([]byte(`{"pointer":null}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Pointer != nil {
		t.Fatalf("null custom pointer = %#v, want nil", decoded.Pointer)
	}
}

func TestCustomTextMarshalerAndUnmarshalerFields(t *testing.T) {
	pointer := CustomText("pointer")
	value := CustomTextEnvelope{Value: "value", Pointer: &pointer}

	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	const wantJSON = `{"value":"text:value","pointer":"text:pointer"}`
	if string(data) != wantJSON {
		t.Fatalf("MarshalJSON() = %s, want %s", data, wantJSON)
	}

	var decoded CustomTextEnvelope
	if err := decoded.UnmarshalJSON([]byte(`{"value":"text:first","pointer":"text:second"}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Value != "first:unmarshaled" {
		t.Fatalf("custom value UnmarshalText was not used: %q", decoded.Value)
	}
	if decoded.Pointer == nil || *decoded.Pointer != "second:unmarshaled" {
		t.Fatalf("custom pointer UnmarshalText was not used: %#v", decoded.Pointer)
	}

	if err := decoded.UnmarshalJSON([]byte(`{"pointer":null}`)); err != nil {
		t.Fatal(err)
	}
	if decoded.Pointer != nil {
		t.Fatalf("null custom text pointer = %#v, want nil", decoded.Pointer)
	}
}

func TestMarshalEscapesControlCharacters(t *testing.T) {
	u := User{Name: "a\x01b\"c\\d"}
	data, err := u.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `a\u0001b\"c\\d`) {
		t.Fatalf("escape output mismatch: %s", data)
	}
	var back User
	if err := back.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if back.Name != u.Name {
		t.Fatalf("escape round trip mismatch: %q != %q", back.Name, u.Name)
	}
}

func TestFeedRoundTrip(t *testing.T) {
	feed := Feed{Users: []User{sampleUser(), {Name: "empty"}}}
	data, err := feed.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got Feed
	if err := got.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, feed) {
		t.Fatalf("feed round trip mismatch\n got: %+v\nwant: %+v", got, feed)
	}
}
