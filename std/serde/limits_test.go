package serde

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestConfiguredInputLimit(t *testing.T) {
	accepted := []byte(`{"name":"12345678901234567890"}`)
	if len(accepted) > 32 {
		t.Fatalf("test setup input length = %d, want at most 32", len(accepted))
	}
	var value LimitedInput
	if err := value.UnmarshalJSON(accepted); err != nil {
		t.Fatalf("input within configured limit rejected: %v", err)
	}

	before := LimitedInput{Name: "unchanged"}
	value = before
	oversized := []byte(`{"name":"` + strings.Repeat("x", 32) + `"}`)
	if err := value.UnmarshalJSON(oversized); err == nil || !strings.Contains(err.Error(), "input size") {
		t.Fatalf("oversized input error = %v, want input-size error", err)
	}
	if value != before {
		t.Fatalf("oversized input changed receiver: got %#v, want %#v", value, before)
	}
}

func TestConfiguredDepthLimitCoversGeneratedRecursion(t *testing.T) {
	var accepted LimitedDepth
	if err := accepted.UnmarshalJSON([]byte(nestedLimitedDepthJSON(4))); err != nil {
		t.Fatalf("value at configured depth rejected: %v", err)
	}

	before := LimitedDepth{Name: "unchanged", Next: &LimitedDepth{Name: "nested"}}
	value := before
	if err := value.UnmarshalJSON([]byte(nestedLimitedDepthJSON(5))); err == nil || !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("value beyond configured depth error = %v, want depth error", err)
	}
	if !reflect.DeepEqual(value, before) {
		t.Fatalf("depth failure changed receiver: got %#v, want %#v", value, before)
	}
}

func TestConfiguredDepthLimitCoversSkippedValues(t *testing.T) {
	accepted := `{"unknown":` + strings.Repeat("[", 3) + `null` + strings.Repeat("]", 3) + `}`
	var value LimitedDepth
	if err := value.UnmarshalJSON([]byte(accepted)); err != nil {
		t.Fatalf("skipped value at configured depth rejected: %v", err)
	}

	rejected := `{"unknown":` + strings.Repeat("[", 4) + `null` + strings.Repeat("]", 4) + `}`
	if err := value.UnmarshalJSON([]byte(rejected)); err == nil || !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Fatalf("skipped value beyond configured depth error = %v, want depth error", err)
	}
}

func TestConfiguredLimitsAppearInGeneratedSource(t *testing.T) {
	source, err := os.ReadFile("meta.go")
	if err != nil {
		t.Fatal(err)
	}
	generated := string(source)
	for _, want := range []string{
		"if maxInput := uint64(32)",
		"serdejsonruntime.Lexer{Data: data, MaxDepth: uint64(4)}",
		"if !l.EnterValue()",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated resource-limit path missing %q", want)
		}
	}
}

func nestedLimitedDepthJSON(depth int) string {
	if depth < 1 {
		return "null"
	}
	return strings.Repeat(`{"next":`, depth-1) + `{}` + strings.Repeat(`}`, depth-1)
}
