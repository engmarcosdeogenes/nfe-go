package sefaz

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// RetornoEvento é o retorno do NFeRecepcaoEvento para um evento da NF-e/NFC-e.
// RetornoSEFAZ é o status do LOTE (128); o do evento (135/136/155 ou rejeição) é
// CStatEvento/XMotivoEvento.
type RetornoEvento = RetornoCancelamento

// verAplicEvento vai no verAplic dos eventos que exigem a versão do aplicativo do autor.
const verAplicEvento = "nfe-go"

// ── Cancelamento por substituição (evento 110112, só NFC-e) ───────────────────

// CancelarPorSubstituicao cancela uma NFC-e emitida em tpEmis=1 que saiu em duplicidade
// com outra NFC-e da mesma venda emitida em contingência off-line (MOC 5.9, NT 2018.004):
// a SEFAZ exige a substituta autorizada, em contingência (921), emitida em até 2 h (914)
// e com os mesmos totais/itens/destinatário (915-919), em até 168 h (501).
func (cl *Cliente) CancelarPorSubstituicao(ctx context.Context, chave, nProt, justificativa, chaveSubstituta string, c *cert.Certificado) (*RetornoEvento, error) {
	if len(chave) != 44 || chave[20:22] != "65" {
		return nil, fmt.Errorf("sefaz: cancelamento por substituição é só de NFC-e (modelo 65)")
	}
	if len(chaveSubstituta) != 44 || chaveSubstituta == chave || chaveSubstituta[20:22] != "65" {
		return nil, fmt.Errorf("sefaz: chave da NFC-e substituta inválida")
	}
	if n := len([]rune(justificativa)); n < 15 || n > 255 {
		return nil, fmt.Errorf("sefaz: justificativa deve ter entre 15 e 255 caracteres (tem %d)", n)
	}
	det := `<detEvento versao="1.00">` +
		`<descEvento>Cancelamento por substituicao</descEvento>` +
		`<cOrgaoAutor>` + chave[:2] + `</cOrgaoAutor>` +
		`<tpAutor>1</tpAutor>` +
		`<verAplic>` + verAplicEvento + `</verAplic>` +
		`<nProt>` + escaparXML(nProt) + `</nProt>` +
		`<xJust>` + escaparXML(justificativa) + `</xJust>` +
		`<chNFeRef>` + escaparXML(chaveSubstituta) + `</chNFeRef>` +
		`</detEvento>`
	return cl.enviarEvento(ctx, c, chave, "110112", 1, det)
}

// ── Ator interessado na NF-e (evento 110150) ────────────────────────────────

// XCondUsoAtorInteressado é o texto exigido quando tpAutorizacao=1 (rejeição 829).
const XCondUsoAtorInteressado = "O emitente ou destinatario da NF-e, declara que permite o transportador declarado no campo CNPJ/CPF deste evento a autorizar os transportadores subcontratados ou redespachados a terem acesso ao download da NF-e"

// AtorInteressado autoriza um transportador (CNPJ ou CPF) a baixar o XML da NF-e (modelo 55)
// pela distribuição DF-e (MOC 5.14, NT 2020.007). Autor = emitente (tpAutor=1).
// permiteSubcontratar = tpAutorizacao 1: o transportador pode autorizar subcontratados/redespacho.
// nSeqEvento 1-20 (594), um por pessoa autorizada; o mesmo CNPJ/CPF de novo é 423.
func (cl *Cliente) AtorInteressado(ctx context.Context, chave, cnpj, cpf string, permiteSubcontratar bool, nSeqEvento int, c *cert.Certificado) (*RetornoEvento, error) {
	if len(chave) != 44 || chave[20:22] != "55" {
		return nil, fmt.Errorf("sefaz: ator interessado é só de NF-e (modelo 55)")
	}
	if nSeqEvento < 1 || nSeqEvento > 20 {
		return nil, fmt.Errorf("sefaz: nSeqEvento do ator interessado deve ser de 1 a 20 (é %d)", nSeqEvento)
	}
	var aut string
	switch {
	case len(cnpj) == 14 && cpf == "":
		aut = `<CNPJ>` + escaparXML(cnpj) + `</CNPJ>`
	case len(cpf) == 11 && cnpj == "":
		aut = `<CPF>` + escaparXML(cpf) + `</CPF>`
	default:
		return nil, fmt.Errorf("sefaz: informe o CNPJ (14) ou o CPF (11) do transportador, um só")
	}
	tpAut, cond := "0", ""
	if permiteSubcontratar {
		tpAut, cond = "1", `<xCondUso>`+XCondUsoAtorInteressado+`</xCondUso>`
	}
	det := `<detEvento versao="1.00">` +
		`<descEvento>Ator interessado na NF-e</descEvento>` +
		`<cOrgaoAutor>` + chave[:2] + `</cOrgaoAutor>` +
		`<tpAutor>1</tpAutor>` +
		`<verAplic>` + verAplicEvento + `</verAplic>` +
		`<autXML>` + aut + `</autXML>` +
		`<tpAutorizacao>` + tpAut + `</tpAutorizacao>` + cond +
		`</detEvento>`
	return cl.enviarEvento(ctx, c, chave, "110150", nSeqEvento, det)
}

// enviarEvento monta o <evento> do emitente com o detEvento dado, assina, envia num lote de
// 1 e lê o retEvento.
func (cl *Cliente) enviarEvento(ctx context.Context, c *cert.Certificado, chave, tpEvento string, nSeq int, detEvento string) (*RetornoEvento, error) {
	xmlEvento := fmt.Sprintf(
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infEvento Id="ID%s%s%02d">`+
			`<cOrgao>%s</cOrgao>`+
			`<tpAmb>%s</tpAmb>`+
			`<CNPJ>%s</CNPJ>`+
			`<chNFe>%s</chNFe>`+
			`<dhEvento>%s</dhEvento>`+
			`<tpEvento>%s</tpEvento>`+
			`<nSeqEvento>%d</nSeqEvento>`+
			`<verEvento>1.00</verEvento>`+
			`%s`+
			`</infEvento>`+
			`</evento>`,
		tpEvento, chave, nSeq, cl.cuf, string(cl.amb), cnpjAutorEvento(chave, c),
		chave, agoraBrasilia(), tpEvento, nSeq, detEvento,
	)
	eventoAssinado, err := sign.AssinarEvento([]byte(xmlEvento), c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: assinar evento %s: %w", tpEvento, err)
	}
	soapBody := fmt.Sprintf(
		`<nfeRecepcaoEvento xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4">`+
			`<nfeDadosMsg>`+
			`<envEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<idLote>%d</idLote>`+
			`%s`+
			`</envEvento>`+
			`</nfeDadosMsg>`+
			`</nfeRecepcaoEvento>`,
		time.Now().UnixMilli(), string(eventoAssinado),
	)
	respBody, err := cl.chamar(ctx, ServicoRecepcaoEvento, soapBody)
	if err != nil {
		return nil, err
	}
	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}
	var result struct {
		Ret struct {
			RetornoSEFAZ
			InfEvento struct {
				ChNFe   string `xml:"chNFe"`
				NProt   string `xml:"nProt"`
				CStat   string `xml:"cStat"`
				XMotivo string `xml:"xMotivo"`
			} `xml:"retEvento>infEvento"`
		} `xml:"retEnvEvento"`
	}
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retEnvEvento %s: %w", tpEvento, err)
	}
	r := result.Ret
	return &RetornoEvento{
		RetornoSEFAZ: r.RetornoSEFAZ, ChNFe: r.InfEvento.ChNFe, NProt: r.InfEvento.NProt,
		CStatEvento: r.InfEvento.CStat, XMotivoEvento: r.InfEvento.XMotivo,
		XMLProcEvento: procEventoDaResposta(eventoAssinado, inner),
	}, nil
}

func escaparXML(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
