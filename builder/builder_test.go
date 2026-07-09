package builder_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)


func entradaExemplo() builder.EntradaNFe {
	return builder.EntradaNFe{
		Serie:    "1",
		NNF:      "42",
		DhEmi:    time.Date(2026, 6, 25, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA DE MERCADORIA",
		TpAmb:    "2", // homologação
		FinNFe:   "1",
		IndFinal: "0",
		IndPres:  "1",
		Emitente: builder.EntradaEmitente{
			CNPJ:     "11.222.333/0001-81",
			Nome:     "METALURGICA TESTE LTDA",
			Fantasia: "METALTESTE",
			IE:       "123456789",
			CRT:      "1", // Simples Nacional
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Chapas",
				Numero:     "100",
				Bairro:     "Industrial",
				CodigoMun:  "5208707",
				Municipio:  "Goiania",
				UF:         "GO",
				CEP:        "74000-000",
				Pais:       "1058",
				NomePais:   "Brasil",
				Fone:       "6299999999",
			},
		},
		Dest: builder.EntradaDest{
			CNPJ:      "99.888.777/0001-55",
			Nome:      "CLIENTE INDUSTRIA SA",
			IndIEDest: "1",
			IE:        "987654321",
			End: builder.EntradaEndereco{
				Logradouro: "Av. do Aco",
				Numero:     "500",
				Bairro:     "Centro",
				CodigoMun:  "5208707",
				Municipio:  "Goiania",
				UF:         "GO",
				CEP:        "74100-000",
				Pais:       "1058",
				NomePais:   "Brasil",
			},
		},
		Itens: []builder.EntradaItem{
			{
				CProd:      "CHAPA-001",
				CEAN:       "SEM GTIN",
				Nome:       "CHAPA DE ACO INOX 304 2MM",
				NCM:        "72193300",
				CFOP:       "5102",
				Unidade:    "KG",
				Quantidade: 100,
				VUnitario:  25.50,
				ICMS:       builder.EntradaICMS{CSOSN: "400"},
			},
			{
				CProd:      "PERF-002",
				CEAN:       "SEM GTIN",
				Nome:       "PERFIL ESTRUTURAL L 2X2",
				NCM:        "72162100",
				CFOP:       "5102",
				Unidade:    "UN",
				Quantidade: 50,
				VUnitario:  18.00,
				VDesconto:  50.00,
				ICMS:       builder.EntradaICMS{CSOSN: "400"},
			},
		},
		Frete: builder.EntradaFrete{
			Modalidade: "1", // FOB
		},
		Pagamento: []builder.EntradaPagamento{
			{Forma: "15", Valor: 3400.00, APrazo: true}, // boleto
		},
		InfCpl: "Pedido interno: #42",
	}
}

func TestBuildGeraXMLValido(t *testing.T) {
	entrada := entradaExemplo()
	xmlBytes, chave, err := builder.Build(entrada)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Chave deve ter 44 dígitos
	if len(chave.String()) != 44 {
		t.Errorf("chave tem %d dígitos, esperava 44", len(chave.String()))
	}

	// XML deve ser parseável
	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	// Verificações básicas
	if nfe.InfNFe.Ide.NatOp != "VENDA DE MERCADORIA" {
		t.Errorf("NatOp: %q", nfe.InfNFe.Ide.NatOp)
	}
	if nfe.InfNFe.Emit.CNPJ != "11222333000181" {
		t.Errorf("CNPJ emitente: %q", nfe.InfNFe.Emit.CNPJ)
	}
	if len(nfe.InfNFe.Det) != 2 {
		t.Errorf("esperava 2 itens, got %d", len(nfe.InfNFe.Det))
	}
	if nfe.InfNFe.Det[0].Prod.XProd != "CHAPA DE ACO INOX 304 2MM" {
		t.Errorf("produto 1: %q", nfe.InfNFe.Det[0].Prod.XProd)
	}
	if nfe.InfNFe.Emit.CRT != "1" {
		t.Errorf("CRT: %q", nfe.InfNFe.Emit.CRT)
	}
	// Simples Nacional: deve ter ICMSSN102, não ICMS00
	if nfe.InfNFe.Det[0].Imposto.ICMS == nil {
		t.Fatal("ICMS nil no item 1")
	}
	if nfe.InfNFe.Det[0].Imposto.ICMS.ICMSSN102 == nil {
		t.Error("esperava ICMSSN102 para CRT=1")
	}

	t.Logf("Chave: %s", chave.String())
	t.Logf("XML (%d bytes):\n%s", len(xmlBytes), xmlBytes)
}

func TestChaveAcesso44Digitos(t *testing.T) {
	for i := 0; i < 10; i++ {
		chave := builder.NovaChave("GO", "11222333000181", "1", "42", "1", "55", time.Now())
		if len(chave.String()) != 44 {
			t.Errorf("iter %d: chave tem %d dígitos", i, len(chave.String()))
		}
	}
}

func TestDVChave(t *testing.T) {
	// Chave conhecida do portal SEFAZ (exemplo da documentação)
	// cUF=52(GO) + AAMM + CNPJ + mod + serie + nNF + tpEmis + cNF + cDV
	chave := builder.NovaChave("GO", "11222333000181", "001", "000000042", "1", "55", time.Now())
	dv := chave.CDV
	if dv < "0" || dv > "9" {
		t.Errorf("CDV inválido: %q", dv)
	}
}

func TestXMLContemNamespace(t *testing.T) {
	entrada := entradaExemplo()
	xmlBytes, _, err := builder.Build(entrada)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xmlBytes), "portalfiscal.inf.br/nfe") {
		t.Error("XML sem namespace SEFAZ")
	}
	if !strings.Contains(string(xmlBytes), "versao=\"4.00\"") {
		t.Error("XML sem versão 4.00")
	}
}

func TestEntradaSemItensFalha(t *testing.T) {
	e := entradaExemplo()
	e.Itens = nil
	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro com itens vazios")
	}
}

func TestCNPJInvalidoFalha(t *testing.T) {
	e := entradaExemplo()
	e.Emitente.CNPJ = "123"
	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro com CNPJ inválido")
	}
}

// ── CRT=3 (Regime Normal) ─────────────────────────────────────────────────────

func entradaCRT3() builder.EntradaNFe {
	base := entradaExemplo()
	base.Emitente.CRT = "3"
	return base
}

func TestCRT3_ICMS00(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P001", CEAN: "SEM GTIN", Nome: "PRODUTO TESTE",
		NCM: "73089090", CFOP: "5102", Unidade: "UN",
		Quantidade: 10, VUnitario: 100.00,
		ICMS: builder.EntradaICMS{CST: "00", Aliq: 12.0},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT=3 ICMS00: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	icms := nfe.InfNFe.Det[0].Imposto.ICMS
	if icms.ICMS00 == nil {
		t.Fatal("esperava ICMS00 para CST=00 CRT=3")
	}
	if icms.ICMSSN102 != nil {
		t.Error("não deveria ter ICMSSN102 para CRT=3")
	}
	if icms.ICMS00.PICMS != "12.00" {
		t.Errorf("alíquota ICMS: %q (esperava 12.00)", icms.ICMS00.PICMS)
	}
	// PIS/COFINS deve ser PISAliq (não NT) para CRT=3
	if nfe.InfNFe.Det[0].Imposto.PIS.PISAliq == nil {
		t.Error("esperava PISAliq para CRT=3")
	}
	if nfe.InfNFe.Det[0].Imposto.COFINS.COFINSAliq == nil {
		t.Error("esperava COFINSAliq para CRT=3")
	}
	t.Logf("ICMS00 OK — VBC=%s VICMS=%s", icms.ICMS00.VBC, icms.ICMS00.VICMS)
}

func TestCRT3_ICMS40_Isento(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P002", CEAN: "SEM GTIN", Nome: "PRODUTO ISENTO",
		NCM: "73089090", CFOP: "5102", Unidade: "UN",
		Quantidade: 5, VUnitario: 200.00,
		ICMS: builder.EntradaICMS{CST: "40"},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT=3 ICMS40: %v", err)
	}

	var nfe builder.NFe
	xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe)

	icms := nfe.InfNFe.Det[0].Imposto.ICMS
	if icms.ICMS40 == nil {
		t.Fatal("esperava ICMS40 para CST=40")
	}
	if icms.ICMS40.CST != "40" {
		t.Errorf("CST: %q (esperava 40)", icms.ICMS40.CST)
	}
}

func TestCRT3_ICMS20_ComReducao(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P003", CEAN: "SEM GTIN", Nome: "PRODUTO COM REDUCAO",
		NCM: "73089090", CFOP: "5102", Unidade: "UN",
		Quantidade: 10, VUnitario: 500.00,
		ICMS: builder.EntradaICMS{CST: "20", PRedBC: 30.0, Aliq: 17.0},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT=3 ICMS20: %v", err)
	}

	var nfe builder.NFe
	xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe)

	icms := nfe.InfNFe.Det[0].Imposto.ICMS
	if icms.ICMS20 == nil {
		t.Fatal("esperava ICMS20 para CST=20")
	}
	// BC = 5000 * (1 - 0.30) = 3500
	if icms.ICMS20.VBC != "3500.00" {
		t.Errorf("VBC com redução: %q (esperava 3500.00)", icms.ICMS20.VBC)
	}
	// ICMS = 3500 * 17% = 595
	if icms.ICMS20.VICMS != "595.00" {
		t.Errorf("VICMS: %q (esperava 595.00)", icms.ICMS20.VICMS)
	}
}

func TestCRT3_ComIPI(t *testing.T) {
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P004", CEAN: "SEM GTIN", Nome: "PRODUTO COM IPI",
		NCM: "73089090", CFOP: "5101", Unidade: "UN",
		Quantidade: 10, VUnitario: 100.00,
		ICMS: builder.EntradaICMS{CST: "00", Aliq: 12.0},
		IPI:  &builder.EntradaIPI{CEnq: "999", CST: "50", Aliq: 5.0},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT=3 com IPI: %v", err)
	}

	var nfe builder.NFe
	xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe)

	imp := nfe.InfNFe.Det[0].Imposto
	if imp.IPI == nil {
		t.Fatal("esperava IPI no item")
	}
	if imp.IPI.IPITrib == nil {
		t.Fatal("esperava IPITrib")
	}
	// IPI = 10 * 100 * 5% = 50
	if imp.IPI.IPITrib.VIPI != "50.00" {
		t.Errorf("VIPI: %q (esperava 50.00)", imp.IPI.IPITrib.VIPI)
	}
	t.Logf("IPI OK — VIPI=%s PIPI=%s", imp.IPI.IPITrib.VIPI, imp.IPI.IPITrib.PIPI)
}

func TestCRT3_CST10_ST(t *testing.T) {
	// 10 un × R$200 = R$2.000 | aliq=12% | MVA=40% | aliqST=18%
	// vBC=2000  vICMS=240
	// vBCST = 2000*(1+0.40)=2800  vICMSST = 2800*18%-240 = 504-240 = 264
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P010", CEAN: "SEM GTIN", Nome: "PRODUTO COM ST CST10",
		NCM: "73089090", CFOP: "6401", Unidade: "UN",
		Quantidade: 10, VUnitario: 200.00,
		ICMS: builder.EntradaICMS{
			CST: "10", Aliq: 12.0,
			PMVAST: 40.0, AliqST: 18.0,
		},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT3 CST10: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	icms := nfe.InfNFe.Det[0].Imposto.ICMS
	if icms.ICMS10 == nil {
		t.Fatal("esperava ICMS10 para CST=10")
	}
	if icms.ICMS40 != nil || icms.ICMS00 != nil {
		t.Error("não deveria ter ICMS40/ICMS00 quando CST=10")
	}

	i10 := icms.ICMS10
	if i10.CST != "10" {
		t.Errorf("CST: %q, esperava \"10\"", i10.CST)
	}
	if i10.VBC != "2000.00" {
		t.Errorf("vBC: %q, esperava \"2000.00\"", i10.VBC)
	}
	if i10.PICMS != "12.00" {
		t.Errorf("pICMS: %q, esperava \"12.00\"", i10.PICMS)
	}
	if i10.VICMS != "240.00" {
		t.Errorf("vICMS: %q, esperava \"240.00\"", i10.VICMS)
	}
	if i10.VBCST != "2800.00" {
		t.Errorf("vBCST: %q, esperava \"2800.00\"", i10.VBCST)
	}
	if i10.PICMSST != "18.00" {
		t.Errorf("pICMSST: %q, esperava \"18.00\"", i10.PICMSST)
	}
	if i10.VICMSST != "264.00" {
		t.Errorf("vICMSST: %q, esperava \"264.00\"", i10.VICMSST)
	}

	// XML deve conter as tags ST literalmente
	xmlStr := string(xmlBytes)
	for _, tag := range []string{"<vBCST>", "<pICMSST>", "<vICMSST>"} {
		if !strings.Contains(xmlStr, tag) {
			t.Errorf("XML não contém tag %s", tag)
		}
	}
	t.Logf("ICMS10 OK — vBC=%s vICMS=%s vBCST=%s vICMSST=%s",
		i10.VBC, i10.VICMS, i10.VBCST, i10.VICMSST)
}

func TestCRT3_CST60_STRetido(t *testing.T) {
	// ST já retido na operação anterior.
	// Caller informa: vBCSTRet=700 | pSTRet=18% | vICMSSTRet=126
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P060", CEAN: "SEM GTIN", Nome: "PRODUTO ST RETIDO CST60",
		NCM: "73089090", CFOP: "5405", Unidade: "UN",
		Quantidade: 5, VUnitario: 100.00,
		ICMS: builder.EntradaICMS{
			CST:        "60",
			VBCSTRet:   700.00,
			PSTRet:     18.0,
			VICMSSTRet: 126.00,
		},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build CRT3 CST60: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	icms := nfe.InfNFe.Det[0].Imposto.ICMS
	if icms.ICMS60 == nil {
		t.Fatal("esperava ICMS60 para CST=60")
	}
	if icms.ICMS40 != nil {
		t.Error("não deveria ter ICMS40 quando CST=60")
	}

	i60 := icms.ICMS60
	if i60.CST != "60" {
		t.Errorf("CST: %q, esperava \"60\"", i60.CST)
	}
	if i60.VBCSTRet != "700.00" {
		t.Errorf("vBCSTRet: %q, esperava \"700.00\"", i60.VBCSTRet)
	}
	if i60.PSTRet != "18.00" {
		t.Errorf("pSTRet: %q, esperava \"18.00\"", i60.PSTRet)
	}
	if i60.VICMSSTRet != "126.00" {
		t.Errorf("vICMSSTRet: %q, esperava \"126.00\"", i60.VICMSSTRet)
	}

	// XML deve conter as tags de ST retido
	xmlStr := string(xmlBytes)
	for _, tag := range []string{"<vBCSTRet>", "<vICMSSTRet>"} {
		if !strings.Contains(xmlStr, tag) {
			t.Errorf("XML não contém tag %s", tag)
		}
	}
	t.Logf("ICMS60 OK — vBCSTRet=%s pSTRet=%s vICMSSTRet=%s",
		i60.VBCSTRet, i60.PSTRet, i60.VICMSSTRet)
}

func TestCRT3_CST_Desconhecido_FallbackICMS40(t *testing.T) {
	// CST não mapeado deve cair no default → ICMS40/CST=40
	e := entradaCRT3()
	e.Itens = []builder.EntradaItem{{
		CProd: "P005", CEAN: "SEM GTIN", Nome: "PRODUTO CST DESCONHECIDO",
		NCM: "73089090", CFOP: "5102", Unidade: "UN",
		Quantidade: 1, VUnitario: 100.00,
		ICMS: builder.EntradaICMS{CST: "99"},
	}}
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var nfe builder.NFe
	xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe)

	if nfe.InfNFe.Det[0].Imposto.ICMS.ICMS40 == nil {
		t.Error("CST desconhecido deveria cair no ICMS40 (default)")
	}
}

func TestDestComCPF(t *testing.T) {
	e := entradaExemplo()
	e.Dest.CNPJ = ""
	e.Dest.CPF = "12345678901"
	e.Dest.IndIEDest = "9"
	e.Dest.IE = ""

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build com CPF: %v", err)
	}
	if !strings.Contains(string(xmlBytes), "<CPF>12345678901</CPF>") {
		t.Error("XML não contém CPF do destinatário")
	}
	if strings.Contains(string(xmlBytes), "<CNPJ></CNPJ>") {
		t.Error("XML contém CNPJ vazio quando deveria usar CPF")
	}
}

func TestPagamentoSemPagamentos_DefaultSemPagto(t *testing.T) {
	e := entradaExemplo()
	e.Pagamento = nil // sem pagamentos → deve gerar detPag tPag=90
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build sem pagamento: %v", err)
	}
	if !strings.Contains(string(xmlBytes), "<tPag>90</tPag>") {
		t.Error("sem pagamentos, esperava tPag=90 (sem pagamento)")
	}
}

// ── Contingência FS-DA (tpEmis=5) ────────────────────────────────────────────

func TestContingencia_FSDA_XMLValido(t *testing.T) {
	e := entradaExemplo()
	e.TpEmis = "5"
	e.DhCont = "2026-06-26T10:00:00-03:00"
	e.XJust = "Queda de internet no estabelecimento"

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build contingência: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	ide := nfe.InfNFe.Ide
	if ide.TpEmis != "5" {
		t.Errorf("tpEmis: %q, esperava \"5\"", ide.TpEmis)
	}
	if ide.DhCont != e.DhCont {
		t.Errorf("dhCont: %q, esperava %q", ide.DhCont, e.DhCont)
	}
	if ide.XJust != e.XJust {
		t.Errorf("xJust: %q, esperava %q", ide.XJust, e.XJust)
	}
	xmlStr := string(xmlBytes)
	for _, tag := range []string{"<dhCont>", "<xJust>"} {
		if !strings.Contains(xmlStr, tag) {
			t.Errorf("XML não contém %s", tag)
		}
	}
	t.Logf("FS-DA OK — tpEmis=%s dhCont=%s", ide.TpEmis, ide.DhCont)
}

func TestContingencia_FSDA_SemDhCont_Erro(t *testing.T) {
	e := entradaExemplo()
	e.TpEmis = "5"
	e.XJust = "Justificativa com mais de quinze chars"
	// DhCont ausente → erro
	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro: tpEmis=5 sem DhCont")
	}
	if !strings.Contains(err.Error(), "DhCont") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestContingencia_FSDA_XJustCurta_Erro(t *testing.T) {
	e := entradaExemplo()
	e.TpEmis = "5"
	e.DhCont = "2026-06-26T10:00:00-03:00"
	e.XJust = "curta" // < 15 chars → erro
	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro: xJust com menos de 15 chars")
	}
	if !strings.Contains(err.Error(), "XJust") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

// ── FinNFe com referência ─────────────────────────────────────────────────────

// chave de 44 dígitos fictícia usada nos testes de referência
const chaveRefTeste = "52260611222333000181550010000000421965432101"

func TestFinNFe_Devolucao_ComRef(t *testing.T) {
	e := entradaExemplo()
	e.FinNFe = "4"
	e.ChaveNFeRef = chaveRefTeste

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build devolução com ref: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	if nfe.InfNFe.Ide.FinNFe != "4" {
		t.Errorf("finNFe: %q, esperava \"4\"", nfe.InfNFe.Ide.FinNFe)
	}
	if len(nfe.InfNFe.Ide.NFref) != 1 {
		t.Fatalf("esperava 1 NFref, got %d", len(nfe.InfNFe.Ide.NFref))
	}
	if nfe.InfNFe.Ide.NFref[0].RefNFe != chaveRefTeste {
		t.Errorf("refNFe: %q, esperava %q", nfe.InfNFe.Ide.NFref[0].RefNFe, chaveRefTeste)
	}
	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, "<NFref>") {
		t.Error("XML não contém <NFref>")
	}
	if !strings.Contains(xmlStr, "<refNFe>"+chaveRefTeste+"</refNFe>") {
		t.Error("XML não contém <refNFe> preenchida")
	}
	t.Logf("Devolução com ref OK — finNFe=%s refNFe=%s",
		nfe.InfNFe.Ide.FinNFe, nfe.InfNFe.Ide.NFref[0].RefNFe)
}

func TestFinNFe_Complementar_ComRef(t *testing.T) {
	e := entradaExemplo()
	e.FinNFe = "2"
	e.ChaveNFeRef = chaveRefTeste

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build complementar com ref: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	if nfe.InfNFe.Ide.FinNFe != "2" {
		t.Errorf("finNFe: %q, esperava \"2\"", nfe.InfNFe.Ide.FinNFe)
	}
	if len(nfe.InfNFe.Ide.NFref) != 1 {
		t.Fatalf("esperava 1 NFref, got %d", len(nfe.InfNFe.Ide.NFref))
	}
	if nfe.InfNFe.Ide.NFref[0].RefNFe != chaveRefTeste {
		t.Errorf("refNFe: %q", nfe.InfNFe.Ide.NFref[0].RefNFe)
	}
	if !strings.Contains(string(xmlBytes), "<NFref>") {
		t.Error("XML não contém <NFref>")
	}
	t.Logf("Complementar com ref OK — finNFe=%s refNFe=%s",
		nfe.InfNFe.Ide.FinNFe, nfe.InfNFe.Ide.NFref[0].RefNFe)
}

func TestFinNFe_Devolucao_SemRef_Erro(t *testing.T) {
	e := entradaExemplo()
	e.FinNFe = "4"
	e.ChaveNFeRef = "" // ausente — deve retornar erro

	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro: devolução sem ChaveNFeRef deve falhar")
	}
	if !strings.Contains(err.Error(), "ChaveNFeRef") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

// ── NFC-e (mod=65) ───────────────────────────────────────────────────────────

func entradaNFCe() builder.EntradaNFe {
	return builder.EntradaNFe{
		Serie: "1", NNF: "1",
		DhEmi:           time.Date(2026, 6, 26, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:           "VENDA A CONSUMIDOR", TpAmb: "2",
		FinNFe:          "1", IndFinal: "1", IndPres: "1",
		Mod:             "65",
		CSC:             "CE154B7B6FB48B77",
		CSCId:           "000001",
		UrlConsultaNFCe: "https://www.sefaz.go.gov.br/nfeweb/sites/nfce/danfeNFCe.aspx",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "LOJA TESTE LTDA", IE: "123456789", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua do Comercio", Numero: "10", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Itens: []builder.EntradaItem{{
			CProd: "P001", CEAN: "SEM GTIN", Nome: "PRODUTO VENDA BALCAO",
			NCM: "73089090", CFOP: "5102", Unidade: "UN",
			Quantidade: 2, VUnitario: 25.00,
			ICMS: builder.EntradaICMS{CSOSN: "400"},
		}},
		Frete:     builder.EntradaFrete{Modalidade: "9"},
		Pagamento: []builder.EntradaPagamento{{Forma: "01", Valor: 50.00}},
	}
}

func TestNFCe_Builder_Basico(t *testing.T) {
	// NFC-e sem destinatário identificado (consumidor anônimo no balcão)
	e := entradaNFCe()

	xmlBytes, chave, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build NFC-e: %v", err)
	}

	// chave deve ter 44 dígitos com mod=65 na posição 20-21
	chaveStr := chave.String()
	if len(chaveStr) != 44 {
		t.Errorf("chave NFC-e tem %d dígitos (esperava 44)", len(chaveStr))
	}
	if chaveStr[20:22] != "65" {
		t.Errorf("mod na chave: %q (esperava \"65\")", chaveStr[20:22])
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	ide := nfe.InfNFe.Ide
	if ide.Mod != "65" {
		t.Errorf("mod no XML: %q (esperava \"65\")", ide.Mod)
	}
	if ide.TpImp != "4" {
		t.Errorf("tpImp no XML: %q (esperava \"4\" para NFC-e)", ide.TpImp)
	}
	if nfe.InfNFe.Dest != nil {
		t.Error("dest deve ser nil para NFC-e sem destinatário identificado")
	}

	xmlStr := string(xmlBytes)
	if strings.Contains(xmlStr, "<dest>") {
		t.Error("XML não deveria conter <dest> para NFC-e sem destinatário")
	}
	if !strings.Contains(xmlStr, "<mod>65</mod>") {
		t.Error("XML não contém <mod>65</mod>")
	}
	if !strings.Contains(xmlStr, "<tpImp>4</tpImp>") {
		t.Error("XML não contém <tpImp>4</tpImp>")
	}
	t.Logf("NFC-e OK — chave=%s mod=%s tpImp=%s", chaveStr, ide.Mod, ide.TpImp)
}

func TestNFCe_SemCSC_Erro(t *testing.T) {
	e := entradaNFCe()
	e.CSC = "" // ausente → deve falhar
	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro: NFC-e sem CSC")
	}
	if !strings.Contains(err.Error(), "CSC") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestNFCe_QRCode_Gerado(t *testing.T) {
	e := entradaNFCe()

	xmlBytes, chave, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build NFC-e QRCode: %v", err)
	}

	var nfe builder.NFe
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}

	if nfe.InfNFeSupl == nil {
		t.Fatal("InfNFeSupl nil — esperava QR Code para NFC-e")
	}
	if nfe.InfNFeSupl.QrCode == "" {
		t.Error("qrCode vazio")
	}
	if !strings.Contains(nfe.InfNFeSupl.UrlChave, chave.String()) {
		t.Errorf("urlChave %q não contém a chave de acesso %q",
			nfe.InfNFeSupl.UrlChave, chave.String())
	}

	xmlStr := string(xmlBytes)
	for _, tag := range []string{"<infNFeSupl>", "<qrCode>", "<urlChave>"} {
		if !strings.Contains(xmlStr, tag) {
			t.Errorf("XML não contém %s", tag)
		}
	}
	t.Logf("NFC-e QR Code OK\n  qrCode=%s\n  urlChave=%s",
		nfe.InfNFeSupl.QrCode, nfe.InfNFeSupl.UrlChave)
}

func TestNFCe_Builder_CNPJDestRejeitado(t *testing.T) {
	// NFC-e com CNPJ no destinatário deve falhar
	e := builder.EntradaNFe{
		Serie: "1", NNF: "2",
		DhEmi:    time.Date(2026, 6, 26, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA A CONSUMIDOR", TpAmb: "2",
		FinNFe:   "1", IndFinal: "1", IndPres: "1",
		Mod: "65",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "LOJA TESTE LTDA", IE: "123456789", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua do Comercio", Numero: "10", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Dest: builder.EntradaDest{CNPJ: "99888777000155", Nome: "EMPRESA CNPJ SA"},
		Itens: []builder.EntradaItem{{
			CProd: "P001", CEAN: "SEM GTIN", Nome: "PRODUTO",
			NCM: "73089090", CFOP: "5102", Unidade: "UN",
			Quantidade: 1, VUnitario: 10.00,
			ICMS: builder.EntradaICMS{CSOSN: "400"},
		}},
		Frete:     builder.EntradaFrete{Modalidade: "9"},
		Pagamento: []builder.EntradaPagamento{{Forma: "01", Valor: 10.00}},
	}

	_, _, err := builder.Build(e)
	if err == nil {
		t.Fatal("esperava erro: NFC-e com CNPJ de destinatário deve ser rejeitado")
	}
	if !strings.Contains(err.Error(), "NFC-e") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

// TestSerieNNF_SemZeroEsquerda reproduz o bug em que <serie> saía com o
// zero-padding de 3 dígitos usado internamente na chave de acesso (ex: "001"),
// violando o schema da SEFAZ (pattern "0|[1-9][0-9]{0,2}", sem zero à
// esquerda) — confirmado com xmllint contra o XSD oficial NF-e 4.00, cStat
// 225 "Falha no Schema XML". <nNF> já tinha o tratamento; <serie> não.
func TestSerieNNF_SemZeroEsquerda(t *testing.T) {
	e := entradaExemplo() // Serie: "1", NNF: "42"
	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var nfe struct {
		InfNFe struct {
			Ide struct {
				Serie string `xml:"serie"`
				NNF   string `xml:"nNF"`
			} `xml:"ide"`
		} `xml:"infNFe"`
	}
	if err := xml.Unmarshal(xmlBytes[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if nfe.InfNFe.Ide.Serie != "1" {
		t.Errorf("serie = %q, want %q (sem zero à esquerda)", nfe.InfNFe.Ide.Serie, "1")
	}
	if nfe.InfNFe.Ide.NNF != "42" {
		t.Errorf("nNF = %q, want %q (sem zero à esquerda)", nfe.InfNFe.Ide.NNF, "42")
	}
}

func TestChaveAcesso_DoisBuildsMesmaEntrada_ChavesDiferentes(t *testing.T) {
	// cNF é gerado com crypto/rand — dois builds da mesma entrada devem
	// produzir chaves diferentes (evitar colisão por previsibilidade)
	e := entradaExemplo()
	_, c1, _ := builder.Build(e)
	_, c2, _ := builder.Build(e)
	if c1.String() == c2.String() {
		t.Error("duas builds da mesma entrada geraram a mesma chave — cNF não é aleatório")
	}
}
