package builder_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Golden de não regressão: notas SEM IBS/CBS (item.IBSCBS == nil) cujo XML não
// pode mudar com a correção da base de cálculo do IBS/CBS (vBC pelo art. 12 da
// LC 214/2025). Regenerar os goldens (só de propósito):
//
//	NFE_ATUALIZAR_GOLDEN=1 go test -run TestNaoRegressao ./builder/
//
// O cNF da chave é aleatório; a chave (Id, cNF, cDV, QR Code) é normalizada
// antes da comparação.
var (
	reCNF = regexp.MustCompile(`<cNF>\d+</cNF>`)
	reCDV = regexp.MustCompile(`<cDV>\d</cDV>`)
	reQR  = regexp.MustCompile(`<qrCode>.*?</qrCode>`)
)

func normalizarGolden(x []byte, chave string) string {
	s := strings.ReplaceAll(string(x), chave, strings.Repeat("X", 44))
	s = reCNF.ReplaceAllString(s, "<cNF>00000000</cNF>")
	s = reCDV.ReplaceAllString(s, "<cDV>0</cDV>")
	return reQR.ReplaceAllString(s, "<qrCode>QR</qrCode>")
}

func TestNaoRegressao_SemIBSCBS(t *testing.T) {
	item := func(cod string, qtd, vun, desc float64, icms builder.EntradaICMS) builder.EntradaItem {
		return builder.EntradaItem{
			CProd: cod, CEAN: "SEM GTIN", Nome: "PRODUTO " + cod, NCM: "73089090",
			CFOP: "5102", Unidade: "UN", Quantidade: qtd, VUnitario: vun, VDesconto: desc, ICMS: icms,
		}
	}
	casos := []struct {
		nome string
		e    func() builder.EntradaNFe
	}{
		{"a_crt3_icms00_rateio", func() builder.EntradaNFe {
			e := entradaCRT3()
			e.Itens = []builder.EntradaItem{
				item("A1", 3, 100.00, 0, builder.EntradaICMS{CST: "00", Aliq: 12}),
				item("A2", 7, 33.33, 10.00, builder.EntradaICMS{CST: "00", Aliq: 18}),
				item("A3", 1, 1234.56, 0, builder.EntradaICMS{CST: "00", Aliq: 12}),
			}
			e.Frete = builder.EntradaFrete{Modalidade: "0", VFrete: 100.00, VSeg: 25.50, VOutro: 40.00}
			e.Pagamento = []builder.EntradaPagamento{{Forma: "15", Valor: 1900.00, APrazo: true}}
			return e
		}},
		{"b_crt3_difal", func() builder.EntradaNFe {
			e := entradaCRT3()
			e.Dest = builder.EntradaDest{
				CPF: "52998224725", Nome: "CONSUMIDOR FINAL", IndIEDest: "9",
				End: builder.EntradaEndereco{
					Logradouro: "Av Paulista", Numero: "1000", Bairro: "Bela Vista",
					CodigoMun: "3550308", Municipio: "Sao Paulo", UF: "SP", CEP: "01310100",
					Pais: "1058", NomePais: "Brasil",
				},
			}
			e.IndFinal = "1"
			it := item("B1", 2, 500.00, 0, builder.EntradaICMS{CST: "00", Aliq: 12})
			it.CFOP = "6108"
			it.ICMSUFDest = &builder.EntradaICMSUFDest{AliqInterna: 18, AliqInterestadual: 12, AliqFCP: 2}
			e.Itens = []builder.EntradaItem{it}
			e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000.00}}
			return e
		}},
		{"c_nfce_simples", func() builder.EntradaNFe {
			e := entradaNFCe()
			e.ValorLiquido = "sim"
			return e
		}},
		{"d_crt3_icms40_deson", func() builder.EntradaNFe {
			e := entradaCRT3()
			e.Itens = []builder.EntradaItem{
				item("D1", 1, 100.00, 0, builder.EntradaICMS{CST: "40", VICMSDeson: 19, MotDesICMS: "9"}),
				item("D2", 2, 50.00, 5.00, builder.EntradaICMS{CST: "00", Aliq: 12}),
			}
			e.Pagamento = []builder.EntradaPagamento{{Forma: "15", Valor: 195.00, APrazo: true}}
			return e
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := c.e()
			e.ValorLiquido = "sim"
			x, chave, err := builder.Build(e)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			got := normalizarGolden(x, chave.String())
			if strings.Contains(got, "<IBSCBS") {
				t.Fatal("nota do golden não pode ter IBS/CBS")
			}
			path := "testdata/golden/sem_ibscbs_" + c.nome + ".xml"
			if os.Getenv("NFE_ATUALIZAR_GOLDEN") != "" {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("XML difere do golden %s\n got: %s\nwant: %s", path, got, want)
			}
		})
	}
}
