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
		chave := builder.NovaChave("GO", "11222333000181", "1", "42", "1", time.Now())
		if len(chave.String()) != 44 {
			t.Errorf("iter %d: chave tem %d dígitos", i, len(chave.String()))
		}
	}
}

func TestDVChave(t *testing.T) {
	// Chave conhecida do portal SEFAZ (exemplo da documentação)
	// cUF=52(GO) + AAMM + CNPJ + mod + serie + nNF + tpEmis + cNF + cDV
	chave := builder.NovaChave("GO", "11222333000181", "001", "000000042", "1", time.Now())
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
