package sefaz_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

func TestObterURL_GOHomologacao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para GO homologação")
	}
	if !strings.Contains(url, "homolog") && !strings.Contains(url, "homologacao") {
		t.Errorf("URL de homologação não parece correta: %s", url)
	}
	t.Logf("GO homologação autorizacao: %s", url)
}

func TestObterURL_GOProducao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoAutorizacao, sefaz.Producao)
	if url == "" {
		t.Fatal("URL vazia para GO produção")
	}
	if strings.Contains(url, "homolog") {
		t.Errorf("URL de produção contém 'homolog': %s", url)
	}
	t.Logf("GO produção autorizacao: %s", url)
}

func TestObterURL_SPHomologacao(t *testing.T) {
	url := sefaz.ObterURL("35", sefaz.ServicoConsultaProtocolo, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para SP homologação")
	}
	t.Logf("SP homologação consulta: %s", url)
}

func TestObterURL_FallbackSVRS(t *testing.T) {
	// AC (12) não tem entrada própria → deve usar SVRS
	url := sefaz.ObterURL("12", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para AC (SVRS fallback)")
	}
	if !strings.Contains(url, "svrs.rs.gov.br") {
		t.Errorf("AC deveria usar SVRS, mas URL é: %s", url)
	}
	t.Logf("AC (SVRS fallback) homologação: %s", url)
}

func TestObterURL_AMUsaSVAN(t *testing.T) {
	// AM (13) usa SVAN
	url := sefaz.ObterURL("13", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para AM")
	}
	if !strings.Contains(url, "sefazvirtual.fazenda.gov.br") {
		t.Errorf("AM deveria usar SVAN, mas URL é: %s", url)
	}
	t.Logf("AM (SVAN) homologação: %s", url)
}

func TestObterURL_TodosServicosGO(t *testing.T) {
	servicos := []sefaz.Servico{
		sefaz.ServicoAutorizacao,
		sefaz.ServicoRetAutorizacao,
		sefaz.ServicoConsultaProtocolo,
		sefaz.ServicoRecepcaoEvento,
		sefaz.ServicoInutilizacao,
		sefaz.ServicoStatusServico,
	}
	for _, srv := range servicos {
		url := sefaz.ObterURL("52", srv, sefaz.Homologacao)
		if url == "" {
			t.Errorf("URL vazia para GO / %s", srv)
		}
	}
}

func TestAmbienteString(t *testing.T) {
	if sefaz.Producao.String() != "Produção" {
		t.Errorf("Producao.String() = %q", sefaz.Producao.String())
	}
	if sefaz.Homologacao.String() != "Homologação" {
		t.Errorf("Homologacao.String() = %q", sefaz.Homologacao.String())
	}
}

// ── NFeDistribuicaoDFe ────────────────────────────────────────────────────────

func TestObterURL_DistribuicaoDFe_Nacional(t *testing.T) {
	// O serviço é nacional — cUF irrelevante, URL deve ser fazenda.gov.br
	for _, cuf := range []string{"52", "35", "12", "99"} {
		url := sefaz.ObterURL(cuf, sefaz.ServicoDistribuicaoDFe, sefaz.Homologacao)
		if url == "" {
			t.Fatalf("URL vazia para DistribuicaoDFe cuf=%s homologação", cuf)
		}
		if !strings.Contains(url, "nfe.fazenda.gov.br") {
			t.Errorf("cuf=%s: URL esperada conter 'nfe.fazenda.gov.br', got: %s", cuf, url)
		}
		if !strings.Contains(url, "hom") {
			t.Errorf("cuf=%s: URL de homologação deveria conter 'hom', got: %s", cuf, url)
		}
	}
}

func TestObterURL_DistribuicaoDFe_Producao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoDistribuicaoDFe, sefaz.Producao)
	if url == "" {
		t.Fatal("URL vazia para DistribuicaoDFe produção")
	}
	if strings.Contains(url, "hom") {
		t.Errorf("URL de produção contém 'hom': %s", url)
	}
	if !strings.Contains(url, "nfe.fazenda.gov.br") {
		t.Errorf("URL de produção não contém 'nfe.fazenda.gov.br': %s", url)
	}
	t.Logf("DistribuicaoDFe produção: %s", url)
}

// ── AutorizarContingencia ─────────────────────────────────────────────────────

func entradaContingencia() builder.EntradaNFe {
	return builder.EntradaNFe{
		Serie: "1", NNF: "99",
		DhEmi:    time.Date(2026, 6, 26, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA DE MERCADORIA", TpAmb: "2", FinNFe: "1",
		IndFinal: "0", IndPres: "1",
		TpEmis: "5",
		DhCont: "2026-06-26T10:00:00-03:00",
		XJust:  "Queda de internet no estabelecimento",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "METALURGICA TESTE LTDA", IE: "123456789", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Chapas", Numero: "100", Bairro: "Industrial",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Dest: builder.EntradaDest{
			CNPJ: "99888777000155", Nome: "CLIENTE SA", IndIEDest: "1", IE: "987654321",
			End: builder.EntradaEndereco{
				Logradouro: "Av. do Aco", Numero: "500", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74100000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Itens: []builder.EntradaItem{{
			CProd: "P001", CEAN: "SEM GTIN", Nome: "PRODUTO TESTE",
			NCM: "73089090", CFOP: "5102", Unidade: "UN",
			Quantidade: 1, VUnitario: 100.00,
			ICMS: builder.EntradaICMS{CSOSN: "400"},
		}},
		Frete:     builder.EntradaFrete{Modalidade: "9"},
		Pagamento: []builder.EntradaPagamento{{Forma: "01", Valor: 100.00}},
	}
}

func certTeste(t *testing.T) *cert.Certificado {
	t.Helper()
	pfx, err := cert.GerarCertificadoTeste("11222333000181", "teste")
	if err != nil {
		t.Fatalf("GerarCertificadoTeste: %v", err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatalf("CarregarPFXBytes: %v", err)
	}
	return c
}

func TestAutorizarContingencia_Sucesso(t *testing.T) {
	assinado, err := sefaz.AutorizarContingencia(entradaContingencia(), certTeste(t))
	if err != nil {
		t.Fatalf("AutorizarContingencia: %v", err)
	}
	if len(assinado) < 1000 {
		t.Errorf("XML assinado muito pequeno: %d bytes", len(assinado))
	}
	xmlStr := string(assinado)
	if !strings.Contains(xmlStr, "<tpEmis>5</tpEmis>") {
		t.Error("XML não contém tpEmis=5")
	}
	if !strings.Contains(xmlStr, "<dhCont>") {
		t.Error("XML não contém <dhCont>")
	}
	if !strings.Contains(xmlStr, "<xJust>") {
		t.Error("XML não contém <xJust>")
	}
	if !strings.Contains(xmlStr, "<SignatureValue>") {
		t.Error("XML não contém assinatura digital")
	}
	t.Logf("AutorizarContingencia OK — %d bytes assinados", len(assinado))
}

func TestAutorizarContingencia_TpEmisErrado_Erro(t *testing.T) {
	e := entradaContingencia()
	e.TpEmis = "1" // não é contingência
	_, err := sefaz.AutorizarContingencia(e, certTeste(t))
	if err == nil {
		t.Fatal("esperava erro: tpEmis≠5")
	}
	if !strings.Contains(err.Error(), "tpEmis") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestAutorizarContingencia_SemDhCont_Erro(t *testing.T) {
	e := entradaContingencia()
	e.DhCont = ""
	_, err := sefaz.AutorizarContingencia(e, certTeste(t))
	if err == nil {
		t.Fatal("esperava erro: DhCont ausente")
	}
	if !strings.Contains(err.Error(), "DhCont") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestRetornoDistribuicao_TemMais(t *testing.T) {
	casos := []struct {
		ultNSU string
		maxNSU string
		espera bool
	}{
		{"000000000000000", "000000000000050", true},
		{"000000000000050", "000000000000050", false},
		{"000000000000100", "000000000000050", false},
		{"", "000000000000050", false},
		{"000000000000010", "", false},
	}
	for _, c := range casos {
		r := sefaz.RetornoDistribuicao{UltNSU: c.ultNSU, MaxNSU: c.maxNSU}
		if got := r.TemMais(); got != c.espera {
			t.Errorf("TemMais() ultNSU=%q maxNSU=%q: got %v, want %v",
				c.ultNSU, c.maxNSU, got, c.espera)
		}
	}
}

// ── SincronizarDFe ───────────────────────────────────────────────────────────

// mockDFeTransport intercepta chamadas HTTP e retorna uma resposta SOAP com
// UltNSU < MaxNSU (TemMais=true) e lote vazio — simula paginação infinita.
type mockDFeTransport struct {
	ultNSU string
	maxNSU string
}

func (m *mockDFeTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">`+
		`<soap12:Body>`+
		`<nfeDistDFeInteresseResult>`+
		`<retDistDFeInt versao="1.01" xmlns="http://www.portalfiscal.inf.br/nfe">`+
		`<tpAmb>2</tpAmb>`+
		`<cStat>137</cStat>`+
		`<xMotivo>Documento(s) localizado(s)</xMotivo>`+
		`<ultNSU>%s</ultNSU>`+
		`<maxNSU>%s</maxNSU>`+
		`<loteDistDFeInt/>`+
		`</retDistDFeInt>`+
		`</nfeDistDFeInteresseResult>`+
		`</soap12:Body>`+
		`</soap12:Envelope>`, m.ultNSU, m.maxNSU)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func TestSincronizarDFe_LimiteProtecao(t *testing.T) {
	mock := &mockDFeTransport{
		ultNSU: "000000000000001",
		maxNSU: "999999999999999",
	}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)
	ctx := context.Background()

	docs, err := cl.SincronizarDFe(ctx, "11222333000181", 0)
	if err == nil {
		t.Fatal("esperava erro de limite de páginas")
	}
	if !strings.Contains(err.Error(), "limite") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("SincronizarDFe parou com %d docs coletados, erro: %v", len(docs), err)
}

// ── CartaCorrecao ─────────────────────────────────────────────────────────────

func TestCartaCorrecao_XCorrecaoCurto(t *testing.T) {
	c := certTeste(t)
	chNFe := strings.Repeat("5", 44)
	_, err := sefaz.CartaCorrecao(c, chNFe, "curto", sefaz.XCondUsoCartaCorrecao, 1, sefaz.Homologacao)
	if err == nil {
		t.Fatal("esperava erro: xCorrecao < 15 chars")
	}
	if !strings.Contains(err.Error(), "mínimo 15") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestCartaCorrecao_XCorrecaoLongo(t *testing.T) {
	c := certTeste(t)
	chNFe := strings.Repeat("5", 44)
	_, err := sefaz.CartaCorrecao(c, chNFe, strings.Repeat("x", 1001), sefaz.XCondUsoCartaCorrecao, 1, sefaz.Homologacao)
	if err == nil {
		t.Fatal("esperava erro: xCorrecao > 1000 chars")
	}
	if !strings.Contains(err.Error(), "máximo 1000") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

// ── InutilizarLote ───────────────────────────────────────────────────────────

func TestInutilizarLote_ValidacaoTamanhos(t *testing.T) {
	c := certTeste(t)
	_, err := sefaz.InutilizarLote(
		c, "52", "11222333000181", "26", "1",
		[]int{1, 2},  // 2 elementos
		[]int{10},    // 1 elemento — divergente
		[]string{"justificativa valida longa o suficiente", "segunda"},
		sefaz.Homologacao,
	)
	if err == nil {
		t.Fatal("esperava erro: tamanhos divergentes")
	}
	if !strings.Contains(err.Error(), "mesmo tamanho") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
	t.Logf("Erro esperado: %v", err)
}

func TestInutilizarLote_Estrutura(t *testing.T) {
	c := certTeste(t)
	// Justificativas curtas (< 15 chars) provocam erro local — sem chamada de rede.
	// Verifica que ambas as faixas são tentadas (não aborta no primeiro erro).
	resultados, err := sefaz.InutilizarLote(
		c, "52", "11222333000181", "26", "1",
		[]int{100, 200},
		[]int{109, 209},
		[]string{"curta", "curta2"},
		sefaz.Homologacao,
	)
	if err != nil {
		t.Fatalf("InutilizarLote retornou erro fatal: %v", err)
	}
	if len(resultados) != 2 {
		t.Fatalf("esperava 2 resultados, got %d", len(resultados))
	}
	if resultados[0].NIni != 100 || resultados[0].NFin != 109 {
		t.Errorf("faixa 0: NIni=%d NFin=%d, want 100/109", resultados[0].NIni, resultados[0].NFin)
	}
	if resultados[1].NIni != 200 || resultados[1].NFin != 209 {
		t.Errorf("faixa 1: NIni=%d NFin=%d, want 200/209", resultados[1].NIni, resultados[1].NFin)
	}
	for i, r := range resultados {
		if r.Erro == nil {
			t.Errorf("resultado[%d]: esperava Erro não-nil (justificativa inválida)", i)
		}
	}
	t.Logf("InutilizarLote Estrutura OK — [%d,%d] err=%v | [%d,%d] err=%v",
		resultados[0].NIni, resultados[0].NFin, resultados[0].Erro,
		resultados[1].NIni, resultados[1].NFin, resultados[1].Erro)
}

func TestTipoDocDFeConstantes(t *testing.T) {
	if sefaz.DocProcNFe != "procNFe_v4.00.xsd" {
		t.Errorf("DocProcNFe = %q", sefaz.DocProcNFe)
	}
	if sefaz.DocResNFe != "resNFe_v1.01.xsd" {
		t.Errorf("DocResNFe = %q", sefaz.DocResNFe)
	}
}

// ── EnviarLote: sem declaração XML duplicada ─────────────────────────────────

// capturaTransport grava o corpo da requisição enviada e responde com um
// lote recebido (cStat 103), sem exigir polling de ConsultarLote.
type capturaTransport struct {
	corpoEnviado []byte
}

func (c *capturaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	c.corpoEnviado = b
	resp := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">` +
		`<soap12:Body><nfeAutorizacaoLoteResult>` +
		`<retEnviNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<tpAmb>2</tpAmb><verAplic>SVRS</verAplic><cStat>103</cStat>` +
		`<xMotivo>Lote recebido com sucesso</xMotivo>` +
		`<dhRecbto>2026-07-08T12:00:00-03:00</dhRecbto><nRec>123456789012345</nRec>` +
		`</retEnviNFe></nfeAutorizacaoLoteResult></soap12:Body></soap12:Envelope>`
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     make(http.Header),
	}, nil
}

// TestEnviarLote_SemDeclaracaoXMLDuplicada reproduz o bug em que uma NF-e
// assinada (que já carrega seu próprio <?xml ?> vindo do builder) era
// concatenada dentro do envelope SOAP sem remover essa declaração — a SEFAZ
// rejeita com "Illegal processing instruction target" por haver uma segunda
// declaração XML no meio do documento.
func TestEnviarLote_SemDeclaracaoXMLDuplicada(t *testing.T) {
	mock := &capturaTransport{}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	nfeAssinada := []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe123"></infNFe></NFe>`)

	_, err := cl.EnviarLote(context.Background(), sefaz.LoteNFe{
		IDLote: "1",
		NFes:   [][]byte{nfeAssinada},
	})
	if err != nil {
		t.Fatalf("EnviarLote retornou erro inesperado: %v", err)
	}

	if n := strings.Count(string(mock.corpoEnviado), "<?xml"); n != 1 {
		t.Errorf("esperava exatamente 1 declaração <?xml no envelope SOAP enviado, achou %d\ncorpo: %s",
			n, mock.corpoEnviado)
	}
}
