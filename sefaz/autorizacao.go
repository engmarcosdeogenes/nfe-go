package sefaz

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"
)

// ── Envio de lote (NFeAutorizacao) ───────────────────────────────────────────

// LoteNFe representa um lote para envio à SEFAZ (máximo 50 NF-e por lote).
type LoteNFe struct {
	// IDLote: número único do lote (até 15 dígitos). Gerado pelo emissor.
	IDLote string
	// IndSinc: 0=assíncrono (recomendado para lotes), 1=síncrono (apenas 1 NF-e)
	IndSinc string
	// NFes: XML completo de cada NF-e (já assinada, sem declaração XML).
	NFes [][]byte
}

// RetornoEnvioLote é o retorno do NFeAutorizacao (envio de lote).
type RetornoEnvioLote struct {
	RetornoSEFAZ
	DhRecbto string `xml:"dhRecbto"` // data/hora recebimento
	NRec     string `xml:"nRec"`     // número do recibo (usar em NFeRetAutorizacao)
}

// EnviarLote envia um lote de NF-e assinadas para autorização.
// Retorna o recibo (nRec) que deve ser usado em ConsultarLote após alguns segundos.
func (cl *Cliente) EnviarLote(ctx context.Context, lote LoteNFe) (*RetornoEnvioLote, error) {
	if len(lote.NFes) == 0 {
		return nil, fmt.Errorf("sefaz: lote vazio")
	}
	if len(lote.NFes) > 50 {
		return nil, fmt.Errorf("sefaz: lote com %d NF-e excede o limite de 50", len(lote.NFes))
	}

	// Monta o XML do lote (cada NF-e sem a declaração <?xml ?> — não pode
	// haver uma segunda declaração no meio do envelope SOAP)
	nfesXML := ""
	for _, nfe := range lote.NFes {
		nfesXML += string(removerDeclaracaoXML(nfe))
	}

	indSinc := lote.IndSinc
	if indSinc == "" {
		if len(lote.NFes) == 1 {
			indSinc = "1" // síncrono quando só há 1 NF-e
		} else {
			indSinc = "0"
		}
	}

	soapBody := fmt.Sprintf(
		`<nfeAutorizacaoLote xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4">`+
			`<nfeDadosMsg>`+
			`<enviNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<idLote>%s</idLote>`+
			`<indSinc>%s</indSinc>`+
			`%s`+
			`</enviNFe>`+
			`</nfeDadosMsg>`+
			`</nfeAutorizacaoLote>`,
		lote.IDLote, indSinc, nfesXML,
	)

	respBody, err := cl.chamar(ctx, ServicoAutorizacao, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	// Estrutura real: <nfeAutorizacaoLoteResult><retEnviNFe ...>...</retEnviNFe></nfeAutorizacaoLoteResult>
	type xmlRetEnvi struct {
		RetornoSEFAZ
		DhRecbto string `xml:"dhRecbto"`
		NRec     string `xml:"nRec"`
	}
	type xmlResult struct {
		Ret xmlRetEnvi `xml:"retEnviNFe"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retEnviNFe: %w", err)
	}

	ret := result.Ret
	return &RetornoEnvioLote{
		RetornoSEFAZ: ret.RetornoSEFAZ,
		DhRecbto:     ret.DhRecbto,
		NRec:         ret.NRec,
	}, nil
}

// ── Consulta de lote (NFeRetAutorizacao) ─────────────────────────────────────

// ProtNFe contém o protocolo de autorização de uma NF-e individual dentro do lote.
type ProtNFe struct {
	ChNFe   string `xml:"chNFe"`   // chave de acesso 44 dígitos
	DhRecbto string `xml:"dhRecbto"`
	NProt   string `xml:"nProt"`   // número do protocolo de autorização
	DigVal  string `xml:"digVal"`  // DigestValue (SHA-1 da NF-e)
	CStat   string `xml:"cStat"`   // 100 = autorizada
	XMotivo string `xml:"xMotivo"`
}

// RetornoConsultaLote é o retorno do NFeRetAutorizacao.
type RetornoConsultaLote struct {
	RetornoSEFAZ
	NRec      string    `xml:"nRec"`
	ProtNFes  []ProtNFe // uma por NF-e no lote
}

// ConsultarLote consulta o resultado de um lote enviado anteriormente.
// nRec: número do recibo retornado por EnviarLote.
func (cl *Cliente) ConsultarLote(ctx context.Context, nRec string) (*RetornoConsultaLote, error) {
	soapBody := fmt.Sprintf(
		`<nfeRetAutorizacaoLote xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeRetAutorizacao4">`+
			`<nfeDadosMsg>`+
			`<consReciNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<tpAmb>%s</tpAmb>`+
			`<nRec>%s</nRec>`+
			`</consReciNFe>`+
			`</nfeDadosMsg>`+
			`</nfeRetAutorizacaoLote>`,
		string(cl.amb), nRec,
	)

	respBody, err := cl.chamar(ctx, ServicoRetAutorizacao, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlProt struct {
		InfProt struct {
			ChNFe    string `xml:"chNFe"`
			DhRecbto string `xml:"dhRecbto"`
			NProt    string `xml:"nProt"`
			DigVal   string `xml:"digVal"`
			CStat    string `xml:"cStat"`
			XMotivo  string `xml:"xMotivo"`
		} `xml:"infProt"`
	}
	type xmlRetCons struct {
		RetornoSEFAZ
		NRec    string    `xml:"nRec"`
		ProtNFe []xmlProt `xml:"protNFe"`
	}
	type xmlResult struct {
		Ret xmlRetCons `xml:"retConsReciNFe"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retConsReciNFe: %w", err)
	}

	prots := make([]ProtNFe, len(result.Ret.ProtNFe))
	for i, p := range result.Ret.ProtNFe {
		prots[i] = ProtNFe{
			ChNFe:    p.InfProt.ChNFe,
			DhRecbto: p.InfProt.DhRecbto,
			NProt:    p.InfProt.NProt,
			DigVal:   p.InfProt.DigVal,
			CStat:    p.InfProt.CStat,
			XMotivo:  p.InfProt.XMotivo,
		}
	}

	return &RetornoConsultaLote{
		RetornoSEFAZ: result.Ret.RetornoSEFAZ,
		NRec:         result.Ret.NRec,
		ProtNFes:     prots,
	}, nil
}

// ── Helper: autorizar com retry automático ────────────────────────────────────

// ResultadoAutorizacao reúne o retorno final de uma autorização.
type ResultadoAutorizacao struct {
	Autorizada bool
	Protocolo  ProtNFe
	// XMLProtocolado é o XML da NF-e com o protocolo embutido (nfeProc), pronto para armazenar.
	XMLProtocolado []byte
}

// Autorizar envia uma única NF-e assinada e aguarda o resultado (com retry automático).
// Espera até maxTentativas × intervaloConsulta antes de retornar erro de timeout.
func (cl *Cliente) Autorizar(ctx context.Context, nfeAssinada []byte, chave string) (*ResultadoAutorizacao, error) {
	idLote := fmt.Sprintf("%d", time.Now().UnixMilli())

	ret, err := cl.EnviarLote(ctx, LoteNFe{
		IDLote:  idLote,
		NFes:    [][]byte{nfeAssinada},
	})
	if err != nil {
		return nil, err
	}

	// cStat 103 = lote recebido, aguardar processamento
	// cStat 104 = lote processado (modo síncrono ou quando já havia resultado)
	if ret.CStat != "103" && ret.CStat != "104" {
		return nil, fmt.Errorf("sefaz: envio recusado cStat=%s: %s", ret.CStat, ret.XMotivo)
	}

	// Consulta com retry
	const maxTentativas = 10
	intervalo := 2 * time.Second

	for i := range maxTentativas {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(intervalo):
		}

		cons, err := cl.ConsultarLote(ctx, ret.NRec)
		if err != nil {
			return nil, err
		}

		// cStat 105 = lote em processamento — aguardar mais
		if cons.CStat == "105" {
			intervalo = min(intervalo*2, 10*time.Second)
			_ = i
			continue
		}

		// Lote processado: localizar protocolo da chave
		for _, p := range cons.ProtNFes {
			if p.ChNFe == chave {
				resultado := &ResultadoAutorizacao{
					Autorizada: p.CStat == "100",
					Protocolo:  p,
				}
				if resultado.Autorizada {
					resultado.XMLProtocolado = montarNFeProc(nfeAssinada, p)
				}
				return resultado, nil
			}
		}

		return nil, fmt.Errorf("sefaz: chave %s não encontrada no retorno do lote", chave)
	}

	return nil, fmt.Errorf("sefaz: timeout aguardando processamento do lote (nRec=%s)", ret.NRec)
}

// montarNFeProc embrulha a NF-e assinada com o protocolo de autorização.
// Formato: <nfeProc versao="4.00" xmlns="..."><NFe>...</NFe><protNFe>...</protNFe></nfeProc>
func montarNFeProc(nfeAssinada []byte, p ProtNFe) []byte {
	prot := fmt.Sprintf(
		`<protNFe versao="4.00"><infProt>`+
			`<tpAmb>%s</tpAmb>`+
			`<verAplic></verAplic>`+
			`<chNFe>%s</chNFe>`+
			`<dhRecbto>%s</dhRecbto>`+
			`<nProt>%s</nProt>`+
			`<digVal>%s</digVal>`+
			`<cStat>%s</cStat>`+
			`<xMotivo>%s</xMotivo>`+
			`</infProt></protNFe>`,
		"", p.ChNFe, p.DhRecbto, p.NProt, p.DigVal, p.CStat, p.XMotivo,
	)
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<nfeProc versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`%s%s`+
			`</nfeProc>`,
		string(removerDeclaracaoXML(nfeAssinada)), prot,
	))
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
