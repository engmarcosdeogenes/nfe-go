package sign_test

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 — SHA-1 exigido pelo xmldsig da SEFAZ
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// c14n escrito à mão (xmlns explícito antes de Id; filhos herdam o default).
// Conferência independente (Git Bash):
//
//	printf '%s' '<infMDFe xmlns="http://www.portalfiscal.inf.br/mdfe" Id="MDFe1" versao="3.00"><tpAmb>2</tpAmb></infMDFe>' | openssl dgst -sha1 -binary | base64
const (
	mdfeXML  = `<MDFe xmlns="http://www.portalfiscal.inf.br/mdfe"><infMDFe Id="MDFe1" versao="3.00"><tpAmb>2</tpAmb></infMDFe><infMDFeSupl><qrCodMDFe>x</qrCodMDFe></infMDFeSupl></MDFe>`
	mdfeC14N = `<infMDFe xmlns="http://www.portalfiscal.inf.br/mdfe" Id="MDFe1" versao="3.00"><tpAmb>2</tpAmb></infMDFe>`
	evXML    = `<eventoMDFe versao="3.00" xmlns="http://www.portalfiscal.inf.br/mdfe"><infEvento Id="ID1"><cOrgao>52</cOrgao></infEvento></eventoMDFe>`
	evC14N   = `<infEvento xmlns="http://www.portalfiscal.inf.br/mdfe" Id="ID1"><cOrgao>52</cOrgao></infEvento>`
)

func sha1b64(s string) string {
	h := sha1.Sum([]byte(s)) // #nosec G401
	return base64.StdEncoding.EncodeToString(h[:])
}

func TestDigest_MDFeEEventoMDFe(t *testing.T) {
	c := certGolden(t)
	re := regexp.MustCompile(`<DigestValue>(.*?)</DigestValue>`)
	casos := []struct {
		nome, xml, c14n, tag, fecha string
		f                           func([]byte, *cert.Certificado) ([]byte, error)
	}{
		{"mdfe", mdfeXML, mdfeC14N, "<infMDFe ", "</MDFe>", sign.AssinarMDFe},
		{"evento", evXML, evC14N, "<infEvento ", "</eventoMDFe>", sign.AssinarEventoMDFe},
	}
	for _, k := range casos {
		out, err := k.f([]byte(k.xml), c)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(re.FindSubmatch(out)[1]); got != sha1b64(k.c14n) {
			t.Errorf("%s: digest %s != %s", k.nome, got, sha1b64(k.c14n))
		}
		if !bytes.HasSuffix(out, []byte("</Signature>"+k.fecha)) {
			t.Errorf("%s: Signature deve ficar logo antes de %s", k.nome, k.fecha)
		}
		if err := sign.VerificarAssinaturaNS(out, k.tag, sign.NSMDFe, c); err != nil {
			t.Errorf("%s: verificar: %v", k.nome, err)
		}
		// adultera 1 byte dentro do elemento assinado
		bad := bytes.Replace(out, []byte("<cOrgao>52"), []byte("<cOrgao>53"), 1)
		bad = bytes.Replace(bad, []byte("<tpAmb>2"), []byte("<tpAmb>1"), 1)
		if bytes.Equal(bad, out) {
			t.Fatalf("%s: adulteração não aplicada", k.nome)
		}
		if sign.VerificarAssinaturaNS(bad, k.tag, sign.NSMDFe, c) == nil {
			t.Errorf("%s: adulterado deveria falhar", k.nome)
		}
		// namespace errado => digest diferente
		if sign.VerificarAssinaturaNS(out, k.tag, sign.NSCTe, c) == nil {
			t.Errorf("%s: ns errado deveria falhar", k.nome)
		}
	}
}

func TestDigest_MDFeSignatureDepoisDoSupl(t *testing.T) {
	out, err := sign.AssinarMDFe([]byte(mdfeXML), certGolden(t))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Index(s, "</infMDFeSupl>") > strings.Index(s, "<Signature") {
		t.Error("Signature deve vir depois de infMDFeSupl")
	}
}

func TestDigest_CTe(t *testing.T) {
	x := `<CTe xmlns="http://www.portalfiscal.inf.br/cte"><infCte Id="CTe1" versao="4.00"><a>1</a></infCte><infCTeSupl><q>x</q></infCTeSupl></CTe>`
	c := certGolden(t)
	out, err := sign.AssinarCTe([]byte(x), c)
	if err != nil {
		t.Fatal(err)
	}
	want := sha1b64(`<infCte xmlns="http://www.portalfiscal.inf.br/cte" Id="CTe1" versao="4.00"><a>1</a></infCte>`)
	if !bytes.Contains(out, []byte("<DigestValue>"+want+"</DigestValue>")) {
		t.Error("digest CTe diverge")
	}
	if err := sign.VerificarAssinaturaNS(out, "<infCte ", sign.NSCTe, c); err != nil {
		t.Error(err)
	}
	ev := `<eventoCTe versao="4.00" xmlns="http://www.portalfiscal.inf.br/cte"><infEvento Id="ID1"><a>1</a></infEvento></eventoCTe>`
	out, err = sign.AssinarEventoCTe([]byte(ev), c)
	if err != nil || sign.VerificarAssinaturaNS(out, "<infEvento ", sign.NSCTe, c) != nil {
		t.Errorf("evento cte: %v", err)
	}
}

func TestQRCodeContingencia_RSASHA1DaChave(t *testing.T) {
	c := certGolden(t)
	chave := "52261011222333000181580010000000011123456780"
	s, err := sign.AssinarQRCodeContingencia(chave, c)
	if err != nil {
		t.Fatal(err)
	}
	sig, _ := base64.StdEncoding.DecodeString(s)
	h := sha1.Sum([]byte(chave)) // #nosec G401
	if err := rsa.VerifyPKCS1v15(&c.Chave.PublicKey, crypto.SHA1, h[:], sig); err != nil {
		t.Errorf("assinatura inválida: %v", err)
	}
	if _, err := sign.AssinarQRCodeContingencia("123", c); err == nil {
		t.Error("chave curta deveria falhar")
	}
}
