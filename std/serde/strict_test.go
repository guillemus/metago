package serde

import (
	"strings"
	"testing"
)

func TestStrictCodecRejectsUnknownFields(t *testing.T) {
	value := StrictValues{Name: "before"}
	err := value.UnmarshalJSON([]byte(`{"name":"changed","unknown":{"nested":[1,true,null]}}`))
	if err == nil {
		t.Fatal("strict codec accepted unknown field")
	}
	for _, want := range []string{"StrictValues", "unknown field", "unknown", "offset"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("strict error %q does not contain %q", err, want)
		}
	}
	if value.Name != "before" {
		t.Fatalf("strict failure changed receiver: %#v", value)
	}
}

func TestStrictCodecAcceptsKnownFields(t *testing.T) {
	var value StrictValues
	if err := value.UnmarshalJSON([]byte(`{"name":"known"}`)); err != nil {
		t.Fatal(err)
	}
	if value.Name != "known" {
		t.Fatalf("strict known field = %q, want known", value.Name)
	}
}

func TestDefaultCodecStillIgnoresUnknownFields(t *testing.T) {
	var value CompatibilityValues
	if err := value.UnmarshalJSON([]byte(`{"unknown":{"nested":[1,true,null]},"string":"known"}`)); err != nil {
		t.Fatal(err)
	}
	if value.String != "known" {
		t.Fatalf("default codec did not continue after unknown field: %#v", value)
	}
}
