package stringer

import "testing"

func TestPriorityString(t *testing.T) {
	for value, want := range map[Priority]string{
		PriorityLow:    "Low",
		PriorityNormal: "Normal",
		PriorityHigh:   "High",
		Priority(99):   "Priority(99)",
	} {
		if got := value.String(); got != want {
			t.Errorf("Priority(%d).String() = %q, want %q", value, got, want)
		}
	}
}

func TestIntegerStringers(t *testing.T) {
	for value, want := range map[Direction]string{
		DirectionUnknown: "Unknown",
		DirectionNorth:   "North",
		DirectionSouth:   "South",
		Direction(7):     "Direction(7)",
	} {
		if got := value.String(); got != want {
			t.Errorf("Direction(%d).String() = %q, want %q", value, got, want)
		}
	}

	for value, want := range map[Permission]string{
		PermissionRead:  "Read",
		PermissionWrite: "Write",
		PermissionAdmin: "Admin",
		Permission(8):   "Permission(8)",
	} {
		if got := value.String(); got != want {
			t.Errorf("Permission(%d).String() = %q, want %q", value, got, want)
		}
	}
}

func TestValueTypeStringers(t *testing.T) {
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"string", Label("hello").String(), "hello"},
		{"int", Count(-42).String(), "-42"},
		{"uint", UCount(42).String(), "42"},
		{"bool", Flag(true).String(), "true"},
		{"float", Score(1.25).String(), "1.25"},
		{"complex", Point(2 + 3i).String(), "(2+3i)"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s: got %q, want %q", check.name, check.got, check.want)
		}
	}
}

func TestOtherPrimitiveStringers(t *testing.T) {
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"known string", StateReady.String(), "Ready"},
		{"unknown string", State("paused").String(), `State("paused")`},
		{"known bool", EnabledYes.String(), "Yes"},
		{"known float", RatioHalf.String(), "Half"},
		{"unknown float", Ratio(1.25).String(), "Ratio(1.25)"},
		{"known complex", PhaseImaginary.String(), "Imaginary"},
		{"unknown complex", Phase(2 + 3i).String(), "Phase((2+3i))"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s: got %q, want %q", check.name, check.got, check.want)
		}
	}
}
