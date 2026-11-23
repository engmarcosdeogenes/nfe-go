package builder_test

import (
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

func itemICMS(icms builder.EntradaICMS, cfop string) builder.EntradaItem {
	return builder.EntradaItem{
		CProd: "P1", CEAN: "SEM GTIN", Nome: "ITEM", NCM: "22030000", CEST: "0302100", CFOP: cfop,
		Unidade: "UN", Quantidade: 3, VUnitario: 33.33, ICMS: icms,
	}
}

func montarComItem(t *testing.T, base builder.EntradaNFe, it builder.EntradaItem) ([]byte, builder.NFe) {
	t.Helper()
	base.Itens = []builder.EntradaItem{it}
	if base.Mod == builder.ModeloNFCe {
		base.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}
	}
	xmlBytes, _, err := builder.Build(base)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}
	return xmlBytes, nfe
}

func num(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("valor %q: %v", s, err)
	}
	return v
}

// iguais compara como a SEFAZ: valores impressos, tolerância de 1 centavo.
func iguais(a, b float64) bool { return a-b < 0.011 && b-a < 0.011 }

// CST 30, 51, 70 e ST com redução/modalidade: XML montado passa no XSD 4.00 e as
// contas batem com as regras da NT (351/352/353 diferimento, 528 vICMS, 933 pMVAST).
func TestICMS_CST30_51_70_XSDERegras(t *testing.T) {
	casos := map[string]builder.EntradaItem{
		"cst30":             itemICMS(builder.EntradaICMS{CST: "30", Aliq: 12, PMVAST: 40, AliqST: 18}, "5403"),
		"cst30_mod6":        itemICMS(builder.EntradaICMS{CST: "30", ModBCST: "6", AliqST: 18}, "5403"),
		"cst51_parcial":     itemICMS(builder.EntradaICMS{CST: "51", Aliq: 18, PDif: 33.33}, "5101"),
		"cst51_total":       itemICMS(builder.EntradaICMS{CST: "51", Aliq: 12, PDif: 100}, "5101"),
		"cst51_red":         itemICMS(builder.EntradaICMS{CST: "51", Aliq: 18, PRedBC: 41.67, PDif: 50}, "5101"),
		"cst70":             itemICMS(builder.EntradaICMS{CST: "70", Aliq: 18, PRedBC: 20, PMVAST: 35, AliqST: 18}, "5401"),
		"cst70_redst":       itemICMS(builder.EntradaICMS{CST: "70", Aliq: 18, PRedBC: 20, PMVAST: 35, PRedBCST: 20, AliqST: 18}, "5401"),
		"cst10_mod6":        itemICMS(builder.EntradaICMS{CST: "10", Aliq: 12, ModBCST: "6", AliqST: 18}, "5401"),
		"cst10_redst":       itemICMS(builder.EntradaICMS{CST: "10", Aliq: 12, PMVAST: 40, PRedBCST: 10, AliqST: 18}, "5401"),
		"cst60_origem_3dig": itemICMS(builder.EntradaICMS{CST: "260", VBCSTRet: 120, PST: 18, VICMSSTRet: 21.6}, "5405"),
	}
	for nome, it := range casos {
		t.Run(nome, func(t *testing.T) {
			xmlBytes, nfe := montarComItem(t, entradaCRT3(), it)
			validarXSD(t, xmlBytes)
			icms := nfe.InfNFe.Det[0].Imposto.ICMS
			vProd := 99.99
			switch {
			case icms.ICMS51 != nil:
				g := icms.ICMS51
				vBC, p, op, pDif, dif, v := num(t, g.VBC), num(t, g.PICMS), num(t, g.VICMSOp), num(t, g.PDif), num(t, g.VICMSDif), num(t, g.VICMS)
				if !iguais(op, vBC*p/100) {
					t.Errorf("351: vICMSOp %.2f != vBC*pICMS %.4f", op, vBC*p/100)
				}
				if !iguais(dif, op*pDif/100) {
					t.Errorf("352: vICMSDif %.2f != vICMSOp*pDif %.4f", dif, op*pDif/100)
				}
				if v != num(t, strconv.FormatFloat(op-dif, 'f', 2, 64)) {
					t.Errorf("353: vICMS %.2f != vICMSOp-vICMSDif %.2f", v, op-dif)
				}
				if !iguais(vBC, vProd*(1-num(t, g.PRedBC)/100)) {
					t.Errorf("vBC %.2f com pRedBC %s", vBC, g.PRedBC)
				}
				if tot := num(t, nfe.InfNFe.Total.ICMSTot.VICMS); tot != v {
					t.Errorf("ICMSTot/vICMS %.2f, esperado o vICMS (não diferido) %.2f", tot, v)
				}
			case icms.ICMS70 != nil:
				g := icms.ICMS70
				if !iguais(num(t, g.VICMS), num(t, g.VBC)*num(t, g.PICMS)/100) {
					t.Errorf("528: vICMS %s != vBC*pICMS", g.VICMS)
				}
				bcst := vProd * (1 + num(t, g.PMVAST)/100)
				if g.PRedBCST != "" {
					bcst *= 1 - num(t, g.PRedBCST)/100
				}
				if !iguais(num(t, g.VBCST), bcst) {
					t.Errorf("vBCST %s, esperado %.2f", g.VBCST, bcst)
				}
				if !iguais(num(t, g.VICMSST), bcst*0.18-num(t, g.VICMS)) {
					t.Errorf("vICMSST %s, esperado BCST*18%% - ICMS próprio", g.VICMSST)
				}
				if tot := nfe.InfNFe.Total.ICMSTot; tot.VST != g.VICMSST || tot.VBCST != g.VBCST {
					t.Errorf("total vST/vBCST %s/%s != item %s/%s", tot.VST, tot.VBCST, g.VICMSST, g.VBCST)
				}
			case icms.ICMS30 != nil:
				g := icms.ICMS30
				if g.ModBCST == "6" && (g.PMVAST != "" || !iguais(num(t, g.VBCST), vProd)) {
					t.Errorf("modBCST 6: pMVAST %q (933: deve faltar) e vBCST %s (esperado vProd)", g.PMVAST, g.VBCST)
				}
				if g.ModBCST == "4" && !iguais(num(t, g.VICMSST), vProd*1.4*0.18-vProd*0.12) {
					t.Errorf("vICMSST %s: esperado BCST*18%% - ICMS da operação própria", g.VICMSST)
				}
				if nfe.InfNFe.Total.ICMSTot.VICMS != "0.00" {
					t.Errorf("CST 30 não tem ICMS próprio; total vICMS %s", nfe.InfNFe.Total.ICMSTot.VICMS)
				}
			case icms.ICMS10 != nil:
				g := icms.ICMS10
				if (g.ModBCST == "4") != (g.PMVAST != "") {
					t.Errorf("932/933: modBCST %s com pMVAST %q", g.ModBCST, g.PMVAST)
				}
			case icms.ICMS60 != nil:
				if icms.ICMS60.Orig != "2" || icms.ICMS60.CST != "60" {
					t.Errorf("CST 260: orig/CST %s/%s", icms.ICMS60.Orig, icms.ICMS60.CST)
				}
			default:
				t.Fatalf("grupo ICMS inesperado: %+v", icms)
			}
		})
	}
}

func TestICMS_ST_SimplesXSD(t *testing.T) {
	casos := map[string]builder.EntradaItem{
		"csosn201_redst": itemICMS(builder.EntradaICMS{CSOSN: "201", Aliq: 2.5, PMVAST: 40, PRedBCST: 10, AliqST: 18}, "5401"),
		"csosn202_mod6":  itemICMS(builder.EntradaICMS{CSOSN: "202", ModBCST: "6", AliqST: 18}, "5401"),
		"csosn500":       itemICMS(builder.EntradaICMS{CSOSN: "500", VBCSTRet: 120, PST: 18, VICMSSTRet: 21.6}, "5405"),
	}
	for nome, it := range casos {
		t.Run(nome, func(t *testing.T) {
			xmlBytes, nfe := montarComItem(t, entradaExemplo(), it)
			validarXSD(t, xmlBytes)
			if g := nfe.InfNFe.Det[0].Imposto.ICMS.ICMSSN202; g != nil && g.PMVAST != "" {
				t.Errorf("933: pMVAST %q com modBCST %s", g.PMVAST, g.ModBCST)
			}
		})
	}
}

func TestICMS_RecusasExplicitas(t *testing.T) {
	casos := map[string]builder.EntradaICMS{
		"cst70_sem_reducao": {CST: "70", Aliq: 18, PMVAST: 35, AliqST: 18},
		"cst30_sem_aliqst":  {CST: "30", PMVAST: 35},
		"cst51_pdif_101":    {CST: "51", Aliq: 18, PDif: 101},
		"cst51_sem_pdif":    {CST: "51", Aliq: 18},
		"mva_com_mod6":      {CST: "10", Aliq: 12, ModBCST: "6", PMVAST: 40, AliqST: 18},
		"modbcst_pauta":     {CST: "10", Aliq: 12, ModBCST: "5", AliqST: 18},
	}
	for nome, icms := range casos {
		t.Run(nome, func(t *testing.T) {
			e := entradaCRT3()
			e.Itens = []builder.EntradaItem{itemICMS(icms, "5401")}
			if _, _, err := builder.Build(e); err == nil {
				t.Fatal("esperava erro do builder, não XML que a SEFAZ recusa")
			}
		})
	}
}

// vTotTrib (Lei 12.741): item informado sai em imposto/vTotTrib e o total W16a é a
// soma dos itens impressos (rejeição 685); item sem valor omite a tag.
func TestVTotTrib_ItensETotal(t *testing.T) {
	e := entradaNFCe()
	a := itemICMS(builder.EntradaICMS{CSOSN: "102"}, "5102")
	a.VTotTrib = 10.006
	b := itemICMS(builder.EntradaICMS{CSOSN: "102"}, "5102")
	b.CProd, b.VTotTrib = "P2", 3.333
	c := itemICMS(builder.EntradaICMS{CSOSN: "102"}, "5102")
	c.CProd = "P3"
	e.Itens = []builder.EntradaItem{a, b, c}
	e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatal(err)
	}
	validarXSD(t, xmlBytes)
	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatal(err)
	}
	got := []string{nfe.InfNFe.Det[0].Imposto.VTotTrib, nfe.InfNFe.Det[1].Imposto.VTotTrib, nfe.InfNFe.Det[2].Imposto.VTotTrib}
	if strings.Join(got, "|") != "10.01|3.33|" {
		t.Errorf("vTotTrib dos itens = %q", got)
	}
	if tot := nfe.InfNFe.Total.ICMSTot.VTotTrib; tot != "13.34" {
		t.Errorf("ICMSTot/vTotTrib = %s, esperado 13.34 (soma dos itens impressos)", tot)
	}
	e.Itens[0].VTotTrib = -1
	if _, _, err := builder.Build(e); err == nil {
		t.Error("vTotTrib negativo deveria ser recusado")
	}
}
