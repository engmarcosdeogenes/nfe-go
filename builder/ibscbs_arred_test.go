package builder

import "testing"

// Empates vão ao par (ABNT NBR 5891, Calculadora RTC v1.7.2).
func TestDivHalfEven(t *testing.T) {
	// n/1000 com n em milésimos de centavo... aqui: reais com 3 casas -> centavos.
	for n, want := range map[uint64]int64{525: 52, 535: 54, 575: 58, 515: 52, 5: 0, 15: 2, 526: 53, 524: 52} {
		if got := divHalfEven(n, 1, 10); got != want {
			t.Errorf("%d/10 = %d, quer %d", n, got, want)
		}
	}
	// 1150,00 x 0,05% = 0,575 -> 0,58 (float dava 0,57)
	if got := divHalfEven(115000, 500, 1_000_000); got != 58 {
		t.Errorf("vBC 115000 x 0,05%% = %d, quer 58", got)
	}
}

func TestFmtAliq(t *testing.T) {
	for v, want := range map[int64]string{1000: "0.10", 350: "0.035", 88000: "8.80", 667: "0.0667", 0: "0.00", 600000: "60.00", 61600: "6.16"} {
		if got := fmtAliq(v); got != want {
			t.Errorf("fmtAliq(%d)=%q, quer %q", v, got, want)
		}
	}
}

// Alíquota negativa ou redução > 100% estouraria o uint64 do divHalfEven.
func TestIBSCBS_AliquotaInvalidaRecusa(t *testing.T) {
	for _, ib := range []EntradaIBSCBS{{AliqCBS: -0.9}, {AliqCBS: 0.9, PercentualReducao: 101}} {
		ib := ib
		item := EntradaItem{Quantidade: 1, VUnitario: 10, ICMS: EntradaICMS{CST: "00", Aliq: 12}, IBSCBS: &ib}
		if _, _, err := montarImposto(item, "3", 0, 0, 0); err == nil {
			t.Errorf("%+v: esperava erro", ib)
		}
	}
}
