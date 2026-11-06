package builder_test

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// TestIBSCBSTot_SomaDosItensArredondados: o total W03 tem que ser a soma dos
// valores IMPRESSOS nos itens (arredondados a centavos), não a soma do valor
// cru. Caso real: NF-e 109 da ATUAR (06/11/2026), 16 itens com pIBSUF 0,10 —
// itens impressos somavam 21,31 e o total saía 21,29; com arredondamento meio-para-cima correto os itens somam 21,32. Rejeitada com cStat=1080.
func TestIBSCBSTot_SomaDosItensArredondados(t *testing.T) {
	vProds := []float64{1125.00, 235.50, 669.00, 66.60, 2145.00, 5440.00, 411.00, 558.00,
		3850.00, 276.00, 2394.00, 340.00, 275.00, 570.00, 217.50, 2720.00}
	e := entradaCRT3()
	e.Itens = nil
	for i, v := range vProds {
		e.Itens = append(e.Itens, builder.EntradaItem{
			CProd: fmt.Sprintf("P%03d", i+1), CEAN: "SEM GTIN", Nome: "PRODUTO TESTE",
			NCM: "39269090", CFOP: "5102", Unidade: "UN",
			Quantidade: 1, VUnitario: v,
			ICMS: builder.EntradaICMS{CST: "00", Aliq: 12.0},
			IBSCBS: &builder.EntradaIBSCBS{
				CST: "000", ClassTrib: "000001",
				AliqIBSUF: 0.10, AliqIBSMun: 0, AliqCBS: 0.9,
			},
		})
	}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}
	cent := func(s string) int64 {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("valor %q: %v", s, err)
		}
		return int64(math.Round(f * 100))
	}
	var uf, mun, ibs, cbs int64
	for _, d := range nfe.InfNFe.Det {
		g := d.Imposto.IBSCBS.GIBSCBS
		uf += cent(g.GIBSUF.VIBSUF)
		mun += cent(g.GIBSMun.VIBSMun)
		ibs += cent(g.VIBS)
		cbs += cent(g.GCBS.VCBS)
	}
	tot := nfe.InfNFe.Total.IBSCBSTot
	if got := cent(tot.GIBS.GIBSUF.VIBSUF); got != uf {
		t.Errorf("total vIBSUF = %d centavos, soma dos itens = %d", got, uf)
	}
	if uf != 2132 {
		t.Errorf("soma vIBSUF dos itens = %d, esperava 2132 (21,32: meio para cima, 1,125 -> 1,13)", uf)
	}
	if got := cent(tot.GIBS.GIBSMun.VIBSMun); got != mun {
		t.Errorf("total vIBSMun = %d, soma dos itens = %d", got, mun)
	}
	if got := cent(tot.GIBS.VIBS); got != ibs {
		t.Errorf("total vIBS = %d, soma dos itens = %d", got, ibs)
	}
	if got := cent(tot.GCBS.VCBS); got != cbs {
		t.Errorf("total vCBS = %d, soma dos itens = %d", got, cbs)
	}
}
