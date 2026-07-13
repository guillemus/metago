package serde

import (
	"slices"
	"testing"
)

func TestDecodedStringsDoNotAliasInput(t *testing.T) {
	input := []byte(`{"name":"line\nquote\"slash\\","email":"日本語","tags":["escaped\tvalue","直接"],"address":{"city":"\u65e5\u672c"},"items":[{"sku":"sku\u002dvalue"}],"metadata":{"key":"value\nnext"}}`)
	var value User
	if err := value.UnmarshalJSON(input); err != nil {
		t.Fatal(err)
	}

	for i := range input {
		input[i] = 'x'
	}

	if value.Name != "line\nquote\"slash\\" {
		t.Fatalf("decoded field changed after input reuse: %q", value.Name)
	}
	if value.Email != "日本語" {
		t.Fatalf("decoded Unicode field changed after input reuse: %q", value.Email)
	}
	if got := value.Metadata["key"]; got != "value\nnext" {
		t.Fatalf("decoded map key/value changed after input reuse: %q", got)
	}
	if got := value.Tags; !slices.Equal(got, []string{"escaped\tvalue", "直接"}) {
		t.Fatalf("decoded slice string changed after input reuse: %q", got)
	}
	if got := value.Address.City; got != "日本" {
		t.Fatalf("decoded escaped Unicode field changed after input reuse: %q", got)
	}
	if got := value.Items[0].SKU; got != "sku-value" {
		t.Fatalf("decoded nested string changed after input reuse: %q", got)
	}
}
