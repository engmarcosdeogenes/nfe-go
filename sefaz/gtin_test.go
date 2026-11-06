package sefaz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// comDV completa o GTIN com o dígito verificador GS1, independente de dvGTINValido.
func comDV(semDV string) string {
	soma := 0
	for i := len(semDV) - 1; i >= 0; i-- {
		d := int(semDV[i] - '0')
		if (len(semDV)-1-i)%2 == 0 {
			d *= 3
		}
		soma += d
	}
	return semDV + strconv.Itoa((10-soma%10)%10)
}

func TestNormalizarGTIN(t *testing.T) {
	gtin13 := comDV("789100031550")
	gtin8 := comDV("7891234")
	gtin14 := comDV("1789100031550")
	casos := []struct {
		nome, in, want string
		errAlvo        error
	}{
		{"13 com espaço", " " + gtin13[:7] + " " + gtin13[7:], gtin13, nil},
		{"8", gtin8, gtin8, nil},
		{"14", gtin14, gtin14, nil},
		{"790", comDV("790123456789"), comDV("790123456789"), nil},
		{"DV errado", gtin13[:12] + strconv.Itoa((int(gtin13[12]-'0')+1)%10), "", ErrGTINDVInvalido},
		{"tamanho", "12345", "", ErrGTINDVInvalido},
		{"14 com zero", "0" + gtin13, "", ErrGTINDVInvalido},
		{"prefixo estrangeiro", comDV("400638133393"), "", ErrGTINPrefixo},
		{"12 dígitos EUA", comDV("03600029145"), "", ErrGTINPrefixo},
	}
	for _, c := range casos {
		got, err := NormalizarGTIN(c.in)
		if c.errAlvo != nil {
			if !errors.Is(err, c.errAlvo) {
				t.Errorf("%s: err = %v, quer %v", c.nome, err, c.errAlvo)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; quer %q", c.nome, got, err, c.want)
		}
	}
}

const retSucesso = `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body>
<ccgConsGTINResponse xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/ccgConsGtin"><ccgConsGTINResult>
<retConsGTIN versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">
<verAplic>RS20230101</verAplic><cStat>9490</cStat><xMotivo>Consulta realizada com sucesso</xMotivo>
<dhResp>2026-10-29T10:00:00-03:00</dhResp><GTIN>%GTIN%</GTIN><tpGTIN>13</tpGTIN>
<xProd>ACHOCOLATADO EM PO 400G</xProd><NCM>18069000</NCM><CEST>1700500</CEST><CEST>1700600</CEST>
</retConsGTIN></ccgConsGTINResult></ccgConsGTINResponse></soap:Body></soap:Envelope>`

const retRejeicao = `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body>
<ccgConsGTINResponse xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/ccgConsGtin"><ccgConsGTINResult>
<retConsGTIN versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">
<verAplic>RS20230101</verAplic><cStat>%CSTAT%</cStat><xMotivo>Rejeicao</xMotivo><dhResp>2026-10-29T10:00:00-03:00</dhResp>
</retConsGTIN></ccgConsGTINResult></ccgConsGTINResponse></soap:Body></soap:Envelope>`

// servidorGTIN sobe um httptest TLS, aponta urlConsultaGTIN para ele e devolve o
// tls.Config que confia no certificado de teste.
func servidorGTIN(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	ant := urlConsultaGTIN
	urlConsultaGTIN = srv.URL
	t.Cleanup(func() { urlConsultaGTIN = ant })
	return srv
}

func TestConsultarGTIN_Sucesso(t *testing.T) {
	gtin := comDV("789100031550")
	var corpo, action, ct string
	srv := servidorGTIN(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		corpo, action, ct = string(b), r.Header.Get("SOAPAction"), r.Header.Get("Content-Type")
		io.WriteString(w, strings.ReplaceAll(retSucesso, "%GTIN%", gtin))
	})
	tlsCfg := srv.Client().Transport.(*http.Transport).TLSClientConfig

	ret, err := ConsultarGTIN(context.Background(), tlsCfg, gtin)
	if err != nil {
		t.Fatal(err)
	}
	if ret.CStat != "9490" || ret.GTIN != gtin || ret.NCM != "18069000" || ret.XProd != "ACHOCOLATADO EM PO 400G" || ret.TpGTIN != "13" {
		t.Errorf("retorno: %+v", ret)
	}
	if len(ret.CEST) != 2 || ret.CEST[1] != "1700600" {
		t.Errorf("CEST: %v", ret.CEST)
	}
	if !strings.Contains(corpo, `<consGTIN versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><GTIN>`+gtin+`</GTIN></consGTIN>`) {
		t.Errorf("corpo sem consGTIN esperado: %s", corpo)
	}
	if !strings.Contains(corpo, `<nfeDadosMsg xmlns="`+nsWSDLConsultaGTIN+`">`) {
		t.Errorf("corpo sem nfeDadosMsg: %s", corpo)
	}
	if action != soapActionConsultaGTIN || !strings.Contains(ct, `action="`+soapActionConsultaGTIN+`"`) {
		t.Errorf("action=%q content-type=%q", action, ct)
	}
}

func TestConsultarGTIN_Rejeicoes(t *testing.T) {
	casos := map[string]error{
		"9491": ErrGTINDVInvalido,
		"9492": ErrGTINPrefixo,
		"9493": ErrGTINCertNaoEmitente,
		"9494": ErrGTINInexistente,
		"9495": ErrGTINSituacaoInvalida,
		"9496": ErrGTINNaoAutorizado,
		"9497": ErrGTINNCMNaoCadastrado,
		"9498": ErrGTINNCMInvalido,
		"656":  ErrGTINConsumoIndevido,
		"284":  ErrGTINCertificado,
		"109":  ErrGTINServicoParalisado,
		"215":  ErrGTINRejeicao,
	}
	for cstat, alvo := range casos {
		srv := servidorGTIN(t, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, strings.ReplaceAll(retRejeicao, "%CSTAT%", cstat))
		})
		ret, err := ConsultarGTIN(context.Background(), srv.Client().Transport.(*http.Transport).TLSClientConfig, comDV("789100031550"))
		var eg *ErroGTIN
		if !errors.Is(err, alvo) || !errors.As(err, &eg) || eg.CStat != cstat || ret.CStat != cstat {
			t.Errorf("cStat %s: err=%v ret=%+v", cstat, err, ret)
		}
	}
}

func TestConsultarGTIN_ValidaAntesDeChamar(t *testing.T) {
	chamou := false
	srv := servidorGTIN(t, func(w http.ResponseWriter, r *http.Request) { chamou = true })
	_, err := ConsultarGTIN(context.Background(), srv.Client().Transport.(*http.Transport).TLSClientConfig, comDV("400638133393"))
	if !errors.Is(err, ErrGTINPrefixo) || chamou {
		t.Errorf("err=%v chamou=%v", err, chamou)
	}
}

func TestConsultarGTIN_HTTPErro(t *testing.T) {
	srv := servidorGTIN(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fault", http.StatusInternalServerError)
	})
	_, err := ConsultarGTIN(context.Background(), srv.Client().Transport.(*http.Transport).TLSClientConfig, comDV("789100031550"))
	var eg *ErroGTIN
	if err == nil || errors.As(err, &eg) || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("err=%v", err)
	}
}
