package sefaz

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// EntradaEPEC é o resumo mínimo que o evento prévio de EPEC (tpEvento
// 110140) exige — não é a NF-e inteira, só os campos que a NT 2014/001
// (item 03.1a-A, schema leiauteEPEC_v1.00.xsd) pede pra SEFAZ liberar a nota
// em contingência antes do canal normal voltar.
//
// DhEmi deve ser exatamente a mesma data/hora de emissão gravada no <ide>
// da NF-e que foi (ou vai ser) assinada com TpEmis=4 — é por ela, junto da
// chave, que a SEFAZ reconcilia o EPEC com a NF-e completa quando ela chegar
// depois (rejeição 467/468 se divergir ou não existir).
type EntradaEPEC struct {
	ChNFe      string // chave de acesso, 44 dígitos
	CNPJ       string // emitente, só dígitos
	UFEmitente string // cUF do emitente (2 dígitos) — vira cOrgaoAutor
	IE         string // IE do emitente
	DhEmi      string // dhEmi da NF-e, formato "2006-01-02T15:04:05-07:00"
	TpNF       string // "0"=entrada "1"=saída
	DestUF     string // "EX" se destinatário no exterior (não suportado ainda)
	DestCNPJ   string // um dos dois obrigatório
	DestCPF    string
	DestIE     string // opcional
	VNF        string // ICMSTot.VNF da NF-e — ver builder.CalcularTotaisEPEC
	VICMS      string // ICMSTot.VICMS
	VST        string // ICMSTot.VST
}

// RetornoEPEC é o retorno do NFeRecepcaoEvento pro registro do EPEC.
// CStatEvento == "135" ("Evento registrado e vinculado a NF-e") confirma que
// a SEFAZ aceitou o EPEC — a partir daí a NF-e pode circular com o DANFE em
// contingência.
type RetornoEPEC struct {
	RetornoSEFAZ
	NProt         string
	CStatEvento   string
	XMotivoEvento string
}

// RegistrarEPEC envia o evento prévio de EPEC pro Ambiente Nacional
// (cOrgao=91) — não é o webservice da UF do emitente nem SVC-AN/SVC-RS
// (esses são pro modo de contingência tpEmis=6/7, um mecanismo diferente).
// O cliente SEFAZ desta chamada é sempre roteado pro AN (ver endpoints.go,
// pseudo-cUF "91"), independente da UF real do emitente — só EntradaEPEC.UF*
// carrega a UF de verdade, usada nos campos do evento.
func RegistrarEPEC(ctx context.Context, in EntradaEPEC, amb Ambiente, c *cert.Certificado) (*RetornoEPEC, error) {
	if len(in.ChNFe) != 44 {
		return nil, fmt.Errorf("sefaz: EPEC: chNFe deve ter 44 dígitos (tem %d)", len(in.ChNFe))
	}
	if in.DestCNPJ == "" && in.DestCPF == "" {
		return nil, fmt.Errorf("sefaz: EPEC: destinatário precisa de CNPJ ou CPF (idEstrangeiro não suportado)")
	}

	cl, err := NovoCliente("91", amb, c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: EPEC: criar cliente AN: %w", err)
	}

	dhEvento := agoraBrasilia()
	idEvento := fmt.Sprintf("ID110140%s01", in.ChNFe)

	destDoc := fmt.Sprintf("<CNPJ>%s</CNPJ>", in.DestCNPJ)
	if in.DestCNPJ == "" {
		destDoc = fmt.Sprintf("<CPF>%s</CPF>", in.DestCPF)
	}
	destIE := ""
	if in.DestIE != "" {
		destIE = fmt.Sprintf("<IE>%s</IE>", in.DestIE)
	}
	dest := fmt.Sprintf(
		`<dest><UF>%s</UF>%s%s<vNF>%s</vNF><vICMS>%s</vICMS><vST>%s</vST></dest>`,
		in.DestUF, destDoc, destIE, in.VNF, in.VICMS, in.VST,
	)

	xmlEvento := fmt.Sprintf(
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infEvento Id="%s">`+
			`<cOrgao>91</cOrgao>`+
			`<tpAmb>%s</tpAmb>`+
			`<CNPJ>%s</CNPJ>`+
			`<chNFe>%s</chNFe>`+
			`<dhEvento>%s</dhEvento>`+
			`<tpEvento>110140</tpEvento>`+
			`<nSeqEvento>1</nSeqEvento>`+
			`<verEvento>1.00</verEvento>`+
			`<detEvento versao="1.00">`+
			`<descEvento>EPEC</descEvento>`+
			`<cOrgaoAutor>%s</cOrgaoAutor>`+
			`<tpAutor>1</tpAutor>`+
			`<verAplic>nfe-go</verAplic>`+
			`<dhEmi>%s</dhEmi>`+
			`<tpNF>%s</tpNF>`+
			`<IE>%s</IE>`+
			`%s`+
			`</detEvento>`+
			`</infEvento>`+
			`</evento>`,
		idEvento, string(amb), in.CNPJ, in.ChNFe, dhEvento,
		in.UFEmitente, in.DhEmi, in.TpNF, in.IE, dest,
	)

	eventoAssinado, err := sign.AssinarEvento([]byte(xmlEvento), c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: EPEC: assinar evento: %w", err)
	}

	// Sem wrapper de operação — o WSDL do Ambiente Nacional declara o body
	// como document/literal NÃO empacotado (wsdl:part aponta direto pro
	// elemento nfeDadosMsg, não pra um elemento com nome da operação). Com
	// <nfeRecepcaoEventoNF> por fora (mesmo padrão usado pro webservice
	// estadual, que É empacotado), o servidor do AN devolve SOAP Fault
	// "Object reference not set to an instance of an object" — confirmado
	// batendo direto no endpoint de homologação com certificado real.
	soapBody := fmt.Sprintf(
		`<nfeDadosMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4">`+
			`<envEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<idLote>%d</idLote>`+
			`%s`+
			`</envEvento>`+
			`</nfeDadosMsg>`,
		time.Now().UnixMilli(), string(eventoAssinado),
	)

	respBody, err := cl.chamar(ctx, ServicoRecepcaoEventoAN, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlInfEvento struct {
		NProt   string `xml:"nProt"`
		CStat   string `xml:"cStat"`
		XMotivo string `xml:"xMotivo"`
	}
	type xmlRetEvento struct {
		RetornoSEFAZ
		InfEvento xmlInfEvento `xml:"retEvento>infEvento"`
	}
	type xmlResult struct {
		Ret xmlRetEvento `xml:"retEnvEvento"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: EPEC: parse retEnvEvento: %w", err)
	}

	return &RetornoEPEC{
		RetornoSEFAZ:  result.Ret.RetornoSEFAZ,
		NProt:         result.Ret.InfEvento.NProt,
		CStatEvento:   result.Ret.InfEvento.CStat,
		XMotivoEvento: result.Ret.InfEvento.XMotivo,
	}, nil
}
