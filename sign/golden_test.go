package sign_test

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// Golden gravado com o código ANTERIOR à generalização por namespace: a
// NF-e/evento/inutilização não podem mudar nem 1 byte. O SignedInfo depende só
// do digest e do Id, então o hash dele também é determinístico.
const (
	xmlNFeGolden  = `<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe52260611222333000181550010000000011000000019" versao="4.00"><ide><cUF>52</cUF></ide></infNFe></NFe>`
	xmlEvGolden   = `<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infEvento Id="ID1101115226"><cOrgao>52</cOrgao></infEvento></evento>`
	xmlInutGolden = `<inutNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infInut Id="ID52"><tpAmb>2</tpAmb></infInut></inutNFe>`
)

var reSigVal = regexp.MustCompile(`<SignatureValue>.*?</SignatureValue>|<X509Certificate>.*?</X509Certificate>`)

func golden(t *testing.T, saida []byte) (digest, hashSemRSA string) {
	t.Helper()
	m := regexp.MustCompile(`<DigestValue>(.*?)</DigestValue>`).FindSubmatch(saida)
	if m == nil {
		t.Fatal("sem DigestValue")
	}
	h := sha256.Sum256(reSigVal.ReplaceAll(saida, nil))
	return string(m[1]), hex.EncodeToString(h[:])
}

func certGolden(t *testing.T) *cert.Certificado {
	t.Helper()
	pfx, err := cert.GerarCertificadoTeste("11222333000181", "teste")
	if err != nil {
		t.Fatal(err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGolden_NFeEventoInutInalterados(t *testing.T) {
	c := certGolden(t)
	casos := []struct {
		nome, xml, digest, hash string
		f                       func([]byte, *cert.Certificado) ([]byte, error)
	}{
		{"nfe", xmlNFeGolden, "uqZzJG6XthgjlaigmX1dLtrdc/M=", "8200adfc43dd8e2afeb54090b740481d3dd9785ff45e88dea5703ab68c473bc6", sign.AssinarNFe},
		{"evento", xmlEvGolden, "b2AQSehEMFO0azUNFAfcgE9CKHQ=", "2d17526548f26c26632eaa8cae8bda67e36463633a80305dccb6087f1a18468d", sign.AssinarEvento},
		{"inut", xmlInutGolden, "OkvZCZ3vUo9rvTF6jSSSJxiBd2M=", "16f3be2fb542dfcf5fa0d4d2e2debd7437587619b2a3b0a3cc5338e97acbbc15", sign.AssinarInutilizacao},
	}
	for _, k := range casos {
		out, err := k.f([]byte(k.xml), c)
		if err != nil {
			t.Fatal(err)
		}
		d, h := golden(t, out)
		if d != k.digest || h != k.hash {
			t.Errorf("%s: digest=%s hash=%s", k.nome, d, h)
		}
	}
}
