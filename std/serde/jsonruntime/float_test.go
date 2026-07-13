package jsonruntime

import (
	"math"
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
