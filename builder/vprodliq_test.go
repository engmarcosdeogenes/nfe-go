package builder_test

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// NT 2026.008: I11a (vUnComLiq/vProdLiq) logo após vProd e W01a (vProdLiqTot)
// como primeiro filho de total. Regras 1278/1279/1282/1283 e ausência do NB01.
func TestVProdLiq_NT2026008(t *testing.T) {
	itens := []builder.EntradaItem{
		{CProd: "A", CEAN: "SEM GTIN", Nome: "A", NCM: "73089090", CFOP: "5102", Unidade: "UN", Quantidade: 3, VUnitario: 2.333, ICMS: builder.EntradaICMS{CSOSN: "400"}},
		{CProd: "B", CEAN: "SEM GTIN", Nome: "B", NCM: "73089090", CFOP: "5102", Unidade: "KG", Quantidade: 1.5, VUnitario: 10.10, ICMS: builder.EntradaICMS{CSOSN: "400"}},
		{CProd: "C", CEAN: "SEM GTIN", Nome: "C", NCM: "73089090", CFOP: "5102", Unidade: "UN", Quantidade: 7, VUnitario: 0.99, ICMS: builder.EntradaICMS{CSOSN: "400"}},
	}
	nfe := entradaExemplo()
	nfe.Itens = itens
	nfe.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}
	nfce := entradaNFCe()
	nfce.Itens = itens
	nfce.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}

	for nome, e := range map[string]builder.EntradaNFe{"nfe55": nfe, "nfce65": nfce} {
		t.Run(nome, func(t *testing.T) {
			xmlBytes, _, err := builder.Build(e)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			s := string(xmlBytes)
			var doc struct {
				Det []struct {
					Prod struct {
						QCom      string `xml:"qCom"`
						VProd     string `xml:"vProd"`
						VUnComLiq string `xml:"vUnComLiq"`
						VProdLiq  string `xml:"vProdLiq"`
					} `xml:"prod"`
				} `xml:"infNFe>det"`
				VProdLiqTot string `xml:"infNFe>total>vProdLiqTot"`
				VProdTot    string `xml:"infNFe>total>ICMSTot>vProd"`
			}
			if err := xml.Unmarshal(xmlBytes, &doc); err != nil {
				t.Fatal(err)
			}
			num := func(v string) float64 { f, _ := strconv.ParseFloat(v, 64); return f }
			soma := 0.0
			for i, d := range doc.Det {
				p := d.Prod
				if p.VUnComLiq == "" || p.VProdLiq == "" { // 1278
					t.Fatalf("item %d sem vUnComLiq/vProdLiq", i+1)
				}
				if diff := math.Abs(num(p.VProdLiq) - num(p.VUnComLiq)*num(p.QCom)); diff > 0.01+1e-9 { // 1279
					t.Errorf("item %d: vProdLiq %s vs vUnComLiq %s × qCom %s (diff %.4f)", i+1, p.VProdLiq, p.VUnComLiq, p.QCom, diff)
				}
				if p.VProdLiq != p.VProd {
					t.Errorf("item %d: vProdLiq %s != vProd %s (até 2026 sem IBS/CBS no vProd)", i+1, p.VProdLiq, p.VProd)
				}
				soma += num(p.VProdLiq)
			}
			if doc.VProdLiqTot == "" { // 1282
				t.Fatal("vProdLiqTot ausente")
			}
			if doc.VProdLiqTot != strconv.FormatFloat(soma, 'f', 2, 64) { // 1283
				t.Errorf("vProdLiqTot %s != soma dos itens %.2f", doc.VProdLiqTot, soma)
			}
			if doc.VProdLiqTot != doc.VProdTot {
				t.Errorf("vProdLiqTot %s != ICMSTot/vProd %s", doc.VProdLiqTot, doc.VProdTot)
			}
			ordem := func(tags ...string) {
				pos := -1
				for _, tag := range tags {
					i := strings.Index(s, tag)
					if i <= pos {
						t.Errorf("ordem errada em %v: %s", tags, tag)
					}
					pos = i
				}
			}
			ordem("<vProd>", "<vUnComLiq>", "<vProdLiq>", "<cEANTrib>")
			ordem("<total>", "<vProdLiqTot>", "<ICMSTot>")
			if strings.Contains(s, "ICMSPrevistoPagtoAntecip") { // 1280
				t.Error("grupo NB01 não deve ser gerado")
			}
			validarXSD(t, xmlBytes)
		})
	}
}

// Trava da NT 2026.008: homologação sempre; produção só a partir de
// 03/11/2026 00:00 -03:00; ValorLiquido "sim"/"nao" sobrepõe. Itens e total
// juntos. Sem os campos, valida contra o PL_010 v1.30 original.
func TestVProdLiq_Trava(t *testing.T) {
	brt := time.FixedZone("BRT", -3*3600)
	casos := []struct {
		nome, tpAmb, override string
		dhEmi                 time.Time
		gera                  bool
	}{
		{"homolog_hoje", "2", "", time.Date(2026, 10, 27, 10, 0, 0, 0, brt), true},
		{"producao_antes", "1", "", time.Date(2026, 11, 2, 23, 59, 0, 0, brt), false},
		{"producao_inicio", "1", "", time.Date(2026, 11, 3, 0, 0, 0, 0, brt), true},
		{"homolog_override_nao", "2", "nao", time.Date(2026, 10, 27, 10, 0, 0, 0, brt), false},
		{"producao_override_sim", "1", "sim", time.Date(2026, 11, 2, 23, 59, 0, 0, brt), true},
	}
	original := xsdOriginal(t)
	for _, c := range casos {
		for nome, base := range map[string]builder.EntradaNFe{"nfe55": entradaExemplo(), "nfce65": entradaNFCe()} {
			t.Run(c.nome+"/"+nome, func(t *testing.T) {
				e := base
				e.TpAmb, e.DhEmi, e.ValorLiquido = c.tpAmb, c.dhEmi, c.override
				xmlBytes, _, err := builder.Build(e)
				if err != nil {
					t.Fatalf("Build: %v", err)
				}
				s := string(xmlBytes)
				itens := strings.Count(s, "<det ")
				for _, tag := range []string{"<vUnComLiq>", "<vProdLiq>"} {
					if n := strings.Count(s, tag); (c.gera && n != itens) || (!c.gera && n != 0) {
						t.Errorf("%s: %d ocorrências em %d itens, gera=%v", tag, n, itens, c.gera)
					}
				}
				if strings.Contains(s, "<vProdLiqTot>") != c.gera {
					t.Errorf("vProdLiqTot presente=%v, gera=%v", !c.gera, c.gera)
				}
				if c.gera {
					validarXSD(t, xmlBytes)
				} else {
					validarXSDEm(t, xmlBytes, original)
				}
			})
		}
	}
}

func TestVProdLiq_OverrideInvalido(t *testing.T) {
	e := entradaExemplo()
	e.ValorLiquido = "talvez"
	if _, _, err := builder.Build(e); err == nil || !strings.Contains(err.Error(), "ValorLiquido") {
		t.Fatalf("esperava erro de ValorLiquido, veio %v", err)
	}
}
