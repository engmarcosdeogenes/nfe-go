package builder_test

import (
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

func TestTpNF(t *testing.T) {
	// Entrada de remetente pessoa física na mesma UF (GO), CFOP configurável.
	base := func(tpNF, cfop string) builder.EntradaNFe {
		e := entradaExemplo()
		e.TpNF = tpNF
		e.Dest.CNPJ, e.Dest.CPF, e.Dest.IndIEDest, e.Dest.IE = "", "12345678901", "9", ""
		for i := range e.Itens {
			e.Itens[i].CFOP = cfop
		}
		return e
	}
	outraUF := func(e builder.EntradaNFe) builder.EntradaNFe { e.Dest.End.UF = "SP"; return e }
	nfce0 := entradaNFCe()
	nfce0.TpNF = "0"

	casos := []struct {
		nome    string
		e       builder.EntradaNFe
		contem  []string
		erroCom string
	}{
		{"entrada 1102 mesma UF", base("0", "1102"),
			[]string{"<tpNF>0</tpNF>", "<idDest>1</idDest>", "<indFinal>1</indFinal>", "<indIEDest>9</indIEDest>"}, ""},
		{"vazio vira saída", base("", "5102"), []string{"<tpNF>1</tpNF>"}, ""},
		{"0 com 5102", base("0", "5102"), nil, "519"},
		{"1 com 1102", base("1", "1102"), nil, "518"},
		{"0 com 2102 mesma UF", base("0", "2102"), nil, "732"},
		{"0 com 1102 outra UF", outraUF(base("0", "1102")), nil, "733"},
		{"65 com tpNF 0", nfce0, nil, "706"},
		{"0 com 3102", base("0", "3102"), nil, "525"},
		{"valor inválido", base("2", "5102"), nil, "TpNF"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			xmlBytes, _, err := builder.Build(c.e)
			if c.erroCom != "" {
				if err == nil || !strings.Contains(err.Error(), c.erroCom) {
					t.Fatalf("esperava erro com %q, veio: %v", c.erroCom, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			for _, s := range c.contem {
				if !strings.Contains(string(xmlBytes), s) {
					t.Errorf("XML sem %s", s)
				}
			}
		})
	}
}
