package enum

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type enumValue interface {
	comparable
	fmt.Stringer
	Valid() bool
}

func testEnumAPI[T enumValue](
	t *testing.T,
	values []T,
	labels []string,
	parse func(string) (T, error),
	generatedValues func() []T,
	invalid T,
) {
	t.Helper()
	if len(values) != len(labels) {
		t.Fatalf("test setup has %d values and %d labels", len(values), len(labels))
	}

	if got := generatedValues(); !reflect.DeepEqual(got, values) {
		t.Errorf("Values() = %#v, want %#v", got, values)
	}
	if len(values) > 0 {
		got := generatedValues()
		got[0] = invalid
		if next := generatedValues(); reflect.DeepEqual(next, got) {
			t.Error("Values() reused mutable backing storage")
		}
	}

	for i, value := range values {
		label := labels[i]
		if !value.Valid() {
			t.Errorf("%v.Valid() = false, want true", value)
		}
		if got := value.String(); got != label {
			t.Errorf("String() = %q, want %q", got, label)
		}

		parsed, err := parse(label)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", label, err)
		} else if parsed != value {
			t.Errorf("Parse(%q) = %v, want %v", label, parsed, value)
		}

		encoded, err := json.Marshal(value)
		if err != nil {
			t.Errorf("Marshal(%v) error = %v", value, err)
		} else if want := fmt.Sprintf("%q", label); string(encoded) != want {
			t.Errorf("Marshal(%v) = %s, want %s", value, encoded, want)
		}

		var decoded T
		if err := json.Unmarshal([]byte(fmt.Sprintf("%q", label)), &decoded); err != nil {
			t.Errorf("Unmarshal(%q) error = %v", label, err)
		} else if decoded != value {
			t.Errorf("Unmarshal(%q) = %v, want %v", label, decoded, value)
		}
	}

	if invalid.Valid() {
		t.Errorf("invalid value %v.Valid() = true, want false", invalid)
	}
	if _, err := parse("not-a-member"); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Parse(invalid) error = %v, want invalid-value error", err)
	}
	if _, err := json.Marshal(invalid); err == nil || !strings.Contains(err.Error(), "cannot marshal invalid") {
		t.Errorf("Marshal(invalid) error = %v, want invalid-value error", err)
	}

	original := values[0]
	decoded := original
	if err := json.Unmarshal([]byte(`"not-a-member"`), &decoded); err == nil {
		t.Error("Unmarshal(invalid member) succeeded")
	}
	if decoded != original {
		t.Errorf("failed Unmarshal changed value to %v, want %v", decoded, original)
	}
	if err := json.Unmarshal([]byte(`123`), &decoded); err == nil {
		t.Error("Unmarshal(non-string JSON) succeeded")
	}
	if decoded != original {
		t.Errorf("failed non-string Unmarshal changed value to %v, want %v", decoded, original)
	}
}

func TestArticleStatusEnum(t *testing.T) {
	testEnumAPI(
		t,
		[]ArticleStatus{ArticleStatusDraft, ArticleStatusPublished, ArticleStatusArchived},
		[]string{"draft", "published", "archived"},
		ParseArticleStatus,
		ArticleStatusValues,
		ArticleStatus("deleted"),
	)
}

func TestSignedIntegerEnum(t *testing.T) {
	testEnumAPI(
		t,
		[]Priority{PriorityLow, PriorityNormal, PriorityHigh},
		[]string{"Low", "Normal", "High"},
		ParsePriority,
		PriorityValues,
		Priority(99),
	)
}

func TestUnsignedIntegerEnum(t *testing.T) {
	testEnumAPI(
		t,
		[]Permission{PermissionRead, PermissionWrite, PermissionAdmin},
		[]string{"Read", "Write", "Admin"},
		ParsePermission,
		PermissionValues,
		Permission(4),
	)
}

func TestFloatEnums(t *testing.T) {
	t.Run("float32", func(t *testing.T) {
		testEnumAPI(
			t,
			[]Ratio{RatioHalf, RatioFull},
			[]string{"Half", "Full"},
			ParseRatio,
			RatioValues,
			Ratio(0.75),
		)
	})
	t.Run("float64", func(t *testing.T) {
		testEnumAPI(
			t,
			[]Scale{ScaleSmall, ScaleLarge},
			[]string{"Small", "Large"},
			ParseScale,
			ScaleValues,
			Scale(2.5),
		)
	})
}
