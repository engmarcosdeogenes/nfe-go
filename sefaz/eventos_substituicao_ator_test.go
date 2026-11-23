package sefaz_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

// sefazEventoFalsa guarda o corpo enviado e responde 128/135 para o evento.
type sefazEventoFalsa struct {
	enviado string
	cStat   string
}

func (s *sefazEventoFalsa) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	s.enviado = string(b)
	corpo := `<?xml version="1.0" encoding="UTF-8"?><soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope"><soap12:Body><nfeResultMsg>` +
		`<retEnvEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00"><idLote>1</idLote><tpAmb>2</tpAmb><verAplic>T</verAplic>` +
		`<cStat>128</cStat><xMotivo>Lote de Evento Processado</xMotivo><retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><cStat>` + s.cStat + `</cStat>` +
		`<xMotivo>Evento registrado e vinculado a NF-e</xMotivo><chNFe>X</chNFe><nProt>152260000000999</nProt></infEvento></retEvento>` +
		`</retEnvEvento></nfeResultMsg></soap12:Body></soap12:Envelope>`
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(corpo)), Header: make(http.Header)}, nil
}

const (
	chaveNFCe      = "52261134152609000106650010000001231000001234"
	chaveNFCeSubst = "52261134152609000106650010000001249000001240"
	chaveNFe       = "52261134152609000106550010000009051000000905"
)

func contem(t *testing.T, xml string, trechos ...string) {
	t.Helper()
	for _, s := range trechos {
		if !strings.Contains(xml, s) {
			t.Errorf("evento enviado sem %q:\n%s", s, xml)
		}
	}
}

func TestCancelarPorSubstituicao_LeiauteMOC(t *testing.T) {
	falsa := &sefazEventoFalsa{cStat: "135"}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, falsa)
	ret, err := cl.CancelarPorSubstituicao(context.Background(), chaveNFCe, "152260000000111", "Venda duplicada em contingencia & online", chaveNFCeSubst, certTeste(t))
	if err != nil {
		t.Fatal(err)
	}
	if ret.CStatEvento != "135" || ret.NProt != "152260000000999" || len(ret.XMLProcEvento) == 0 {
		t.Errorf("retorno: %+v", ret)
	}
	contem(t, falsa.enviado,
		`<infEvento Id="ID110112`+chaveNFCe+`01">`, `<cOrgao>52</cOrgao>`, `<tpAmb>2</tpAmb>`, `<CNPJ>34152609000106</CNPJ>`,
		`<tpEvento>110112</tpEvento><nSeqEvento>1</nSeqEvento><verEvento>1.00</verEvento>`,
		`<detEvento versao="1.00"><descEvento>Cancelamento por substituicao</descEvento><cOrgaoAutor>52</cOrgaoAutor><tpAutor>1</tpAutor><verAplic>nfe-go</verAplic>`+
			`<nProt>152260000000111</nProt><xJust>Venda duplicada em contingencia &amp; online</xJust><chNFeRef>`+chaveNFCeSubst+`</chNFeRef></detEvento>`,
		`<Signature`)
}

func TestCancelarPorSubstituicao_Recusas(t *testing.T) {
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, &sefazEventoFalsa{cStat: "135"})
	casos := map[string][3]string{
		"nfe modelo 55":    {chaveNFe, chaveNFCeSubst, "justificativa longa o bastante"},
		"substituta = ela": {chaveNFCe, chaveNFCe, "justificativa longa o bastante"},
		"substituta curta": {chaveNFCe, "123", "justificativa longa o bastante"},
		"just curta":       {chaveNFCe, chaveNFCeSubst, "curta"},
	}
	for nome, c := range casos {
		if _, err := cl.CancelarPorSubstituicao(context.Background(), c[0], "1", c[2], c[1], certTeste(t)); err == nil {
			t.Errorf("%s: esperava erro", nome)
		}
	}
}

func TestAtorInteressado_LeiauteMOC(t *testing.T) {
	falsa := &sefazEventoFalsa{cStat: "135"}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, falsa)
	if _, err := cl.AtorInteressado(context.Background(), chaveNFe, "11222333000181", "", false, 2, certTeste(t)); err != nil {
		t.Fatal(err)
	}
	contem(t, falsa.enviado,
		`<infEvento Id="ID110150`+chaveNFe+`02">`, `<tpEvento>110150</tpEvento><nSeqEvento>2</nSeqEvento>`,
		`<detEvento versao="1.00"><descEvento>Ator interessado na NF-e</descEvento><cOrgaoAutor>52</cOrgaoAutor><tpAutor>1</tpAutor><verAplic>nfe-go</verAplic>`+
			`<autXML><CNPJ>11222333000181</CNPJ></autXML><tpAutorizacao>0</tpAutorizacao></detEvento>`)
	if strings.Contains(falsa.enviado, "xCondUso") {
		t.Error("tpAutorizacao 0 não pode levar xCondUso (830)")
	}

	if _, err := cl.AtorInteressado(context.Background(), chaveNFe, "", "12345678909", true, 1, certTeste(t)); err != nil {
		t.Fatal(err)
	}
	contem(t, falsa.enviado, `<autXML><CPF>12345678909</CPF></autXML><tpAutorizacao>1</tpAutorizacao><xCondUso>`+sefaz.XCondUsoAtorInteressado+`</xCondUso></detEvento>`)

	for nome, f := range map[string]func() error{
		"nfce": func() error {
			_, e := cl.AtorInteressado(context.Background(), chaveNFCe, "11222333000181", "", false, 1, certTeste(t))
			return e
		},
		"seq 21": func() error {
			_, e := cl.AtorInteressado(context.Background(), chaveNFe, "11222333000181", "", false, 21, certTeste(t))
			return e
		},
		"cnpj e cpf": func() error {
			_, e := cl.AtorInteressado(context.Background(), chaveNFe, "11222333000181", "12345678909", false, 1, certTeste(t))
			return e
		},
		"nenhum": func() error {
			_, e := cl.AtorInteressado(context.Background(), chaveNFe, "", "", false, 1, certTeste(t))
			return e
		},
	} {
		if f() == nil {
			t.Errorf("%s: esperava erro", nome)
		}
	}
}
