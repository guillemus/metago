package serde

import "testing"

func TestDecodedStringsDoNotAliasInput(t *testing.T) {
	input := []byte(`{"name":"retained","metadata":{"key":"value"}}`)
	var value User
	if err := value.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}

	for i := range input {
		input[i] = 'x'
	}

	if value.Name != "retained" {
		t.Fatalf("decoded field changed after input reuse: %q", value.Name)
	}
	if got := value.Metadata["key"]; got != "value" {
		t.Fatalf("decoded map key/value changed after input reuse: %q", got)
	}
}
