package sign_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

func gerarXMLeTeste(t *testing.T) ([]byte, *cert.Certificado) {
	t.Helper()

	entrada := builder.EntradaNFe{
		Serie:    "1",
		NNF:      "1",
		DhEmi:    time.Date(2026, 6, 25, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA DE MERCADORIA",
		TpAmb:    "2",
		FinNFe:   "1",
		IndFinal: "0",
		IndPres:  "1",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "METALURGICA TESTE LTDA",
			IE: "123456789", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Chapas", Numero: "100", Bairro: "Industrial",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Dest: builder.EntradaDest{
			CNPJ: "99888777000155", Nome: "CLIENTE TESTE SA", IndIEDest: "9",
			End: builder.EntradaEndereco{
				Logradouro: "Av. Teste", Numero: "1", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74100000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Itens: []builder.EntradaItem{{
			CProd: "P001", CEAN: "SEM GTIN", Nome: "PRODUTO TESTE",
			NCM: "72193300", CFOP: "5102", Unidade: "UN",
			Quantidade: 10, VUnitario: 100.00,
			ICMS: builder.EntradaICMS{CSOSN: "400"},
		}},
		Frete:     builder.EntradaFrete{Modalidade: "9"},
		Pagamento: []builder.EntradaPagamento{{Forma: "01", Valor: 1000.00}},
	}

	xmlBytes, _, err := builder.Build(entrada)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	pfx, err := cert.GerarCertificadoTeste("11222333000181", "teste")
	if err != nil {
		t.Fatalf("GerarCertificado: %v", err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatalf("CarregarPFX: %v", err)
	}

	return xmlBytes, c
}

func TestAssinarNFe(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatalf("AssinarNFe: %v", err)
	}

	// Deve conter bloco de assinatura
	if !bytes.Contains(assinado, []byte("<Signature xmlns=")) {
		t.Error("XML assinado não contém <Signature>")
	}
	if !bytes.Contains(assinado, []byte("<SignatureValue>")) {
		t.Error("XML assinado não contém <SignatureValue>")
	}
	if !bytes.Contains(assinado, []byte("<DigestValue>")) {
		t.Error("XML assinado não contém <DigestValue>")
	}
	if !bytes.Contains(assinado, []byte("<X509Certificate>")) {
		t.Error("XML assinado não contém <X509Certificate>")
	}

	// Deve manter </NFe> após a assinatura
	if !bytes.HasSuffix(assinado, []byte("</NFe>")) {
		t.Error("XML não termina com </NFe>")
	}

	// Namespace SEFAZ deve estar presente no infNFe (via NFe pai)
	if !bytes.Contains(assinado, []byte("portalfiscal.inf.br/nfe")) {
		t.Error("namespace SEFAZ ausente")
	}

	t.Logf("XML assinado (%d bytes)", len(assinado))
}

func TestVerificarAssinatura(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatalf("AssinarNFe: %v", err)
	}

	if err := sign.VerificarAssinatura(assinado, c); err != nil {
		t.Fatalf("VerificarAssinatura: %v", err)
	}
}

func TestXMLAlteradoFalhaVerificacao(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatalf("AssinarNFe: %v", err)
	}

	// Altera um byte no valor do produto (falsificação)
	adulterado := bytes.Replace(assinado,
		[]byte("<vProd>1000.00</vProd>"),
		[]byte("<vProd>9999.00</vProd>"), 1)

	if err := sign.VerificarAssinatura(adulterado, c); err == nil {
		t.Fatal("esperava erro de verificação com XML adulterado")
	} else {
		t.Logf("XML adulterado detectado corretamente: %v", err)
	}
}

func TestAssinaturaNaoQuebraEstrutura(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatal(err)
	}

	xml := string(assinado)

	// Signature deve estar DENTRO de NFe
	idxSig := strings.Index(xml, "<Signature")
	idxFechaNFe := strings.LastIndex(xml, "</NFe>")
	if idxSig > idxFechaNFe {
		t.Error("<Signature> está fora de </NFe>")
	}

	// Signature deve estar APÓS infNFe
	idxFechaInf := strings.LastIndex(xml, "</infNFe>")
	if idxSig < idxFechaInf {
		t.Error("<Signature> está antes de </infNFe>")
	}
}

func TestNamespaceInfNFeC14NContemXmlns(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatal(err)
	}

	// O DigestValue foi calculado sobre infNFe com xmlns — verificação indireta:
	// se VerificarAssinatura passa, a C14N está correta
	if err := sign.VerificarAssinatura(assinado, c); err != nil {
		t.Fatalf("C14N provavelmente incorreto: %v", err)
	}
}

// ── AssinarEvento ─────────────────────────────────────────────────────────────

func eventoXML(chave string) []byte {
	return []byte(`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<infEvento Id="ID110111` + chave + `01">` +
		`<cOrgao>52</cOrgao>` +
		`<tpAmb>2</tpAmb>` +
		`<CNPJ>11222333000181</CNPJ>` +
		`<chNFe>` + chave + `</chNFe>` +
		`<dhEvento>2026-06-25T10:00:00-03:00</dhEvento>` +
		`<tpEvento>110111</tpEvento>` +
		`<nSeqEvento>1</nSeqEvento>` +
		`<verEvento>1.00</verEvento>` +
		`<detEvento versao="1.00">` +
		`<descEvento>Cancelamento</descEvento>` +
		`<nProt>352260000123456</nProt>` +
		`<xJust>Cancelamento solicitado pelo cliente por erro de digitacao</xJust>` +
		`</detEvento>` +
		`</infEvento>` +
		`</evento>`)
}

func TestAssinarEvento(t *testing.T) {
	_, c := gerarXMLeTeste(t)
	chave := "52260611222333000181550010000000011234567890"
	// Chave fictícia de 44 dígitos para o teste
	chave = "52260611222333000181550010000000421047402703"

	xml := eventoXML(chave)
	assinado, err := sign.AssinarEvento(xml, c)
	if err != nil {
		t.Fatalf("AssinarEvento: %v", err)
	}

	if !bytes.Contains(assinado, []byte("<Signature xmlns=")) {
		t.Error("evento assinado não contém <Signature>")
	}
	if !bytes.Contains(assinado, []byte("<SignatureValue>")) {
		t.Error("evento assinado não contém <SignatureValue>")
	}
	if !bytes.Contains(assinado, []byte("</evento>")) {
		t.Error("evento deve terminar com </evento>")
	}
	// Signature deve estar dentro de </evento>
	xml2 := string(assinado)
	idxSig := strings.Index(xml2, "<Signature")
	idxFecha := strings.LastIndex(xml2, "</evento>")
	if idxSig > idxFecha {
		t.Error("<Signature> está fora de </evento>")
	}
	t.Logf("evento assinado: %d bytes", len(assinado))
}

func TestAssinarInutilizacao(t *testing.T) {
	_, c := gerarXMLeTeste(t)

	inutXML := []byte(`<inutNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<infInut Id="ID52260611222333000181550010000000010000000091">` +
		`<tpAmb>2</tpAmb>` +
		`<xServ>INUTILIZAR</xServ>` +
		`<cUF>52</cUF>` +
		`<ano>26</ano>` +
		`<CNPJ>11222333000181</CNPJ>` +
		`<mod>55</mod>` +
		`<serie>001</serie>` +
		`<nNFIni>000000001</nNFIni>` +
		`<nNFFin>000000009</nNFFin>` +
		`<xJust>Notas emitidas por engano no sistema</xJust>` +
		`</infInut>` +
		`</inutNFe>`)

	assinado, err := sign.AssinarInutilizacao(inutXML, c)
	if err != nil {
		t.Fatalf("AssinarInutilizacao: %v", err)
	}

	if !bytes.Contains(assinado, []byte("<Signature xmlns=")) {
		t.Error("inutilização assinada não contém <Signature>")
	}
	if !bytes.Contains(assinado, []byte("</inutNFe>")) {
		t.Error("deve terminar com </inutNFe>")
	}
	// Signature antes de </inutNFe>
	xml2 := string(assinado)
	idxSig := strings.Index(xml2, "<Signature")
	idxFecha := strings.LastIndex(xml2, "</inutNFe>")
	if idxSig > idxFecha {
		t.Error("<Signature> está fora de </inutNFe>")
	}
	t.Logf("inutilização assinada: %d bytes", len(assinado))
}

func TestAssinarEvento_SemId(t *testing.T) {
	_, c := gerarXMLeTeste(t)
	xmlSemId := []byte(`<evento versao="1.00"><infEvento><tpEvento>110111</tpEvento></infEvento></evento>`)
	_, err := sign.AssinarEvento(xmlSemId, c)
	if err == nil {
		t.Fatal("esperava erro para infEvento sem atributo Id")
	}
	t.Logf("erro esperado: %v", err)
}

func TestAssinarNFe_SemInfNFe(t *testing.T) {
	_, c := gerarXMLeTeste(t)
	xmlInvalido := []byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><semtag/></NFe>`)
	_, err := sign.AssinarNFe(xmlInvalido, c)
	if err == nil {
		t.Fatal("esperava erro para XML sem <infNFe>")
	}
	t.Logf("erro esperado: %v", err)
}
