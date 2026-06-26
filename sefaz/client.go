package sefaz

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

// soapAction mapeia cada serviço para o HTTP SOAPAction header exigido pela SEFAZ.
var soapAction = map[Servico]string{
	ServicoAutorizacao:       "http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4/nfeAutorizacaoLote",
	ServicoRetAutorizacao:    "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRetAutorizacao4/nfeRetAutorizacaoLote",
	ServicoConsultaProtocolo: "http://www.portalfiscal.inf.br/nfe/wsdl/NFeConsultaProtocolo4/nfeConsultaNF",
	ServicoRecepcaoEvento:    "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4/nfeRecepcaoEvento",
	ServicoInutilizacao:      "http://www.portalfiscal.inf.br/nfe/wsdl/NFeInutilizacao4/nfeInutilizacaoNF",
	ServicoStatusServico:     "http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4/nfeStatusServicoNF",
	ServicoDistribuicaoDFe:   "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe/nfeDistDFeInteresse",
}

// Cliente é o cliente SOAP para os webservices SEFAZ.
// Inicializar via NovoCliente.
type Cliente struct {
	cuf     string
	amb     Ambiente
	http    *http.Client
}

// NovoCliente cria um Cliente autenticado com o certificado A1 fornecido.
// cuf: código IBGE da UF emitente (2 dígitos, ex: "52" para GO).
func NovoCliente(cuf string, amb Ambiente, c *cert.Certificado) (*Cliente, error) {
	tlsCfg := c.TLSConfig()
	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	return &Cliente{cuf: cuf, amb: amb, http: httpClient}, nil
}

// NovoClienteTransporte cria um Cliente com transport HTTP customizado.
// Destinado a testes — permite injetar um http.RoundTripper mock sem certificado real.
func NovoClienteTransporte(cuf string, amb Ambiente, rt http.RoundTripper) *Cliente {
	return &Cliente{
		cuf:  cuf,
		amb:  amb,
		http: &http.Client{Transport: rt, Timeout: 30 * time.Second},
	}
}

// ── Envelope SOAP ─────────────────────────────────────────────────────────────

const soapEnvelopeTPL = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` +
	`xmlns:xsd="http://www.w3.org/2001/XMLSchema" ` +
	`xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">` +
	`<soap12:Body>%s</soap12:Body>` +
	`</soap12:Envelope>`

func envelopar(body string) []byte {
	return []byte(fmt.Sprintf(soapEnvelopeTPL, body))
}

// ── Chamada genérica ──────────────────────────────────────────────────────────

// chamar envia um envelope SOAP para o serviço indicado e retorna o body da resposta.
func (cl *Cliente) chamar(ctx context.Context, srv Servico, soapBody string) ([]byte, error) {
	url := ObterURL(cl.cuf, srv, cl.amb)
	if url == "" {
		return nil, fmt.Errorf("sefaz: URL não encontrada para cuf=%s srv=%s amb=%s", cl.cuf, srv, cl.amb)
	}

	payload := envelopar(soapBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("sefaz: criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	if action, ok := soapAction[srv]; ok {
		req.Header.Set("SOAPAction", action)
	}

	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sefaz: HTTP %s: %w", srv, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sefaz: ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sefaz: HTTP %d de %s: %s", resp.StatusCode, url, truncar(body, 512))
	}

	return body, nil
}

// ── Tipos de resposta compartilhados ─────────────────────────────────────────

// RetornoSEFAZ contém o cStat e xMotivo presentes em qualquer retorno SEFAZ.
type RetornoSEFAZ struct {
	CStat   string `xml:"cStat"`
	XMotivo string `xml:"xMotivo"`
	Ambiente string `xml:"tpAmb"`
}

// extrairEnvelope extrai o conteúdo do primeiro <return> dentro do SOAP Body.
func extrairEnvelope(soapResp []byte) ([]byte, error) {
	type soapBody struct {
		InnerXML []byte `xml:",innerxml"`
	}
	type envelope struct {
		Body soapBody `xml:"Body"`
	}
	var env envelope
	if err := xml.Unmarshal(soapResp, &env); err != nil {
		return nil, fmt.Errorf("sefaz: parse envelope: %w (resposta: %s)", err, truncar(soapResp, 256))
	}
	// O conteúdo está dentro de <nfeXxxResult> ou similar; retornamos o innerXML do Body
	// para que cada função de serviço faça seu próprio unmarshal.
	return env.Body.InnerXML, nil
}

func truncar(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
