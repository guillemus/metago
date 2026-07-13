package jsonruntime

import (
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func TestFloatMatchesStrconvAcrossFastAndFallbackForms(t *testing.T) {
	values := []string{
		"0", "-0", "1", "-1", "1.5", "-1.5", "1.25", "19.5", "0.125",
		"9.99", "0.1", "1e10", "1e-10", "4503599627370496", "4503599627370497",
		"1.17549435e-38", "3.4028235e+38", "5e-324", "1.7976931348623157e+308",
	}
	for _, raw := range values {
		t.Run(raw, func(t *testing.T) {
			for _, bits := range []int{32, 64} {
				want, wantErr := strconv.ParseFloat(raw, bits)
				lexer := Lexer{Data: []byte(raw)}
				got := lexer.Float(bits)
				if (lexer.Err != nil) != (wantErr != nil) {
					t.Fatalf("bits=%d: got error %v, want error %v", bits, lexer.Err, wantErr)
				}
				if wantErr != nil {
					continue
				}
				if math.Float64bits(got) != math.Float64bits(want) {
					t.Fatalf("bits=%d: got %.17g, want %.17g", bits, got, want)
				}
			}
		})
	}
}

func TestFloatMatchesStrconvForRandomValues(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 10000 {
		value := math.Float64frombits(rng.Uint64())
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		raw := strconv.FormatFloat(value, 'g', -1, 64)
		want, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatal(err)
		}
		lexer := Lexer{Data: []byte(raw)}
		got := lexer.Float64()
		if lexer.Err != nil {
			t.Fatalf("%q: %v", raw, lexer.Err)
		}
		if math.Float64bits(got) != math.Float64bits(want) || lexer.Pos != len(raw) {
			t.Fatalf("%q: got %x at %d, want %x at %d", raw, math.Float64bits(got), lexer.Pos, math.Float64bits(want), len(raw))
		}
	}
}

func TestFloatMatchesStrconvForRandomDecimalSyntax(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for range 20000 {
		raw := randomJSONNumber(rng)
		for _, bits := range []int{32, 64} {
			want, wantErr := strconv.ParseFloat(raw, bits)
			lexer := Lexer{Data: []byte(raw)}
			got := lexer.Float(bits)
			if (lexer.Err != nil) != (wantErr != nil) {
				t.Fatalf("%q bits=%d: got error %v, want %v", raw, bits, lexer.Err, wantErr)
			}
			if lexer.Pos != len(raw) {
				t.Fatalf("%q bits=%d: stopped at %d, want %d", raw, bits, lexer.Pos, len(raw))
			}
			if wantErr == nil && math.Float64bits(got) != math.Float64bits(want) {
				t.Fatalf("%q bits=%d: got %x, want %x", raw, bits, math.Float64bits(got), math.Float64bits(want))
			}
		}
	}
}

func randomJSONNumber(rng *rand.Rand) string {
	raw := make([]byte, 0, 64)
	if rng.Intn(2) == 0 {
		raw = append(raw, '-')
	}
	if rng.Intn(5) == 0 {
		raw = append(raw, '0')
	} else {
		raw = append(raw, byte('1'+rng.Intn(9)))
		for range rng.Intn(24) {
			raw = append(raw, byte('0'+rng.Intn(10)))
		}
	}
	if rng.Intn(4) != 0 {
		raw = append(raw, '.')
		for range 1 + rng.Intn(30) {
			raw = append(raw, byte('0'+rng.Intn(10)))
		}
	}
	if rng.Intn(2) == 0 {
		raw = append(raw, 'e')
		raw = strconv.AppendInt(raw, int64(rng.Intn(801)-400), 10)
	}
	return string(raw)
}
