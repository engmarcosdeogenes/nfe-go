package sefaz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ── NFeDistribuicaoDFe ────────────────────────────────────────────────────────
// Serviço nacional para download de NF-e e eventos de interesse do contribuinte.
// Permite sincronizar XMLs de notas emitidas e recebidas por um CNPJ, usando
// NSU (Número Sequencial Único) como cursor de paginação.
//
// Fluxo típico de sincronização:
//   ultNSU := carregarDoBank()  // "0" na primeira vez
//   for {
//       ret, err := cli.DistribuirDFe(ctx, cnpj, ultNSU)
//       if err != nil { break }
//       for _, doc := range ret.Docs { processar(doc) }
//       ultNSU = ret.UltNSU
//       salvarNoBanco(ultNSU)
//       if !ret.TemMais() { break }
//   }

// TipoDocDFe identifica o schema do documento retornado.
type TipoDocDFe string

const (
	// DocProcNFe é uma NF-e autorizada completa (nfeProc — emitida pelo CNPJ).
	DocProcNFe TipoDocDFe = "procNFe_v4.00.xsd"
	// DocProcEvento é um evento vinculado a uma NF-e (cancelamento, EPEC, etc.).
	DocProcEvento TipoDocDFe = "procEventoNFe_v1.00.xsd"
	// DocResNFe é um resumo de NF-e recebida (destinatário ainda não manifestou).
	// O XML completo só chega após chamar ConsultarNSUDFe com o NSU deste documento.
	DocResNFe TipoDocDFe = "resNFe_v1.01.xsd"
	// DocResEvento é um resumo de evento recebido.
	DocResEvento TipoDocDFe = "resEvento_v1.01.xsd"
)

// DocDFe representa um documento retornado pelo NFeDistribuicaoDFe.
type DocDFe struct {
	// NSU: Número Sequencial Único (15 dígitos) que identifica o documento na SEFAZ.
	NSU string
	// Schema: tipo do documento (use as constantes DocProcNFe, DocResNFe, etc.).
	Schema TipoDocDFe
	// XML: conteúdo do documento já descodificado de base64 e descomprimido de gzip.
	// Para DocResNFe e DocResEvento, chame ConsultarNSUDFe para obter o XML completo.
	XML []byte
}

// RetornoDistribuicao é o retorno do NFeDistribuicaoDFe.
type RetornoDistribuicao struct {
	RetornoSEFAZ
	// UltNSU: último NSU desta resposta — usar como próximo ultNSU para continuar a paginação.
	UltNSU string
	// MaxNSU: NSU máximo disponível na SEFAZ para este CNPJ.
	MaxNSU string
	// Docs: lista de documentos retornados (vazia quando não há documentos novos).
	Docs []DocDFe
}

// TemMais retorna true se ainda há documentos a buscar (UltNSU < MaxNSU).
func (r *RetornoDistribuicao) TemMais() bool {
	return r.UltNSU != "" && r.MaxNSU != "" && r.UltNSU < r.MaxNSU
}

// DistribuirDFe busca um lote de documentos de interesse do CNPJ a partir de ultNSU.
// Use "0" como ultNSU na primeira chamada. Chame em loop até !ret.TemMais().
// A SEFAZ limita a 50 documentos por chamada.
func (cl *Cliente) DistribuirDFe(ctx context.Context, cnpj, ultNSU string) (*RetornoDistribuicao, error) {
	cnpj = soAlfaNum(cnpj)
	if ultNSU == "" {
		ultNSU = "0"
	}
	corpo := fmt.Sprintf(`<distNSU><ultNSU>%015s</ultNSU></distNSU>`, ultNSU)
	return cl.chamarDistribuicao(ctx, cnpj, corpo)
}

// ConsultarNSUDFe consulta um documento específico pelo seu NSU.
// Útil para obter o XML completo de documentos que chegaram como resumo (DocResNFe / DocResEvento).
func (cl *Cliente) ConsultarNSUDFe(ctx context.Context, cnpj, nsu string) (*RetornoDistribuicao, error) {
	cnpj = soAlfaNum(cnpj)
	corpo := fmt.Sprintf(`<consNSU><NSU>%015s</NSU></consNSU>`, nsu)
	return cl.chamarDistribuicao(ctx, cnpj, corpo)
}

// ConsultarChaveDFe consulta um documento NF-e pela chave de acesso (44 dígitos).
func (cl *Cliente) ConsultarChaveDFe(ctx context.Context, cnpj, chave string) (*RetornoDistribuicao, error) {
	cnpj = soAlfaNum(cnpj)
	chave = soAlfaNum(chave)
	corpo := fmt.Sprintf(`<consChNFe><chNFe>%s</chNFe></consChNFe>`, chave)
	return cl.chamarDistribuicao(ctx, cnpj, corpo)
}

// chamarDistribuicao monta o envelope SOAP e chama o NFeDistribuicaoDFe.
func (cl *Cliente) chamarDistribuicao(ctx context.Context, cnpj, corpoConsulta string) (*RetornoDistribuicao, error) {
	soapBody := fmt.Sprintf(
		`<nfeDistDFeInteresse xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe">`+
			`<nfeDadosMsg>`+
			`<distDFeInt versao="1.01" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<tpAmb>%s</tpAmb>`+
			`<cUFAutor>%s</cUFAutor>`+
			`<CNPJ>%s</CNPJ>`+
			`%s`+
			`</distDFeInt>`+
			`</nfeDadosMsg>`+
			`</nfeDistDFeInteresse>`,
		string(cl.amb), cl.cuf, cnpj, corpoConsulta,
	)

	respBody, err := cl.chamar(ctx, ServicoDistribuicaoDFe, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlDocZip struct {
		NSU      string `xml:"NSU,attr"`
		Schema   string `xml:"schema,attr"`
		Conteudo string `xml:",chardata"`
	}
	type xmlLote struct {
		Docs []xmlDocZip `xml:"docZip"`
	}
	type xmlRet struct {
		RetornoSEFAZ
		UltNSU string  `xml:"ultNSU"`
		MaxNSU string  `xml:"maxNSU"`
		Lote   xmlLote `xml:"loteDistDFeInt"`
	}
	type xmlResult struct {
		Ret xmlRet `xml:"retDistDFeInt"`
	}

	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retDistDFeInt: %w", err)
	}

	ret := result.Ret
	docs := make([]DocDFe, 0, len(ret.Lote.Docs))
	for _, d := range ret.Lote.Docs {
		xmlBytes, err := descomprimirDFe(strings.TrimSpace(d.Conteudo))
		if err != nil {
			// Mantém o documento sem XML ao invés de abortar o lote inteiro
			docs = append(docs, DocDFe{NSU: d.NSU, Schema: TipoDocDFe(d.Schema)})
			continue
		}
		docs = append(docs, DocDFe{
			NSU:    d.NSU,
			Schema: TipoDocDFe(d.Schema),
			XML:    xmlBytes,
		})
	}

	return &RetornoDistribuicao{
		RetornoSEFAZ: ret.RetornoSEFAZ,
		UltNSU:       ret.UltNSU,
		MaxNSU:       ret.MaxNSU,
		Docs:         docs,
	}, nil
}

// descomprimirDFe decodifica base64 e descomprime gzip de um documento DFe retornado pela SEFAZ.
func descomprimirDFe(b64gzip string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(b64gzip)
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

// soAlfaNum remove pontos, traços, barras e espaços de strings de documento (CNPJ, CPF, chave).
func soAlfaNum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
