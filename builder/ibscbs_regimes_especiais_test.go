package builder_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Testes de forma (marshal direto, sem passar por Build) pros grupos
// opcionais do IBS/CBS (NT 2025.002-RTC) que nenhum cliente real usa ainda —
// confirmam só que a struct Go serializa na ordem/aninhamento exigidos pelo
// XSD real (DFeTiposBasicos_v1.00.xsd, TTribNFe/TCIBS_NFe), não regra de
// negócio. Cálculo/decisão de quando aplicar cada um fica pra quando
// aparecer cliente que precise.

func TestIBSCBS_GDifGDevTribGRed_DentroDeGIBSUF(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "051", ClassTrib: "200003",
		GIBSCBS: &builder.GIBSCBS{
			VBC: "1000.00",
			GIBSUF: builder.GIBSUF{
				PIBSUF:   "0.10",
				GDif:     &builder.GDif{PDif: "50.00", VDif: "0.50"},
				GDevTrib: &builder.GDevTrib{PDevTrib: "10.00", VDevTrib: "0.10"},
				GRed:     &builder.GRed{PRedAliq: "20.00", PAliqEfet: "0.08"},
				VIBSUF:   "0.50",
			},
			GIBSMun: builder.GIBSMun{PIBSMun: "0.00", VIBSMun: "0.00"},
			VIBS:    "0.50",
			GCBS:    builder.GCBS{PCBS: "0.90", VCBS: "9.00"},
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	// Ordem exigida pelo XSD dentro de gIBSUF: pIBSUF, gDif, gDevTrib, gRed, vIBSUF.
	ordem := []string{"<pIBSUF>", "<gDif>", "<gDevTrib>", "<gRed>", "<vIBSUF>"}
	verificarOrdem(t, s, ordem)
}

func TestIBSCBS_GALCZFMCBS_DentroDeGCBS(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "220", ClassTrib: "220001",
		GIBSCBS: &builder.GIBSCBS{
			VBC:     "1000.00",
			GIBSUF:  builder.GIBSUF{PIBSUF: "0.00", VIBSUF: "0.00"},
			GIBSMun: builder.GIBSMun{PIBSMun: "0.00", VIBSMun: "0.00"},
			VIBS:    "0.00",
			GCBS: builder.GCBS{
				PCBS: "0.00",
				GALCZFMCBS: &builder.GALCZFMCBS{
					TpALCZFMCBS: "1", NProcSuframa: "12345678",
					PAliqEfetRegCBS: "0.90", VTribRegCBS: "9.00",
				},
				VCBS: "0.00",
			},
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<gALCZFMCBS><tpALCZFMCBS>1</tpALCZFMCBS><nProcSuframa>12345678</nProcSuframa>") {
		t.Errorf("gALCZFMCBS mal formado: %s", s)
	}
}

func TestIBSCBS_GTribRegularEGTribCompraGov_AposGCBS(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "410", ClassTrib: "410001",
		GIBSCBS: &builder.GIBSCBS{
			VBC:     "1000.00",
			GIBSUF:  builder.GIBSUF{PIBSUF: "0.00", VIBSUF: "0.00"},
			GIBSMun: builder.GIBSMun{PIBSMun: "0.00", VIBSMun: "0.00"},
			VIBS:    "0.00",
			GCBS:    builder.GCBS{PCBS: "0.00", VCBS: "0.00"},
			GTribRegular: &builder.GTribRegular{
				CSTReg: "000", ClassTribReg: "000001",
				PAliqEfetRegIBSUF: "0.10", VTribRegIBSUF: "1.00",
				PAliqEfetRegIBSMun: "0.00", VTribRegIBSMun: "0.00",
				PAliqEfetRegCBS: "0.90", VTribRegCBS: "9.00",
			},
			GTribCompraGov: &builder.GTribCompraGov{
				PAliqIBSUF: "0.10", VTribIBSUF: "1.00",
				PAliqIBSMun: "0.00", VTribIBSMun: "0.00",
				PAliqCBS: "0.90", VTribCBS: "9.00",
			},
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	ordem := []string{"</gCBS>", "<gTribRegular>", "</gTribRegular>", "<gTribCompraGov>"}
	verificarOrdem(t, s, ordem)
}

func TestIBSCBS_GIBSCBSMono_CST620(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "620", ClassTrib: "620001",
		GIBSCBSMono: &builder.GIBSCBSMono{
			GMonoPadrao: &builder.GMonoPadrao{
				QBCMono: "100.0000", AdRemIBS: "0.05", AdRemCBS: "0.09",
				VIBSMono: "5.00", VCBSMono: "9.00",
			},
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "<gIBSCBS>") {
		t.Errorf("choice violado: gIBSCBS e gIBSCBSMono não podem coexistir: %s", s)
	}
	if !strings.Contains(s, "<gIBSCBSMono><gMonoPadrao>") {
		t.Errorf("gIBSCBSMono mal formado: %s", s)
	}
}

func TestIBSCBS_GTransfCred_CST800(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "800", ClassTrib: "800001",
		GTransfCred: &builder.GTransfCred{VIBS: "10.00", VCBS: "20.00"},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<gTransfCred><vIBS>10.00</vIBS><vCBS>20.00</vCBS></gTransfCred>") {
		t.Errorf("gTransfCred mal formado: %s", s)
	}
}

func TestIBSCBS_GAjusteCompet_CST811(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "811", ClassTrib: "811001",
		GAjusteCompet: &builder.GAjusteCompet{CompetApur: "2026-08", VIBS: "1.00", VCBS: "2.00"},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<gAjusteCompet><competApur>2026-08</competApur><vIBS>1.00</vIBS><vCBS>2.00</vCBS></gAjusteCompet>") {
		t.Errorf("gAjusteCompet mal formado: %s", s)
	}
}

func TestIBSCBS_GEstornoCredEGCredPres_IndependentesDoChoice(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "000", ClassTrib: "000001",
		GIBSCBS: &builder.GIBSCBS{
			VBC:     "1000.00",
			GIBSUF:  builder.GIBSUF{PIBSUF: "0.10", VIBSUF: "1.00"},
			GIBSMun: builder.GIBSMun{PIBSMun: "0.00", VIBSMun: "0.00"},
			VIBS:    "1.00",
			GCBS:    builder.GCBS{PCBS: "0.90", VCBS: "9.00"},
		},
		GEstornoCred: &builder.GEstornoCred{VIBSEstCred: "1.00", VCBSEstCred: "2.00"},
		GCredPresOper: &builder.GCredPresOper{
			VBCCredPres: "1000.00", CCredPres: "1",
			GIBSCredPres: &builder.GCredPres{PCredPres: "55.00", VCredPres: "5.50"},
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	// gEstornoCred e gCredPresOper vêm depois do choice (gIBSCBS aqui), nessa ordem.
	ordem := []string{"</gIBSCBS>", "<gEstornoCred>", "</gEstornoCred>", "<gCredPresOper>"}
	verificarOrdem(t, s, ordem)
}

func TestIBSCBS_GCredPresIBSZFM(t *testing.T) {
	ib := builder.IBSCBS{
		CST: "000", ClassTrib: "000001",
		GCredPresIBSZFM: &builder.GCredPresIBSZFM{
			CompetApur: "2026-08", TpCredPresIBSZFM: "2", VCredPresIBSZFM: "15.00",
		},
	}
	out, err := xml.Marshal(ib)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<gCredPresIBSZFM><competApur>2026-08</competApur><tpCredPresIBSZFM>2</tpCredPresIBSZFM><vCredPresIBSZFM>15.00</vCredPresIBSZFM></gCredPresIBSZFM>") {
		t.Errorf("gCredPresIBSZFM mal formado: %s", s)
	}
}

func verificarOrdem(t *testing.T, s string, elementos []string) {
	t.Helper()
	pos := -1
	for _, el := range elementos {
		i := strings.Index(s, el)
		if i == -1 {
			t.Fatalf("elemento %q ausente: %s", el, s)
		}
		if i < pos {
			t.Fatalf("elemento %q fora de ordem: %s", el, s)
		}
		pos = i
	}
}
