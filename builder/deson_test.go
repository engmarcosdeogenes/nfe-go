package builder_test

import (
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// CST 40 com cBenef em GO exige vICMSDeson/motDesICMS (RCTE art. 167-C §13); indDeduzDeson=0 mantém o vNF.
func TestICMS40Desonerado(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{e.Itens[0]}
	e.Itens[0].Quantidade, e.Itens[0].VUnitario, e.Itens[0].VDesconto = 1, 100, 0
	e.Itens[0].ICMS = builder.EntradaICMS{CST: "40", VICMSDeson: 19, MotDesICMS: "9"}
	x, _, err := builder.Build(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(x)
	for _, w := range []string{
		"<CST>40</CST><vICMSDeson>19.00</vICMSDeson><motDesICMS>9</motDesICMS><indDeduzDeson>0</indDeduzDeson>",
		"<vICMS>0.00</vICMS><vICMSDeson>19.00</vICMSDeson>", // ICMSTot
		"<vNF>100.00</vNF>",
	} {
		if !strings.Contains(s, w) {
			t.Errorf("XML sem %s", w)
		}
	}
	e.Itens[0].ICMS.MotDesICMS = ""
	if _, _, err := builder.Build(e); err == nil {
		t.Error("vICMSDeson sem motDesICMS devia falhar")
	}
}
