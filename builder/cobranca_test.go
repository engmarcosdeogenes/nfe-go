package builder_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// entradaComCobranca: NF-e de 100,00 em 3 parcelas, boleto a prazo.
func entradaComCobranca() builder.EntradaNFe {
	e := entradaExemplo()
	venc := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, e.DhEmi.Location()) }
	e.Pagamento = []builder.EntradaPagamento{{Forma: "15", Valor: 100, APrazo: true}}
	e.Cobranca = &builder.EntradaCobranca{
		NFat: "42", VOrig: 100,
		Duplicatas: []builder.EntradaDuplicata{
			{DVenc: venc(7, 25), VDup: 33.33},
			{DVenc: venc(8, 24), VDup: 33.33},
			{DVenc: venc(9, 23), VDup: 33.34},
		},
	}
	return e
}

func TestCobranca_XML(t *testing.T) {
	xmlBytes, _, err := builder.Build(entradaComCobranca())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(xmlBytes)
	ini, fim := strings.Index(s, "<cobr>"), strings.Index(s, "</cobr>")
	if ini < 0 || fim < 0 {
		t.Fatalf("XML sem <cobr>: %s", s)
	}
	got := s[ini : fim+len("</cobr>")]
	want, err := os.ReadFile("testdata/golden/cobr_3_parcelas.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Errorf("cobr difere do golden\n got: %s\nwant: %s", got, want)
	}
	if !(strings.Index(s, "</transp>") < ini && fim < strings.Index(s, "<pag>")) {
		t.Error("<cobr> tem que ficar depois de </transp> e antes de <pag>")
	}
	if !strings.Contains(s, "<detPag><indPag>1</indPag><tPag>15</tPag>") {
		t.Error("esperava detPag a prazo (indPag=1) com tPag=15")
	}
}

func TestCobranca_Nil_SemCobr(t *testing.T) {
	xmlBytes, _, err := builder.Build(entradaExemplo())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(xmlBytes), "<cobr>") {
		t.Error("Cobranca nil não deve gerar <cobr>")
	}
}

func TestCobranca_Rejeicoes(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	casos := []struct {
		nome  string
		mexe  func(e *builder.EntradaNFe)
		cStat string // vazio = tem que passar; "pag" = erro de coerência com pagamento
	}{
		{"valido: vencimento no dia da emissão", func(e *builder.EntradaNFe) {
			e.Cobranca.Duplicatas = []builder.EntradaDuplicata{{DVenc: e.DhEmi, VDup: 100}}
		}, ""},
		{"valido: NDup explícito e vLiq com desconto", func(e *builder.EntradaNFe) {
			e.Cobranca.VDesc, e.Cobranca.VLiq = 10, 90
			e.Cobranca.Duplicatas = []builder.EntradaDuplicata{{NDup: "001", DVenc: e.DhEmi, VDup: 90}}
		}, ""},
		{"760 NFC-e", func(e *builder.EntradaNFe) {
			n := entradaNFCe()
			n.Cobranca = e.Cobranca
			*e = n
		}, "760"},
		{"901 desconto maior", func(e *builder.EntradaNFe) { e.Cobranca.VDesc = 150 }, "901"},
		{"902 vLiq diferente", func(e *builder.EntradaNFe) { e.Cobranca.VDesc, e.Cobranca.VLiq = 10, 95 }, "902"},
		{"905 líquido zero", func(e *builder.EntradaNFe) { e.Cobranca.VOrig = 0 }, "905"},
		{"852 nDup fora de sequência", func(e *builder.EntradaNFe) { e.Cobranca.Duplicatas[1].NDup = "007" }, "852"},
		{"900 vencimento ontem", func(e *builder.EntradaNFe) {
			e.Cobranca.Duplicatas[0].DVenc = time.Date(2026, 6, 24, 0, 0, 0, 0, loc)
		}, "900"},
		{"900 vencimento zero", func(e *builder.EntradaNFe) { e.Cobranca.Duplicatas[0].DVenc = time.Time{} }, "900"},
		{"850 vencimento antes da anterior", func(e *builder.EntradaNFe) {
			e.Cobranca.Duplicatas[2].DVenc = time.Date(2026, 8, 1, 0, 0, 0, 0, loc)
		}, "850"},
		{"851 soma diferente", func(e *builder.EntradaNFe) { e.Cobranca.Duplicatas[2].VDup = 40 }, "851"},
		{"sem pagamento a prazo", func(e *builder.EntradaNFe) { e.Pagamento[0].APrazo = false }, "a prazo"},
		{"pagamento a prazo com forma 01", func(e *builder.EntradaNFe) { e.Pagamento[0].Forma = "01" }, "a prazo"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := entradaComCobranca()
			c.mexe(&e)
			_, _, err := builder.Build(e)
			if c.cStat == "" {
				if err != nil {
					t.Fatalf("esperava passar, veio: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.cStat) {
				t.Fatalf("esperava erro com %q, veio: %v", c.cStat, err)
			}
		})
	}
}
