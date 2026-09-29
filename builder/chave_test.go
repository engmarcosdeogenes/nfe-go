package builder

import (
	"testing"
	"time"
)

// Exemplo oficial SERPRO: 12.ABC.345/01DE-35. O DV do CNPJ usa o mesmo
// módulo 11 (ASCII − 48, pesos 2..9) de calcularDV.
func TestCalcularDV_CNPJAlfanumericoSERPRO(t *testing.T) {
	if dv := calcularDV("12ABC34501DE"); dv != "3" {
		t.Errorf("1º DV = %s, esperava 3", dv)
	}
	if dv := calcularDV("12ABC34501DE3"); dv != "5" {
		t.Errorf("2º DV = %s, esperava 5", dv)
	}
}

func TestNovaChave_CNPJAlfanumerico(t *testing.T) {
	cnpj := FormatarCNPJ("12.abc.345/01de-35")
	if cnpj != "12ABC34501DE35" {
		t.Fatalf("FormatarCNPJ = %q", cnpj)
	}
	c := NovaChave("SP", cnpj, "1", "42", "1", "55", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	c.CNF = "12345678"
	base := c.CUF + c.AAMM + c.CNPJ + c.Mod + c.Serie + c.NNF + c.TpEmis + c.CNF
	if base != "35260912ABC34501DE3555001000000042112345678" {
		t.Fatalf("base = %s", base)
	}
	// Esperado calculado à parte (A=17, B=18, ...): se letra virasse 0 daria outro DV.
	if dv := calcularDV(base); dv != "4" {
		t.Errorf("cDV = %s, esperava 4", dv)
	}
}

func TestCalcularDV_ChaveNumericaInalterada(t *testing.T) {
	if dv := calcularDV("3526092066656000019755001000000042112345678"); dv != "0" {
		t.Errorf("cDV = %s, esperava 0", dv)
	}
	if got := FormatarCNPJ("20.666.560/0001-97"); got != "20666560000197" {
		t.Errorf("FormatarCNPJ numérico = %q", got)
	}
}

func TestFormatarCPF_SoDigitos(t *testing.T) {
	if got := FormatarCPF("123.456.789-0a"); got != "1234567890" {
		t.Errorf("FormatarCPF = %q", got)
	}
	if got := FormatarCPF("123.456.789-00"); got != "12345678900" {
		t.Errorf("FormatarCPF = %q", got)
	}
}
