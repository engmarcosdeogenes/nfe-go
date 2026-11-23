package danfe

import "testing"

// O corte é 01/12/2026 00:00 em -03:00, comparado no instante: o fuso do
// XML (UF em -04/-05) não pode puxar nota para o lado errado.
func TestUsaLeiauteRTC(t *testing.T) {
	for dhEmi, quer := range map[string]bool{
		"2026-11-30T23:59:59-03:00": false,
		"2026-12-01T00:00:00-03:00": true,
		"2026-11-30T23:00:00-04:00": true, // 00:00 em Brasília
		"2026-11-30T21:59:59-05:00": false,
		"2026-06-25T10:00:00-03:00": false,
		"2027-01-10T08:00:00-03:00": true,
		"lixo":                      true, // leiaute vigente
	} {
		if got := usaLeiauteRTC(dhEmi); got != quer {
			t.Errorf("usaLeiauteRTC(%q) = %v, quer %v", dhEmi, got, quer)
		}
	}
}

func TestFormatarPct(t *testing.T) {
	for v, quer := range map[float64]string{0: "0,00", 0.9: "0,90", 0.036: "0,036", 12: "12,00", 0.0333: "0,0333", 0.12345: "0,1235"} {
		if got := formatarPct(v); got != quer {
			t.Errorf("formatarPct(%v) = %q, quer %q", v, got, quer)
		}
	}
}
