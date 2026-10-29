package sefaz

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Consulta ao Cadastro Centralizado de GTIN (CCG), NT 2022.001 e MOC
// "Web Service - Consulta GTIN". Serviço único no SVRS, só produção, síncrono,
// com TLS mútuo: o certificado tem de ser de um emitente de NF-e/NFC-e.
//
// Não confirmado por WSDL: o namespace do nfeDadosMsg e a SOAPAction abaixo
// seguem o padrão dos demais serviços (portalfiscal.inf.br/nfe/wsdl/<serviço>/
// <método>). O ?wsdl do SVRS devolve 403 sem certificado, então conferir no
// primeiro uso real. Se vier HTTP 500/fault, é o primeiro suspeito.
const (
	nsWSDLConsultaGTIN     = "http://www.portalfiscal.inf.br/nfe/wsdl/ccgConsGtin"
	soapActionConsultaGTIN = nsWSDLConsultaGTIN + "/ccgConsGTIN"
	versaoConsGTIN         = "1.00"
)

// urlConsultaGTIN é variável só para os testes apontarem para um httptest.
var urlConsultaGTIN = "https://dfe-servico.svrs.rs.gov.br/ws/ccgConsGTIN/ccgConsGTIN.asmx"

// CStatGTINSucesso é o cStat de consulta realizada com sucesso.
const CStatGTINSucesso = "9490"

// Erros da consulta GTIN. Use errors.Is no *ErroGTIN devolvido.
var (
	ErrGTINDVInvalido        = errors.New("GTIN com dígito verificador inválido")             // 9491
	ErrGTINPrefixo           = errors.New("GTIN não possui prefixo 789 ou 790 (Brasil)")      // 9492
	ErrGTINCertNaoEmitente   = errors.New("certificado não é de emitente de NF-e ou NFC-e")   // 9493
	ErrGTINInexistente       = errors.New("GTIN inexistente no CCG")                          // 9494
	ErrGTINSituacaoInvalida  = errors.New("GTIN com situação inválida no CCG")                // 9495
	ErrGTINNaoAutorizado     = errors.New("dono da marca não autorizou publicar o GTIN")      // 9496
	ErrGTINNCMNaoCadastrado  = errors.New("GTIN sem NCM cadastrado no CCG")                   // 9497
	ErrGTINNCMInvalido       = errors.New("GTIN com NCM inválido ou fora de vigência no CCG") // 9498
	ErrGTINConsumoIndevido   = errors.New("consumo indevido do serviço")                      // 656
	ErrGTINCertificado       = errors.New("certificado de transmissão rejeitado")             // 280-286
	ErrGTINServicoParalisado = errors.New("serviço paralisado")                               // 108, 109
	ErrGTINRejeicao          = errors.New("rejeição não mapeada")                             // demais
)

var errPorCStatGTIN = map[string]error{
	"9491": ErrGTINDVInvalido,
	"9492": ErrGTINPrefixo,
	"9493": ErrGTINCertNaoEmitente,
	"9494": ErrGTINInexistente,
	"9495": ErrGTINSituacaoInvalida,
	"9496": ErrGTINNaoAutorizado,
	"9497": ErrGTINNCMNaoCadastrado,
	"9498": ErrGTINNCMInvalido,
	"656":  ErrGTINConsumoIndevido,
	"280":  ErrGTINCertificado,
	"281":  ErrGTINCertificado,
	"282":  ErrGTINCertificado,
	"283":  ErrGTINCertificado,
	"284":  ErrGTINCertificado,
	"285":  ErrGTINCertificado,
	"286":  ErrGTINCertificado,
	"108":  ErrGTINServicoParalisado,
	"109":  ErrGTINServicoParalisado,
}

// ErroGTIN é a rejeição da consulta, local (9491/9492 antes de chamar) ou da
// SEFAZ. errors.Is(err, ErrGTINInexistente) etc. funciona via Unwrap.
type ErroGTIN struct {
	CStat   string
	XMotivo string
}

func (e *ErroGTIN) Error() string {
	return fmt.Sprintf("sefaz: consulta GTIN: cStat=%s %s", e.CStat, e.XMotivo)
}

func (e *ErroGTIN) Unwrap() error {
	if err, ok := errPorCStatGTIN[e.CStat]; ok {
		return err
	}
	return ErrGTINRejeicao
}

// RetornoGTIN é o retConsGTIN. Em rejeição só CStat, XMotivo e DhResp vêm
// preenchidos (o MOC manda não devolver os dados do CCG).
type RetornoGTIN struct {
	VerAplic string
	CStat    string
	XMotivo  string
	DhResp   string
	GTIN     string
	TpGTIN   string
	XProd    string
	NCM      string
	CEST     []string // 0 a 3 ocorrências
}

// NormalizarGTIN tira o que não é dígito e valida tamanho (8, 12, 13, 14), DV
// e prefixo GS1 Brasil (789/790), as regras P10-10 e P10-20 do MOC. Devolve os
// dígitos como vieram (sem zeros à esquerda), que é o que vai no consGTIN.
func NormalizarGTIN(gtin string) (string, error) {
	var b strings.Builder
	for _, r := range gtin {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	g := b.String()
	switch len(g) {
	case 8, 12, 13, 14:
	default:
		return "", &ErroGTIN{CStat: "9491", XMotivo: fmt.Sprintf("GTIN %q com tamanho inválido (validação local)", gtin)}
	}
	if len(g) == 14 && g[0] == '0' {
		return "", &ErroGTIN{CStat: "9491", XMotivo: "GTIN-14 não pode começar com zero (validação local)"}
	}
	n14 := strings.Repeat("0", 14-len(g)) + g
	if !dvGTINValido(n14) {
		return "", &ErroGTIN{CStat: "9491", XMotivo: "GTIN com dígito verificador inválido (validação local)"}
	}
	prefixo := n14[1:4]
	if n14[:6] == "000000" {
		prefixo = n14[6:9]
	}
	if prefixo != "789" && prefixo != "790" {
		return "", &ErroGTIN{CStat: "9492", XMotivo: "GTIN não possui prefixo 789 ou 790 (validação local)"}
	}
	return g, nil
}

// dvGTINValido aplica o módulo 10 GS1 sobre os 14 dígitos: pesos 3 e 1
// alternados da direita para a esquerda, a partir do dígito antes do DV.
func dvGTINValido(n14 string) bool {
	soma := 0
	for i := 0; i < 13; i++ {
		d := int(n14[i] - '0')
		if i%2 == 0 {
			d *= 3
		}
		soma += d
	}
	return (10-soma%10)%10 == int(n14[13]-'0')
}

// ConsultarGTIN consulta o GTIN no CCG (serviço ccgConsGTIN do SVRS). tlsConfig
// é o de cert.Certificado.TLSConfig() de um emitente. Valida DV e prefixo antes
// de chamar. Rejeição volta como *ErroGTIN junto com o RetornoGTIN lido.
//
// Não tem limite de ritmo aqui: consulta em rajada leva 656 (consumo
// indevido), e quem chama é que deve espaçar.
func ConsultarGTIN(ctx context.Context, tlsConfig *tls.Config, gtin string) (RetornoGTIN, error) {
	g, err := NormalizarGTIN(gtin)
	if err != nil {
		return RetornoGTIN{}, err
	}

	body := `<nfeDadosMsg xmlns="` + nsWSDLConsultaGTIN + `">` +
		`<consGTIN versao="` + versaoConsGTIN + `" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<GTIN>` + g + `</GTIN>` +
		`</consGTIN>` +
		`</nfeDadosMsg>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlConsultaGTIN, bytes.NewReader(envelopar(body)))
	if err != nil {
		return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: criar request: %w", err)
	}
	// Mesmo esquema do Cliente.chamar: action no Content-Type (SOAP 1.2) e no header.
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+soapActionConsultaGTIN+`"`)
	req.Header.Set("SOAPAction", soapActionConsultaGTIN)

	httpCl := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Timeout:   30 * time.Second,
	}
	resp, err := httpCl.Do(req)
	if err != nil {
		return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: HTTP: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: ler resposta: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: HTTP %d: %s", resp.StatusCode, truncar(respBody, 512))
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return RetornoGTIN{}, err
	}
	// O retConsGTIN pode vir direto no Body ou dentro de <ccgConsGTINResult>;
	// o decoder acha o elemento pelo nome em qualquer profundidade.
	ret, err := lerRetConsGTIN(inner)
	if err != nil {
		return RetornoGTIN{}, err
	}
	if ret.CStat != CStatGTINSucesso {
		return ret, &ErroGTIN{CStat: ret.CStat, XMotivo: ret.XMotivo}
	}
	return ret, nil
}

func lerRetConsGTIN(inner []byte) (RetornoGTIN, error) {
	type xmlRet struct {
		VerAplic string   `xml:"verAplic"`
		CStat    string   `xml:"cStat"`
		XMotivo  string   `xml:"xMotivo"`
		DhResp   string   `xml:"dhResp"`
		GTIN     string   `xml:"GTIN"`
		TpGTIN   string   `xml:"tpGTIN"`
		XProd    string   `xml:"xProd"`
		NCM      string   `xml:"NCM"`
		CEST     []string `xml:"CEST"`
	}
	dec := xml.NewDecoder(bytes.NewReader(inner))
	for {
		tok, err := dec.Token()
		if err != nil {
			return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: retConsGTIN ausente na resposta: %s", truncar(inner, 256))
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "retConsGTIN" {
			continue
		}
		var r xmlRet
		if err := dec.DecodeElement(&r, &se); err != nil {
			return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: parse retConsGTIN: %w", err)
		}
		if r.CStat == "" {
			return RetornoGTIN{}, fmt.Errorf("sefaz: consulta GTIN: retConsGTIN sem cStat")
		}
		return RetornoGTIN(r), nil
	}
}
