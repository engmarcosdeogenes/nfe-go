package builder_test

import (
	"encoding/xml"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

func buildNFe(t *testing.T, e builder.EntradaNFe) builder.NFe {
	t.Helper()
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}
	return nfe
}

func itemOrigem(icms builder.EntradaICMS) builder.EntradaItem {
	return builder.EntradaItem{
		CProd: "P1", CEAN: "SEM GTIN", Nome: "PECA", NCM: "85365090", CFOP: "5102",
		Unidade: "UN", Quantidade: 1, VUnitario: 100, ICMS: icms,
	}
}

func TestOrigem_CRT3_CampoOrigComCST00(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{Orig: "2", CST: "00", Aliq: 4})}
	icms := buildNFe(t, e).InfNFe.Det[0].Imposto.ICMS.ICMS00
	if icms == nil || icms.Orig != "2" || icms.CST != "00" || icms.PICMS != "4.00" {
		t.Fatalf("esperava ICMS00 orig 2 pICMS 4.00, veio %+v", icms)
	}
}

func TestOrigem_CST3Digitos(t *testing.T) {
	casos := []struct{ cst, orig, wantOrig, wantCST string }{
		{"200", "", "2", "00"},
		{"700", "", "7", "00"},
		{"000", "", "0", "00"},
		{"060", "", "0", "60"},
		{"200", "1", "1", "00"}, // origem explícita tem prioridade
	}
	for _, c := range casos {
		e := entradaCRT3()
		e.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{Orig: c.orig, CST: c.cst, Aliq: 12, VBCSTRet: 1, PST: 1, VICMSSTRet: 1})}
		icms := buildNFe(t, e).InfNFe.Det[0].Imposto.ICMS
		var gotOrig, gotCST string
		switch {
		case icms.ICMS00 != nil:
			gotOrig, gotCST = icms.ICMS00.Orig, icms.ICMS00.CST
		case icms.ICMS60 != nil:
			gotOrig, gotCST = icms.ICMS60.Orig, icms.ICMS60.CST
		}
		if gotOrig != c.wantOrig || gotCST != c.wantCST {
			t.Errorf("CST %q orig %q: veio orig %q CST %q, esperava %q %q", c.cst, c.orig, gotOrig, gotCST, c.wantOrig, c.wantCST)
		}
	}
}

func TestOrigem_TodosOsGruposCST(t *testing.T) {
	for _, cst := range []string{"00", "10", "20", "40", "41", "50", "60"} {
		e := entradaCRT3()
		e.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{Orig: "8", CST: cst, Aliq: 12, AliqST: 18, PRedBC: 10})}
		icms := buildNFe(t, e).InfNFe.Det[0].Imposto.ICMS
		var orig string
		switch {
		case icms.ICMS00 != nil:
			orig = icms.ICMS00.Orig
		case icms.ICMS10 != nil:
			orig = icms.ICMS10.Orig
		case icms.ICMS20 != nil:
			orig = icms.ICMS20.Orig
		case icms.ICMS40 != nil:
			orig = icms.ICMS40.Orig
		case icms.ICMS60 != nil:
			orig = icms.ICMS60.Orig
		}
		if orig != "8" {
			t.Errorf("CST %s: orig %q, esperava 8", cst, orig)
		}
	}
}

func TestOrigem_CSOSN(t *testing.T) {
	casos := []builder.EntradaICMS{
		{Orig: "1", CSOSN: "102"},
		{Orig: "1", CSOSN: "101", Aliq: 2},
		{Orig: "1", CSOSN: "201", Aliq: 2, AliqST: 18},
		{Orig: "1", CSOSN: "202", AliqST: 18},
		{Orig: "1", CSOSN: "500"},
	}
	for _, c := range casos {
		e := entradaExemplo()
		e.Itens = []builder.EntradaItem{itemOrigem(c)}
		icms := buildNFe(t, e).InfNFe.Det[0].Imposto.ICMS
		var orig, csosn string
		switch {
		case icms.ICMSSN101 != nil:
			orig, csosn = icms.ICMSSN101.Orig, icms.ICMSSN101.CSOSN
		case icms.ICMSSN102 != nil:
			orig, csosn = icms.ICMSSN102.Orig, icms.ICMSSN102.CSOSN
		case icms.ICMSSN201 != nil:
			orig, csosn = icms.ICMSSN201.Orig, icms.ICMSSN201.CSOSN
		case icms.ICMSSN202 != nil:
			orig, csosn = icms.ICMSSN202.Orig, icms.ICMSSN202.CSOSN
		case icms.ICMSSN500 != nil:
			orig, csosn = icms.ICMSSN500.Orig, icms.ICMSSN500.CSOSN
		}
		if orig != "1" || csosn != c.CSOSN {
			t.Errorf("CSOSN %s: veio orig %q csosn %q", c.CSOSN, orig, csosn)
		}
	}
}

func TestOrigem_VaziaContinuaZero(t *testing.T) {
	e := entradaExemplo()
	if o := buildNFe(t, e).InfNFe.Det[0].Imposto.ICMS; o.ICMSSN102 == nil || o.ICMSSN102.Orig != "0" {
		t.Fatalf("Simples sem origem deveria sair orig 0: %+v", o.ICMSSN102)
	}
}

func TestOrigem_InvalidaRecusada(t *testing.T) {
	for _, orig := range []string{"9", "A", "10"} {
		e := entradaCRT3()
		e.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{Orig: orig, CST: "00", Aliq: 12})}
		if _, _, err := builder.Build(e); err == nil {
			t.Errorf("origem %q deveria ser recusada", orig)
		}
	}
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{CST: "900", Aliq: 12})}
	if _, _, err := builder.Build(e); err == nil {
		t.Error(`CST "900" (origem 9 não existe) deveria ser recusado`)
	}
}

// Nota 156 do cliente: motor (origem 7) a 12% e peça importada (origem 2) a 4%
// na mesma NF-e para consumidor final em SP (interna 18%, FCP 2%).
func notaMista() builder.EntradaNFe {
	e := entradaCRT3()
	e.Dest.End.UF = "SP"
	e.Dest.IndIEDest = "9"
	motor := builder.EntradaItem{
		CProd: "M1", CEAN: "SEM GTIN", Nome: "MOTOR", NCM: "89039900", CFOP: "6102",
		Unidade: "UN", Quantidade: 1, VUnitario: 1000,
		ICMS:       builder.EntradaICMS{CST: "700", Aliq: 12},
		ICMSUFDest: &builder.EntradaICMSUFDest{AliqInterna: 18, AliqInterestadual: 12, AliqFCP: 2},
	}
	peca := builder.EntradaItem{
		CProd: "P1", CEAN: "SEM GTIN", Nome: "PECA", NCM: "85365090", CFOP: "6102",
		Unidade: "UN", Quantidade: 2, VUnitario: 250,
		ICMS:       builder.EntradaICMS{Orig: "2", CST: "00", Aliq: 4},
		ICMSUFDest: &builder.EntradaICMSUFDest{AliqInterna: 18, AliqInterestadual: 4, AliqFCP: 2},
	}
	e.Itens = []builder.EntradaItem{motor, peca}
	e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 1500}}
	return e
}

func TestOrigem_DIFALComAliquotasDiferentesNaMesmaNota(t *testing.T) {
	nfe := buildNFe(t, notaMista())
	m, p := nfe.InfNFe.Det[0].Imposto, nfe.InfNFe.Det[1].Imposto
	if m.ICMS.ICMS00.Orig != "7" || p.ICMS.ICMS00.Orig != "2" {
		t.Errorf("origens: motor %q peça %q", m.ICMS.ICMS00.Orig, p.ICMS.ICMS00.Orig)
	}
	// motor: 1000*(18-12)% = 60; peça: 500*(18-4)% = 70
	if m.ICMSUFDest.PICMSInter != "12.00" || m.ICMSUFDest.VICMSUFDest != "60.00" {
		t.Errorf("motor DIFAL: %+v", m.ICMSUFDest)
	}
	if p.ICMSUFDest.PICMSInter != "4.00" || p.ICMSUFDest.VICMSUFDest != "70.00" {
		t.Errorf("peça DIFAL: %+v", p.ICMSUFDest)
	}
	tot := nfe.InfNFe.Total.ICMSTot
	// vICMSUFDest = 60+70; vFCPUFDest = 1500*2%; vICMS = 120+20
	if tot.VICMSUFDest != "130.00" || tot.VFCPUFDest != "30.00" || tot.VICMS != "140.00" {
		t.Errorf("totais: vICMSUFDest %s vFCPUFDest %s vICMS %s", tot.VICMSUFDest, tot.VFCPUFDest, tot.VICMS)
	}
}

func TestXSD_Origem(t *testing.T) {
	sn := entradaExemplo()
	sn.Itens = []builder.EntradaItem{itemOrigem(builder.EntradaICMS{Orig: "1", CSOSN: "102"})}
	for nome, e := range map[string]builder.EntradaNFe{"nota_mista_difal": notaMista(), "csosn102_orig1": sn} {
		t.Run(nome, func(t *testing.T) {
			xmlBytes, _, err := builder.Build(e)
			if err != nil {
				t.Fatal(err)
			}
			validarXSD(t, xmlBytes)
		})
	}
}
