package danfe

import "testing"

// Limites do manual v6.0: papel >= 56 mm (3.3), margens laterais >= 2 mm
// (3.3), QR Code >= 25 x 25 mm (3.4).
func TestCupom_LimitesDoManual(t *testing.T) {
	if cupomLarg < 56 {
		t.Errorf("largura do papel %v mm < 56", cupomLarg)
	}
	if cupomMarg < 2 {
		t.Errorf("margem lateral %v mm < 2", cupomMarg)
	}
	if cupomQR < 25 {
		t.Errorf("QR Code %v mm < 25", cupomQR)
	}
}

func TestFormatarDec(t *testing.T) {
	tests := []struct {
		v        float64
		min, max int
		want     string
	}{
		{6, 0, 4, "6"},
		{0.482, 0, 4, "0,482"},
		{5.4, 2, 4, "5,40"},
		{5.49, 2, 4, "5,49"},
		{1234.5, 2, 4, "1.234,50"},
		{12.34567, 2, 4, "12,3457"},
		{0, 2, 4, "0,00"},
		{-3, 2, 4, "-3,00"},
	}
	for _, tc := range tests {
		if got := formatarDec(tc.v, tc.min, tc.max); got != tc.want {
			t.Errorf("formatarDec(%v,%d,%d) = %q, esperava %q", tc.v, tc.min, tc.max, got, tc.want)
		}
	}
}
