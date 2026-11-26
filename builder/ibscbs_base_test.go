package builder_test

import (
	"encoding/xml"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Base do IBS/CBS pelo art. 12 da LC 214/2025 (NT 2025.002-RTC, UB16-10):
//
//	vBC = vProd + vFrete + vSeg + vOutro - vDesc - vPIS - vCOFINS - vICMS
//	      - vICMSUFDest - vFCPUFDest - vICMSMono   (por item, valores impressos)
//
// arredondamento HALF_EVEN a centavos; pAliqEfet com 4 casas, impresso com 2-4.
// Os casos c* vêm da Calculadora RTC v1.7.2 (notas-fiscais internal/domain/testdata/rtc).

type ibsItemEsp struct {
	vBC, pIBSUF, vIBSUF, pIBSMun, vIBSMun, vIBS, pCBS, vCBS string
	pRed                                                    string    // "" = sem gRed
	efet                                                    [3]string // pAliqEfet UF, Mun, CBS
}

type ibsTotEsp struct{ vBC, vIBSUF, vIBSMun, vIBS, vCBS string }

func ibsItemSemImposto(cod string, qtd, vun float64, ib *builder.EntradaIBSCBS) builder.EntradaItem {
	return builder.EntradaItem{
		CProd: cod, CEAN: "SEM GTIN", Nome: "PRODUTO " + cod, NCM: "73089090",
		CFOP: "5102", Unidade: "UN", Quantidade: qtd, VUnitario: vun,
		ICMS:      builder.EntradaICMS{CST: "40"},
		PISCofins: builder.EntradaPISCofins{CST: "07"},
		IBSCBS:    ib,
	}
}

func ibsTaxado(cod string, qtd, vun, desc, aliqICMS float64, ib *builder.EntradaIBSCBS) builder.EntradaItem {
	it := ibsItemSemImposto(cod, qtd, vun, ib)
	it.VDesconto = desc
	it.ICMS = builder.EntradaICMS{CST: "00", Aliq: aliqICMS}
	it.PISCofins = builder.EntradaPISCofins{CST: "01", AliqPIS: 0.65, AliqCOFINS: 3}
	return it
}

func ibsAliq(uf, mun, cbs, red float64) *builder.EntradaIBSCBS {
	return &builder.EntradaIBSCBS{CST: "000", ClassTrib: "000001", AliqIBSUF: uf, AliqIBSMun: mun, AliqCBS: cbs, PercentualReducao: red}
}

func ibsDestDIFAL() builder.EntradaDest {
	return builder.EntradaDest{
		CPF: "52998224725", Nome: "CONSUMIDOR FINAL", IndIEDest: "9",
		End: builder.EntradaEndereco{
			Logradouro: "Av Paulista", Numero: "1000", Bairro: "Bela Vista",
			CodigoMun: "3550308", Municipio: "Sao Paulo", UF: "SP", CEP: "01310100",
			Pais: "1058", NomePais: "Brasil",
		},
	}
}

func TestIBSCBS_BaseArt12(t *testing.T) {
	casos := []struct {
		nome  string
		monta func() builder.EntradaNFe
		itens []ibsItemEsp
		tot   ibsTotEsp
		// premissas: valores de ICMS/PIS/COFINS do item 0 (vazio = não confere)
		icms, pis, cofins string
	}{
		{
			nome: "c07_2026",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				it := ibsTaxado("C07", 1, 1000, 50, 18, ibsAliq(0.10, 0, 0.90, 0))
				it.PISCofins = builder.EntradaPISCofins{CST: "01", AliqPIS: 1.65, AliqCOFINS: 7.6}
				e.Itens = []builder.EntradaItem{it}
				e.Frete = builder.EntradaFrete{Modalidade: "0", VFrete: 100}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1050}}
				return e
			},
			// 1000+100-50-180-16.50-76 = 777.50; UF 0.7775 -> 0.78; CBS 6.9975 -> 7.00
			icms: "180.00", pis: "16.50", cofins: "76.00",
			itens: []ibsItemEsp{{vBC: "777.50", pIBSUF: "0.10", vIBSUF: "0.78", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.78", pCBS: "0.90", vCBS: "7.00"}},
			tot:   ibsTotEsp{"777.50", "0.78", "0.00", "0.78", "7.00"},
		},
		{
			nome: "c08_2027",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				it := ibsItemSemImposto("C08", 1, 1000, ibsAliq(0.05, 0.05, 8.8, 0))
				it.VDesconto = 50
				e.Itens = []builder.EntradaItem{it}
				e.Frete = builder.EntradaFrete{Modalidade: "0", VFrete: 100}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1050}}
				return e
			},
			// 1000+100-50 = 1050; 1050*0.05% = 0.525 -> 0.52 (par); CBS 1050*8.8% = 92.40
			itens: []ibsItemEsp{{vBC: "1050.00", pIBSUF: "0.05", vIBSUF: "0.52", pIBSMun: "0.05", vIBSMun: "0.52", vIBS: "1.04", pCBS: "8.80", vCBS: "92.40"}},
			tot:   ibsTotEsp{"1050.00", "0.52", "0.52", "1.04", "92.40"},
		},
		{
			nome: "c11_2027_meio_centavo",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{
					ibsItemSemImposto("C11A", 1, 1050, ibsAliq(0.05, 0.05, 8.8, 0)),
					ibsItemSemImposto("C11B", 1, 1150, ibsAliq(0.05, 0.05, 8.8, 0)),
				}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 2200}}
				return e
			},
			// 1050: 0.525 -> 0.52 (par); 1150: 0.575 -> 0.58 (par); CBS 92.40 / 101.20
			itens: []ibsItemEsp{
				{vBC: "1050.00", pIBSUF: "0.05", vIBSUF: "0.52", pIBSMun: "0.05", vIBSMun: "0.52", vIBS: "1.04", pCBS: "8.80", vCBS: "92.40"},
				{vBC: "1150.00", pIBSUF: "0.05", vIBSUF: "0.58", pIBSMun: "0.05", vIBSMun: "0.58", vIBS: "1.16", pCBS: "8.80", vCBS: "101.20"},
			},
			tot: ibsTotEsp{"2200.00", "1.10", "1.10", "2.20", "193.60"},
		},
		{
			nome: "c05_2027_red30",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{ibsItemSemImposto("C05", 1, 10000, ibsAliq(0.05, 0.05, 8.8, 30))}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 10000}}
				return e
			},
			// efetiva: 0.05*0.7 = 0.035; CBS 8.8*0.7 = 6.16; 10000*0.035% = 3.50; CBS 616.00
			itens: []ibsItemEsp{{vBC: "10000.00", pIBSUF: "0.05", vIBSUF: "3.50", pIBSMun: "0.05", vIBSMun: "3.50", vIBS: "7.00", pCBS: "8.80", vCBS: "616.00",
				pRed: "30.00", efet: [3]string{"0.035", "0.035", "6.16"}}},
			tot: ibsTotEsp{"10000.00", "3.50", "3.50", "7.00", "616.00"},
		},
		{
			nome: "c03_2026_red60",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{ibsItemSemImposto("C03", 1, 1234.56, ibsAliq(0.10, 0, 0.90, 60))}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1234.56}}
				return e
			},
			// efetiva: 0.10*0.4 = 0.04; Mun 0.00; CBS 0.9*0.4 = 0.36
			// UF 1234.56*0.04% = 0.4938 -> 0.49; CBS 1234.56*0.36% = 4.4444 -> 4.44
			itens: []ibsItemEsp{{vBC: "1234.56", pIBSUF: "0.10", vIBSUF: "0.49", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.49", pCBS: "0.90", vCBS: "4.44",
				pRed: "60.00", efet: [3]string{"0.04", "0.00", "0.36"}}},
			tot: ibsTotEsp{"1234.56", "0.49", "0.00", "0.49", "4.44"},
		},
		{
			nome: "c01_2026",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{ibsItemSemImposto("C01", 1, 1000, ibsAliq(0.10, 0, 0.90, 0))}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}
				return e
			},
			itens: []ibsItemEsp{{vBC: "1000.00", pIBSUF: "0.10", vIBSUF: "1.00", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "1.00", pCBS: "0.90", vCBS: "9.00"}},
			tot:   ibsTotEsp{"1000.00", "1.00", "0.00", "1.00", "9.00"},
		},
		{
			// (i) 3 itens: A 1x600.00, B 1x1400.00, C 3x333.33 = 999.99 (soma 2999.99).
			// Rateio por valor em centavos, resto no último item:
			//   frete 100.00: A 10000*60000/299999 = 2000 | B 4666 | C 3333 +1 (resto) = 3334 -> 20.00 / 46.66 / 33.34
			//   seguro 30.00: A 600 | B 1400 | C 999 +1 = 1000 -> 6.00 / 14.00 / 10.00
			//   outras 12.50: A 250 | B 583 | C 416 +1 = 417 -> 2.50 / 5.83 / 4.17
			// desconto: A 0 | B 20.00 | C 9.99. ICMS 12% / PIS 0.65% / COFINS 3% sobre vProd:
			//   A 72.00/3.90/18.00 | B 168.00/9.10/42.00 | C 120.00 (119.9988)/6.50/30.00
			// vBC A = 600+20+6+2.50-72-3.90-18 = 534.60
			// vBC B = 1400+46.66+14+5.83-20-168-9.10-42 = 1227.39
			// vBC C = 999.99+33.34+10+4.17-9.99-120-6.50-30 = 881.01
			// UF 0.10%: 0.5346->0.53 | 1.22739->1.23 | 0.88101->0.88 ; CBS 0.90%: 4.8114->4.81 | 11.04651->11.05 | 7.92909->7.93
			nome: "extra_i_rateio_3_itens",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				ib := func() *builder.EntradaIBSCBS { return ibsAliq(0.10, 0, 0.90, 0) }
				e.Itens = []builder.EntradaItem{
					ibsTaxado("RA", 1, 600, 0, 12, ib()),
					ibsTaxado("RB", 1, 1400, 20, 12, ib()),
					ibsTaxado("RC", 3, 333.33, 9.99, 12, ib()),
				}
				e.Frete = builder.EntradaFrete{Modalidade: "0", VFrete: 100, VSeg: 30, VOutro: 12.50}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 3112.50}} // 2999.99+142.50-29.99
				return e
			},
			itens: []ibsItemEsp{
				{vBC: "534.60", pIBSUF: "0.10", vIBSUF: "0.53", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.53", pCBS: "0.90", vCBS: "4.81"},
				{vBC: "1227.39", pIBSUF: "0.10", vIBSUF: "1.23", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "1.23", pCBS: "0.90", vCBS: "11.05"},
				{vBC: "881.01", pIBSUF: "0.10", vIBSUF: "0.88", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.88", pCBS: "0.90", vCBS: "7.93"},
			},
			tot: ibsTotEsp{"2643.00", "2.64", "0.00", "2.64", "23.79"},
		},
		{
			// (ii) DIFAL: 1x1000, ICMS 12% = 120.00, PIS 6.50, COFINS 30.00,
			// vICMSUFDest = 1000*(18-12)% = 60.00, vFCPUFDest = 1000*2% = 20.00 (vICMSUFRemet = 0).
			// vBC = 1000-6.50-30-120-60-20 = 763.50; UF 0.7635->0.76; CBS 6.8715->6.87
			nome: "extra_ii_difal",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Dest, e.IndFinal = ibsDestDIFAL(), "1"
				it := ibsTaxado("DF", 1, 1000, 0, 12, ibsAliq(0.10, 0, 0.90, 0))
				it.CFOP = "6108"
				it.ICMSUFDest = &builder.EntradaICMSUFDest{AliqInterna: 18, AliqInterestadual: 12, AliqFCP: 2}
				e.Itens = []builder.EntradaItem{it}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1000}}
				return e
			},
			icms: "120.00", pis: "6.50", cofins: "30.00",
			itens: []ibsItemEsp{{vBC: "763.50", pIBSUF: "0.10", vIBSUF: "0.76", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.76", pCBS: "0.90", vCBS: "6.87"}},
			tot:   ibsTotEsp{"763.50", "0.76", "0.00", "0.76", "6.87"},
		},
		{
			// (iii) ICMS-ST (CST 10): ICMS 12% = 120.00; vBCST = 1000*1.40 = 1400; vICMSST = 1400*18%-120 = 132.00.
			// vICMSST NÃO entra: vBC = 1000-6.50-30-120 = 843.50; UF 0.8435->0.84; CBS 7.5915->7.59
			nome: "extra_iii_icms_st_nao_subtrai",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				it := ibsTaxado("ST", 1, 1000, 0, 12, ibsAliq(0.10, 0, 0.90, 0))
				it.CEST = "1000100"
				it.ICMS = builder.EntradaICMS{CST: "10", Aliq: 12, PMVAST: 40, AliqST: 18}
				e.Itens = []builder.EntradaItem{it}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1132}} // vProd + vICMSST
				return e
			},
			icms: "120.00", pis: "6.50", cofins: "30.00",
			itens: []ibsItemEsp{{vBC: "843.50", pIBSUF: "0.10", vIBSUF: "0.84", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.84", pCBS: "0.90", vCBS: "7.59"}},
			tot:   ibsTotEsp{"843.50", "0.84", "0.00", "0.84", "7.59"},
		},
		{
			// (iv) desconto maior que o resto: item 1: 100.00 - 100.00 - 12.00 - 0.65 - 3.00 = -15.65 -> vBC 0.00 (piso), tributos 0.00.
			// Item 2 (500.00, sem tributos) mantém a nota válida: vBC 500.00; UF 0.50; CBS 4.50.
			nome: "extra_iv_desconto_maior_zera",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{
					ibsTaxado("Z1", 1, 100, 100, 12, ibsAliq(0.10, 0, 0.90, 0)),
					ibsItemSemImposto("Z2", 1, 500, ibsAliq(0.10, 0, 0.90, 0)),
				}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 500}}
				return e
			},
			itens: []ibsItemEsp{
				{vBC: "0.00", pIBSUF: "0.10", vIBSUF: "0.00", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.00", pCBS: "0.90", vCBS: "0.00"},
				{vBC: "500.00", pIBSUF: "0.10", vIBSUF: "0.50", pIBSMun: "0.00", vIBSMun: "0.00", vIBS: "0.50", pCBS: "0.90", vCBS: "4.50"},
			},
			tot: ibsTotEsp{"500.00", "0.50", "0.00", "0.50", "4.50"},
		},
		{
			// (v-a) empate que cai para baixo: 1250.00 * 0.05% = 0.625 -> 0.62 (2 é par), UF e Mun; vIBS 1.24; CBS 0.05% idem 0.62
			nome: "extra_v_empate_para_baixo",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{ibsItemSemImposto("E1", 1, 1250, ibsAliq(0.05, 0.05, 0.05, 0))}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1250}}
				return e
			},
			itens: []ibsItemEsp{{vBC: "1250.00", pIBSUF: "0.05", vIBSUF: "0.62", pIBSMun: "0.05", vIBSMun: "0.62", vIBS: "1.24", pCBS: "0.05", vCBS: "0.62"}},
			tot:   ibsTotEsp{"1250.00", "0.62", "0.62", "1.24", "0.62"},
		},
		{
			// (v-b) empate que sobe para o par: 1270.00 * 0.05% = 0.635 -> 0.64 (4 é par); vIBS 1.28
			nome: "extra_v_empate_para_cima",
			monta: func() builder.EntradaNFe {
				e := entradaCRT3()
				e.Itens = []builder.EntradaItem{ibsItemSemImposto("E2", 1, 1270, ibsAliq(0.05, 0.05, 0.05, 0))}
				e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1270}}
				return e
			},
			itens: []ibsItemEsp{{vBC: "1270.00", pIBSUF: "0.05", vIBSUF: "0.64", pIBSMun: "0.05", vIBSMun: "0.64", vIBS: "1.28", pCBS: "0.05", vCBS: "0.64"}},
			tot:   ibsTotEsp{"1270.00", "0.64", "0.64", "1.28", "0.64"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			xmlBytes, _, err := builder.Build(c.monta())
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			var nfe builder.NFe
			if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
				t.Fatalf("XML inválido: %v", err)
			}
			det := nfe.InfNFe.Det
			if len(det) != len(c.itens) {
				t.Fatalf("%d itens, esperava %d", len(det), len(c.itens))
			}
			eq := func(campo, got, want string) {
				t.Helper()
				if got != want {
					t.Errorf("%s = %q, esperava %q", campo, got, want)
				}
			}
			if c.icms != "" {
				imp := det[0].Imposto
				if imp.ICMS.ICMS00 != nil {
					eq("premissa vICMS", imp.ICMS.ICMS00.VICMS, c.icms)
				} else if imp.ICMS.ICMS10 != nil {
					eq("premissa vICMS", imp.ICMS.ICMS10.VICMS, c.icms)
				}
				eq("premissa vPIS", imp.PIS.PISAliq.VPIS, c.pis)
				eq("premissa vCOFINS", imp.COFINS.COFINSAliq.VCOFINS, c.cofins)
			}
			for i, w := range c.itens {
				ib := det[i].Imposto.IBSCBS
				if ib == nil || ib.GIBSCBS == nil {
					t.Fatalf("item %d sem gIBSCBS", i+1)
				}
				g := ib.GIBSCBS
				p := func(s string) string { return "item " + string(rune('1'+i)) + " " + s }
				eq(p("vBC"), g.VBC, w.vBC)
				eq(p("pIBSUF"), g.GIBSUF.PIBSUF, w.pIBSUF)
				eq(p("vIBSUF"), g.GIBSUF.VIBSUF, w.vIBSUF)
				eq(p("pIBSMun"), g.GIBSMun.PIBSMun, w.pIBSMun)
				eq(p("vIBSMun"), g.GIBSMun.VIBSMun, w.vIBSMun)
				eq(p("vIBS"), g.VIBS, w.vIBS)
				eq(p("pCBS"), g.GCBS.PCBS, w.pCBS)
				eq(p("vCBS"), g.GCBS.VCBS, w.vCBS)
				if w.pRed == "" {
					if g.GIBSUF.GRed != nil || g.GIBSMun.GRed != nil || g.GCBS.GRed != nil {
						t.Errorf("%s: gRed não esperado", p(""))
					}
					continue
				}
				for k, r := range []*builder.GRed{g.GIBSUF.GRed, g.GIBSMun.GRed, g.GCBS.GRed} {
					if r == nil {
						t.Errorf("%s: gRed ausente (%d)", p(""), k)
						continue
					}
					eq(p("pRedAliq"), r.PRedAliq, w.pRed)
					eq(p("pAliqEfet"), r.PAliqEfet, w.efet[k])
				}
			}
			tot := nfe.InfNFe.Total.IBSCBSTot
			if tot == nil {
				t.Fatal("sem IBSCBSTot")
			}
			eq("tot vBCIBSCBS", tot.VBCIBSCBS, c.tot.vBC)
			eq("tot vIBSUF", tot.GIBS.GIBSUF.VIBSUF, c.tot.vIBSUF)
			eq("tot vIBSMun", tot.GIBS.GIBSMun.VIBSMun, c.tot.vIBSMun)
			eq("tot vIBS", tot.GIBS.VIBS, c.tot.vIBS)
			eq("tot vCBS", tot.GCBS.VCBS, c.tot.vCBS)
		})
	}
}
