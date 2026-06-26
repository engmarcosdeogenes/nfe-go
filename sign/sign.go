// Package sign implementa a assinatura digital xmldsig da NF-e conforme
// Manual de Integração da SEFAZ (seção 4 — assinatura digital).
//
// Fluxo:
//  1. Extrai <infNFe> do XML e adiciona xmlns → forma canônica
//  2. SHA-1 do infNFe canônico → DigestValue
//  3. Monta <SignedInfo> com DigestValue (já em forma canônica)
//  4. RSA-SHA1(<SignedInfo canônico>) → SignatureValue
//  5. Monta <Signature> e insere no XML antes de </NFe>
package sign

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 — SHA-1 é obrigatório pelo Manual de Integração NF-e da SEFAZ (xmldsig)
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

// ── API pública ───────────────────────────────────────────────────────────────

// AssinarNFe recebe o XML bruto da NF-e (saído do builder) e o certificado,
// e retorna o XML completo com <Signature> embutido, pronto para enviar à SEFAZ.
func AssinarNFe(nfeXML []byte, c *cert.Certificado) ([]byte, error) {
	return assinarGenerico(nfeXML, []byte("<infNFe "), []byte("</NFe>"), c)
}

// AssinarEvento assina um XML de evento NF-e (cancelamento, CCe, etc.).
// O elemento assinado é <infEvento Id="...">.
func AssinarEvento(eventoXML []byte, c *cert.Certificado) ([]byte, error) {
	return assinarGenerico(eventoXML, []byte("<infEvento "), []byte("</evento>"), c)
}

// AssinarInutilizacao assina um XML de inutilização NF-e.
// O elemento assinado é <infInut Id="...">.
func AssinarInutilizacao(inutXML []byte, c *cert.Certificado) ([]byte, error) {
	return assinarGenerico(inutXML, []byte("<infInut "), []byte("</inutNFe>"), c)
}

// ── Núcleo de assinatura ──────────────────────────────────────────────────────

// assinarGenerico assina qualquer elemento xmldsig da SEFAZ (infNFe, infEvento, infInut).
func assinarGenerico(xmlBytes, tagInicio, tagFechamentoRaiz []byte, c *cert.Certificado) ([]byte, error) {
	elementoC14N, refURI, err := extrairElementoC14N(xmlBytes, tagInicio)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	digestB64 := sha1B64(elementoC14N)
	signedInfoC14N := montarSignedInfoC14N(refURI, digestB64)

	sigBytes, err := assinarRSASHA1(c.Chave, []byte(signedInfoC14N))
	if err != nil {
		return nil, fmt.Errorf("sign: RSA-SHA1: %w", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)
	certB64 := base64.StdEncoding.EncodeToString(c.CertDER)

	// Dentro de <Signature xmlns="...">, o <SignedInfo> não repete xmlns.
	signedInfoEmbed := strings.Replace(signedInfoC14N,
		` xmlns="http://www.w3.org/2000/09/xmldsig#"`, "", 1)
	signature := []byte(fmt.Sprintf(
		`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">%s<SignatureValue>%s</SignatureValue>`+
			`<KeyInfo><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo></Signature>`,
		signedInfoEmbed, sigB64, certB64,
	))

	idx := bytes.LastIndex(xmlBytes, tagFechamentoRaiz)
	if idx < 0 {
		return append(xmlBytes, signature...), nil
	}
	resultado := make([]byte, 0, len(xmlBytes)+len(signature))
	resultado = append(resultado, xmlBytes[:idx]...)
	resultado = append(resultado, signature...)
	resultado = append(resultado, xmlBytes[idx:]...)
	return resultado, nil
}

// ── Extração e canonicalização ────────────────────────────────────────────────

// extrairElementoC14N localiza o elemento identificado por tagInicio dentro de xmlBytes,
// adiciona o namespace explícito e retorna os bytes canônicos + o valor de Id.
// tagInicio deve incluir o espaço final: ex []byte("<infNFe ")
func extrairElementoC14N(xmlBytes, tagInicio []byte) ([]byte, string, error) {
	ini := bytes.Index(xmlBytes, tagInicio)
	if ini < 0 {
		return nil, "", fmt.Errorf("tag %s não encontrada no XML", tagInicio)
	}

	fimAbertura := bytes.Index(xmlBytes[ini:], []byte(">"))
	if fimAbertura < 0 {
		return nil, "", fmt.Errorf("tag %s não fechada", tagInicio)
	}
	abertura := string(xmlBytes[ini : ini+fimAbertura+1])

	refURI := extrairAtrib(abertura, "Id")
	if refURI == "" {
		return nil, "", fmt.Errorf("atributo Id não encontrado em %s", tagInicio)
	}

	// Deriva nome da tag para localizar o fechamento
	nomeTag := strings.TrimSuffix(strings.TrimPrefix(string(tagInicio), "<"), " ")
	fechamento := []byte("</" + nomeTag + ">")
	fim := bytes.Index(xmlBytes[ini:], fechamento)
	if fim < 0 {
		return nil, "", fmt.Errorf("</%s> não encontrado", nomeTag)
	}
	fim = ini + fim + len(fechamento)

	elemento := make([]byte, fim-ini)
	copy(elemento, xmlBytes[ini:fim])

	// Insere xmlns logo após a abertura da tag (namespace decl antes de atributos regulares)
	const ns = `xmlns="http://www.portalfiscal.inf.br/nfe" `
	elemento = bytes.Replace(elemento, tagInicio, append(tagInicio, []byte(ns)...), 1)

	return elemento, refURI, nil
}

// extrairAtrib retorna o valor de um atributo XML: Attr="valor".
func extrairAtrib(tag, nome string) string {
	busca := nome + `="`
	idx := strings.Index(tag, busca)
	if idx < 0 {
		return ""
	}
	ini := idx + len(busca)
	fim := strings.Index(tag[ini:], `"`)
	if fim < 0 {
		return ""
	}
	return tag[ini : ini+fim]
}

// ── SignedInfo ────────────────────────────────────────────────────────────────

// montarSignedInfoC14N monta o <SignedInfo> em forma canônica (xmlns explícito,
// elementos vazios com </tag> — C14N exige <elem></elem>, não <elem/>).
func montarSignedInfoC14N(refURI, digestB64 string) string {
	return fmt.Sprintf(
		`<SignedInfo xmlns="http://www.w3.org/2000/09/xmldsig#">`+
			`<CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"></CanonicalizationMethod>`+
			`<SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"></SignatureMethod>`+
			`<Reference URI="#%s">`+
			`<Transforms>`+
			`<Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"></Transform>`+
			`<Transform Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"></Transform>`+
			`</Transforms>`+
			`<DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"></DigestMethod>`+
			`<DigestValue>%s</DigestValue>`+
			`</Reference>`+
			`</SignedInfo>`,
		refURI, digestB64,
	)
}

// ── Criptografia ──────────────────────────────────────────────────────────────

func sha1B64(data []byte) string {
	h := sha1.Sum(data) // #nosec G401 — SHA-1 obrigatório pelo padrão xmldsig/SEFAZ
	return base64.StdEncoding.EncodeToString(h[:])
}

func assinarRSASHA1(chave *rsa.PrivateKey, data []byte) ([]byte, error) {
	h := sha1.New() // #nosec G401 — SHA-1 obrigatório pelo padrão xmldsig/SEFAZ
	h.Write(data)
	digest := h.Sum(nil)
	return rsa.SignPKCS1v15(rand.Reader, chave, crypto.SHA1, digest)
}

// ── Verificação ───────────────────────────────────────────────────────────────

// VerificarAssinatura verifica a assinatura de um XML de NF-e já assinado.
// Útil para validar o próprio output antes de enviar à SEFAZ.
func VerificarAssinatura(nfeAssinado []byte, c *cert.Certificado) error {
	infNFeC14N, refURI, err := extrairElementoC14N(nfeAssinado, []byte("<infNFe "))
	if err != nil {
		return fmt.Errorf("verificar: %w", err)
	}

	digestEsperado := sha1B64(infNFeC14N)

	digestNoXML := extrairTagConteudo(nfeAssinado, "DigestValue")
	if digestNoXML == "" {
		return fmt.Errorf("verificar: DigestValue não encontrado")
	}
	if digestEsperado != digestNoXML {
		return fmt.Errorf("verificar: digest diverge\n  esperado: %s\n  no XML:   %s",
			digestEsperado, digestNoXML)
	}

	sigB64 := extrairTagConteudo(nfeAssinado, "SignatureValue")
	if sigB64 == "" {
		return fmt.Errorf("verificar: SignatureValue não encontrado")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("verificar: SignatureValue base64 inválido: %w", err)
	}

	signedInfoC14N := montarSignedInfoC14N(refURI, digestNoXML)
	h := sha1.New() // #nosec G401 — SHA-1 obrigatório pelo padrão xmldsig/SEFAZ
	h.Write([]byte(signedInfoC14N))
	digest := h.Sum(nil)

	if err := rsa.VerifyPKCS1v15(&c.Chave.PublicKey, crypto.SHA1, digest, sigBytes); err != nil {
		return fmt.Errorf("verificar: assinatura RSA inválida: %w", err)
	}

	return nil
}

func extrairTagConteudo(xmlBytes []byte, tag string) string {
	abre := []byte("<" + tag + ">")
	fecha := []byte("</" + tag + ">")
	ini := bytes.Index(xmlBytes, abre)
	if ini < 0 {
		return ""
	}
	ini += len(abre)
	fim := bytes.Index(xmlBytes[ini:], fecha)
	if fim < 0 {
		return ""
	}
	return string(xmlBytes[ini : ini+fim])
}
