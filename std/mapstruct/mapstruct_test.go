package mapstruct

import (
	"reflect"
	"strings"
	"testing"
)

func TestUserDecodeAndEncode(t *testing.T) {
	var user User
	input := map[string]any{
		"id":           "42",
		"display_name": "Ada",
		"address": map[string]any{
			"city":    "Barcelona",
			"country": "ES",
		},
	}
	if err := user.Decode(input); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := User{ID: "42", Name: "Ada", Address: Address{City: "Barcelona", Country: "ES"}}
	if user != want {
		t.Errorf("Decode() = %+v, want %+v", user, want)
	}
	if got := user.Encode(); !reflect.DeepEqual(got, input) {
		t.Errorf("Encode() = %#v, want %#v", got, input)
	}
}

func TestUserDecodeRequiresAllFields(t *testing.T) {
	user := User{ID: "original", Name: "Original"}
	err := user.Decode(map[string]any{"id": "changed"})
	if err == nil || !strings.Contains(err.Error(), `field "display_name" is required`) {
		t.Fatalf("Decode() error = %v, want missing display_name", err)
	}
	if user.ID != "original" {
		t.Errorf("Decode() partially modified receiver: ID = %q", user.ID)
	}
}

func TestUserDecodeReportsNestedTypeError(t *testing.T) {
	var user User
	err := user.Decode(map[string]any{
		"id":           "42",
		"display_name": "Ada",
		"address": map[string]any{
			"city":    123,
			"country": "ES",
		},
	})
	if err == nil || !strings.Contains(err.Error(), `field "city" must be string`) {
		t.Fatalf("Decode() error = %v, want city type error", err)
	}
}

func TestAllowMissingWithRequiredOverride(t *testing.T) {
	preferences := Preferences{Theme: "dark", Locale: "en"}
	if err := preferences.Decode(map[string]any{"locale": "ca"}); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if preferences.Theme != "dark" || preferences.Locale != "ca" {
		t.Errorf("Decode() = %+v, want partial update", preferences)
	}

	err := preferences.Decode(map[string]any{"theme": "light"})
	if err == nil || !strings.Contains(err.Error(), `field "locale" is required`) {
		t.Fatalf("Decode() error = %v, want required locale", err)
	}
	if preferences.Theme != "dark" {
		t.Errorf("failed Decode() partially modified receiver: Theme = %q", preferences.Theme)
	}
}
