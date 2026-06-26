package sefaz

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// ── Consulta de protocolo ─────────────────────────────────────────────────────

// RetornoConsultaProtocolo é o retorno do NFeConsultaProtocolo.
type RetornoConsultaProtocolo struct {
	RetornoSEFAZ
	ChNFe   string `xml:"chNFe"`
	NProt   string `xml:"nProt"`
	DhRecbto string `xml:"dhRecbto"`
	// XMLNFeProc: o XML completo da nfeProc retornado pela SEFAZ, se autorizada.
	XMLNFeProc []byte
}

// ConsultarProtocolo consulta a situação de uma NF-e pela chave de acesso (44 dígitos).
func (cl *Cliente) ConsultarProtocolo(ctx context.Context, chave string) (*RetornoConsultaProtocolo, error) {
	soapBody := fmt.Sprintf(
		`<nfeConsultaNF xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeConsultaProtocolo4">`+
			`<nfeDadosMsg>`+
			`<consSitNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<tpAmb>%s</tpAmb>`+
			`<xServ>CONSULTAR</xServ>`+
			`<chNFe>%s</chNFe>`+
			`</consSitNFe>`+
			`</nfeDadosMsg>`+
			`</nfeConsultaNF>`,
		string(cl.amb), chave,
	)

	respBody, err := cl.chamar(ctx, ServicoConsultaProtocolo, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlInfProt struct {
		ChNFe    string `xml:"chNFe"`
		NProt    string `xml:"nProt"`
		DhRecbto string `xml:"dhRecbto"`
		CStat    string `xml:"cStat"`
		XMotivo  string `xml:"xMotivo"`
	}
	type xmlRetCons struct {
		RetornoSEFAZ
		InfProt xmlInfProt `xml:"protNFe>infProt"`
	}
	type xmlResult struct {
		Ret     xmlRetCons `xml:"retConsSitNFe"`
		NFeProc []byte     `xml:",innerxml"` // para capturar nfeProc se presente
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retConsSitNFe: %w", err)
	}

	return &RetornoConsultaProtocolo{
		RetornoSEFAZ: result.Ret.RetornoSEFAZ,
		ChNFe:        result.Ret.InfProt.ChNFe,
		NProt:        result.Ret.InfProt.NProt,
		DhRecbto:     result.Ret.InfProt.DhRecbto,
	}, nil
}

// ── Cancelamento (evento 110111) ──────────────────────────────────────────────

// RetornoCancelamento é o retorno do NFeRecepcaoEvento para cancelamento.
type RetornoCancelamento struct {
	RetornoSEFAZ
	ChNFe string
	NProt string
}

// Cancelar cancela uma NF-e autorizada.
// chave: chave de acesso (44 dígitos).
// nProt: número do protocolo de autorização.
// justificativa: texto de 15-255 caracteres.
func (cl *Cliente) Cancelar(ctx context.Context, chave, nProt, justificativa string, c *cert.Certificado) (*RetornoCancelamento, error) {
	if len(justificativa) < 15 || len(justificativa) > 255 {
		return nil, fmt.Errorf("sefaz: justificativa deve ter entre 15 e 255 caracteres (tem %d)", len(justificativa))
	}

	dhEvento := time.Now().UTC().Format("2006-01-02T15:04:05-03:00")
	idEvento := fmt.Sprintf("ID110111%s01", chave)

	// XML do evento antes de assinar
	xmlEvento := fmt.Sprintf(
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infEvento Id="%s">`+
			`<cOrgao>%s</cOrgao>`+
			`<tpAmb>%s</tpAmb>`+
			`<CNPJ>%s</CNPJ>`+
			`<chNFe>%s</chNFe>`+
			`<dhEvento>%s</dhEvento>`+
			`<tpEvento>110111</tpEvento>`+
			`<nSeqEvento>1</nSeqEvento>`+
			`<verEvento>1.00</verEvento>`+
			`<detEvento versao="1.00">`+
			`<descEvento>Cancelamento</descEvento>`+
			`<nProt>%s</nProt>`+
			`<xJust>%s</xJust>`+
			`</detEvento>`+
			`</infEvento>`+
			`</evento>`,
		idEvento, cl.cuf, string(cl.amb), c.CNPJ(),
		chave, dhEvento, nProt, justificativa,
	)

	eventoAssinado, err := sign.AssinarEvento([]byte(xmlEvento), c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: assinar evento: %w", err)
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

	type xmlInfEvento struct {
		ChNFe   string `xml:"chNFe"`
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
		return nil, fmt.Errorf("sefaz: parse retEnvEvento: %w", err)
	}

	ret := result.Ret
	return &RetornoCancelamento{
		RetornoSEFAZ: ret.RetornoSEFAZ,
		ChNFe:        ret.InfEvento.ChNFe,
		NProt:        ret.InfEvento.NProt,
	}, nil
}

// ── Carta de Correção Eletrônica (evento 110110) ──────────────────────────────

// XCondUsoCartaCorrecao é o texto obrigatório de condição de uso da CC-e conforme NT.
const XCondUsoCartaCorrecao = "A Carta de Correcao e disciplinada pelo paragrafo 1o-A do art. 7o do Convenio S/N de 15 de dezembro de 1970 e pode ser utilizada para regularizacao de erro ocorrido na emissao de documento fiscal, desde que o erro nao esteja relacionado com: I - as variaveis que determinam o valor do imposto tais como: base de calculo, aliquota, diferenca de preco, quantidade, valor da operacao ou da prestacao; II - a correcao de dados cadastrais que implique mudanca do remetente ou do destinatario; III - a data de emissao ou de saida."

// CartaCorrecao envia uma Carta de Correção Eletrônica (CC-e, tpEvento=110110).
// chNFe: chave de acesso (44 dígitos) — os 2 primeiros dígitos determinam a UF roteadora.
// xCorrecao: descrição da correção (15-1000 caracteres).
// xCondUso: condição de uso — usar XCondUsoCartaCorrecao.
// nSeqEvento: sequencial da CC-e para esta NF-e (1-20).
// Retorna o nProt do protocolo de registro ou erro.
func CartaCorrecao(c *cert.Certificado, chNFe, xCorrecao, xCondUso string, nSeqEvento int, amb Ambiente) (protocolo string, err error) {
	if len(xCorrecao) < 15 {
		return "", fmt.Errorf("sefaz: xCorrecao deve ter no mínimo 15 caracteres (tem %d)", len(xCorrecao))
	}
	if len(xCorrecao) > 1000 {
		return "", fmt.Errorf("sefaz: xCorrecao deve ter no máximo 1000 caracteres (tem %d)", len(xCorrecao))
	}
	if len(chNFe) != 44 {
		return "", fmt.Errorf("sefaz: chNFe deve ter 44 dígitos (tem %d)", len(chNFe))
	}

	cuf := chNFe[:2]
	cl, clErr := NovoCliente(cuf, amb, c)
	if clErr != nil {
		return "", fmt.Errorf("sefaz: CartaCorrecao: criar cliente: %w", clErr)
	}

	dhEvento := time.Now().UTC().Format("2006-01-02T15:04:05-03:00")
	idEvento := fmt.Sprintf("ID110110%s%02d", chNFe, nSeqEvento)

	xmlEvento := fmt.Sprintf(
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infEvento Id="%s">`+
			`<cOrgao>%s</cOrgao>`+
			`<tpAmb>%s</tpAmb>`+
			`<CNPJ>%s</CNPJ>`+
			`<chNFe>%s</chNFe>`+
			`<dhEvento>%s</dhEvento>`+
			`<tpEvento>110110</tpEvento>`+
			`<nSeqEvento>%d</nSeqEvento>`+
			`<verEvento>1.00</verEvento>`+
			`<detEvento versao="1.00">`+
			`<descEvento>Carta de Correcao</descEvento>`+
			`<xCorrecao>%s</xCorrecao>`+
			`<xCondUso>%s</xCondUso>`+
			`</detEvento>`+
			`</infEvento>`+
			`</evento>`,
		idEvento, cuf, string(amb), c.CNPJ(),
		chNFe, dhEvento, nSeqEvento, xCorrecao, xCondUso,
	)

	eventoAssinado, err := sign.AssinarEvento([]byte(xmlEvento), c)
	if err != nil {
		return "", fmt.Errorf("sefaz: assinar CC-e: %w", err)
	}

	ctx := context.Background()
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
		return "", err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("sefaz: parse retEnvEvento CC-e: %w", err)
	}

	return result.Ret.InfEvento.NProt, nil
}

// ── Manifestação do destinatário (eventos 210210 / 210200 / 210220 / 210240) ──

// RetornoManifestacao é o retorno do NFeRecepcaoEvento para manifestação do destinatário.
type RetornoManifestacao struct {
	RetornoSEFAZ
	ChNFe string
	NProt string
}

// tiposManifestacao mapeia o identificador curto para tpEvento + descEvento.
var tiposManifestacao = map[string]struct {
	TpEvento  string
	Desc      string
	ExigeJust bool
}{
	"ciencia":          {"210210", "Ciência da Operação", false},
	"confirmacao":      {"210200", "Confirmação da Operação", false},
	"desconhecimento":  {"210220", "Desconhecimento da Operação", false},
	"nao_realizada":    {"210240", "Operação não Realizada", true},
}

// Manifestar registra uma manifestação do destinatário para uma NF-e recebida.
// cnpj: CNPJ do destinatário (só dígitos).
// chave: chave de acesso (44 dígitos).
// tipo: "ciencia" | "confirmacao" | "desconhecimento" | "nao_realizada".
// justificativa: obrigatório para "nao_realizada" (15-255 chars), ignorado nos demais.
func (cl *Cliente) Manifestar(ctx context.Context, cnpj, chave, tipo, justificativa string, c *cert.Certificado) (*RetornoManifestacao, error) {
	cfg, ok := tiposManifestacao[tipo]
	if !ok {
		return nil, fmt.Errorf("sefaz: tipo de manifestação inválido: %q (use ciencia|confirmacao|desconhecimento|nao_realizada)", tipo)
	}
	if cfg.ExigeJust && (len(justificativa) < 15 || len(justificativa) > 255) {
		return nil, fmt.Errorf("sefaz: justificativa deve ter entre 15 e 255 caracteres para nao_realizada (tem %d)", len(justificativa))
	}

	dhEvento := time.Now().UTC().Format("2006-01-02T15:04:05-03:00")
	idEvento := fmt.Sprintf("ID%s%s01", cfg.TpEvento, chave)

	detEvento := fmt.Sprintf(`<detEvento versao="1.00"><descEvento>%s</descEvento>`, cfg.Desc)
	if cfg.ExigeJust {
		detEvento += fmt.Sprintf(`<xJust>%s</xJust>`, justificativa)
	}
	detEvento += `</detEvento>`

	xmlEvento := fmt.Sprintf(
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infEvento Id="%s">`+
			`<cOrgao>91</cOrgao>`+
			`<tpAmb>%s</tpAmb>`+
			`<CNPJ>%s</CNPJ>`+
			`<chNFe>%s</chNFe>`+
			`<dhEvento>%s</dhEvento>`+
			`<tpEvento>%s</tpEvento>`+
			`<nSeqEvento>1</nSeqEvento>`+
			`<verEvento>1.00</verEvento>`+
			`%s`+
			`</infEvento>`+
			`</evento>`,
		idEvento, string(cl.amb), cnpj, chave, dhEvento, cfg.TpEvento, detEvento,
	)

	eventoAssinado, err := sign.AssinarEvento([]byte(xmlEvento), c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: assinar evento manifestação: %w", err)
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

	type xmlInfEvento struct {
		ChNFe   string `xml:"chNFe"`
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
		return nil, fmt.Errorf("sefaz: parse retEnvEvento manifestação: %w", err)
	}

	ret := result.Ret
	return &RetornoManifestacao{
		RetornoSEFAZ: ret.RetornoSEFAZ,
		ChNFe:        ret.InfEvento.ChNFe,
		NProt:        ret.InfEvento.NProt,
	}, nil
}

// ── Inutilização ──────────────────────────────────────────────────────────────

// RetornoInutilizacao é o retorno do NFeInutilizacao.
type RetornoInutilizacao struct {
	RetornoSEFAZ
	NProt string
}

// ResultadoInutilizacao contém o resultado de uma faixa individual em InutilizarLote.
type ResultadoInutilizacao struct {
	NIni      int
	NFin      int
	Protocolo string
	Erro      error
}

// inutilizarFaixa executa a inutilização de uma única faixa com ano explícito.
// Compartilhado por Inutilizar (que computa ano do relógio) e InutilizarLote.
func (cl *Cliente) inutilizarFaixa(ctx context.Context, c *cert.Certificado, cnpj, ano, serie, nNFIni, nNFFin, justificativa string) (*RetornoInutilizacao, error) {
	if len(justificativa) < 15 || len(justificativa) > 255 {
		return nil, fmt.Errorf("sefaz: justificativa deve ter entre 15 e 255 caracteres (tem %d)", len(justificativa))
	}

	idInut := fmt.Sprintf("ID%s%s%s55%s%s%s",
		cl.cuf, cnpj, ano, serie,
		fmt.Sprintf("%09s", nNFIni), fmt.Sprintf("%09s", nNFFin),
	)

	xmlInut := fmt.Sprintf(
		`<inutNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infInut Id="%s">`+
			`<tpAmb>%s</tpAmb>`+
			`<xServ>INUTILIZAR</xServ>`+
			`<cUF>%s</cUF>`+
			`<ano>%s</ano>`+
			`<CNPJ>%s</CNPJ>`+
			`<mod>55</mod>`+
			`<serie>%s</serie>`+
			`<nNFIni>%s</nNFIni>`+
			`<nNFFin>%s</nNFFin>`+
			`<xJust>%s</xJust>`+
			`</infInut>`+
			`</inutNFe>`,
		idInut, string(cl.amb), cl.cuf, ano, cnpj,
		serie, nNFIni, nNFFin, justificativa,
	)

	inutAssinado, err := sign.AssinarInutilizacao([]byte(xmlInut), c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: assinar inutilização: %w", err)
	}

	soapBody := fmt.Sprintf(
		`<nfeInutilizacaoNF xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeInutilizacao4">`+
			`<nfeDadosMsg>%s</nfeDadosMsg>`+
			`</nfeInutilizacaoNF>`,
		string(inutAssinado),
	)

	respBody, err := cl.chamar(ctx, ServicoInutilizacao, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlInfInut struct {
		NProt   string `xml:"nProt"`
		CStat   string `xml:"cStat"`
		XMotivo string `xml:"xMotivo"`
	}
	type xmlRetInut struct {
		RetornoSEFAZ
		InfInut xmlInfInut `xml:"infInut"`
	}
	type xmlResult struct {
		Ret xmlRetInut `xml:"retInutNFe"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retInutNFe: %w", err)
	}

	return &RetornoInutilizacao{
		RetornoSEFAZ: result.Ret.RetornoSEFAZ,
		NProt:        result.Ret.InfInut.NProt,
	}, nil
}

// Inutilizar inutiliza uma faixa de números de NF-e (ano-calendário corrente).
// serie: série (ex: "1"), nNFIni/nNFFin: faixa numérica (ex: "1", "10").
// justificativa: 15-255 caracteres.
func (cl *Cliente) Inutilizar(ctx context.Context, cnpj, serie, nNFIni, nNFFin, justificativa string, c *cert.Certificado) (*RetornoInutilizacao, error) {
	ano := fmt.Sprintf("%02d", time.Now().Year()%100)
	return cl.inutilizarFaixa(ctx, c, cnpj, ano, serie, nNFIni, nNFFin, justificativa)
}

// InutilizarLote inutiliza múltiplas faixas de numeração sequencialmente, sem abortar no primeiro erro.
// cuf: código IBGE da UF emitente (2 dígitos, ex: "52" para GO). cnpj: 14 dígitos do emitente.
// ano: 2 dígitos do ano-calendário (ex: "26" para 2026).
// Cada índice i representa a faixa [nIniList[i], nFinList[i]] com justificativa xJustList[i].
func InutilizarLote(c *cert.Certificado, cuf, cnpj, ano, serie string, nIniList []int, nFinList []int, xJustList []string, amb Ambiente) ([]ResultadoInutilizacao, error) {
	if len(nIniList) != len(nFinList) || len(nIniList) != len(xJustList) {
		return nil, fmt.Errorf("sefaz: InutilizarLote: nIniList(%d), nFinList(%d) e xJustList(%d) devem ter o mesmo tamanho",
			len(nIniList), len(nFinList), len(xJustList))
	}

	cl, err := NovoCliente(cuf, amb, c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: InutilizarLote: criar cliente: %w", err)
	}

	ctx := context.Background()
	resultados := make([]ResultadoInutilizacao, len(nIniList))
	for i := range nIniList {
		r := ResultadoInutilizacao{NIni: nIniList[i], NFin: nFinList[i]}
		ret, err := cl.inutilizarFaixa(ctx, c, cnpj, ano, serie,
			fmt.Sprintf("%d", nIniList[i]),
			fmt.Sprintf("%d", nFinList[i]),
			xJustList[i],
		)
		if err != nil {
			r.Erro = err
		} else {
			r.Protocolo = ret.NProt
		}
		resultados[i] = r
	}
	return resultados, nil
}

// ── Status do Serviço ─────────────────────────────────────────────────────────

// RetornoStatusServico é o retorno do NFeStatusServico.
type RetornoStatusServico struct {
	RetornoSEFAZ
	// DhRetorno: previsão de retorno em caso de indisponibilidade.
	DhRetorno string `xml:"dhRetorno"`
	// TMed: tempo médio de resposta em segundos.
	TMed string `xml:"tMed"`
}

// StatusServico consulta a disponibilidade do webservice SEFAZ.
func (cl *Cliente) StatusServico(ctx context.Context) (*RetornoStatusServico, error) {
	soapBody := fmt.Sprintf(
		`<nfeStatusServicoNF xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4">`+
			`<nfeDadosMsg>`+
			`<consStatServ versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<tpAmb>%s</tpAmb>`+
			`<cUF>%s</cUF>`+
			`<xServ>STATUS</xServ>`+
			`</consStatServ>`+
			`</nfeDadosMsg>`+
			`</nfeStatusServicoNF>`,
		string(cl.amb), cl.cuf,
	)

	respBody, err := cl.chamar(ctx, ServicoStatusServico, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlRetStat struct {
		RetornoSEFAZ
		DhRetorno string `xml:"dhRetorno"`
		TMed      string `xml:"tMed"`
	}
	type xmlResult struct {
		Ret xmlRetStat `xml:"retConsStatServ"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retConsStatServ: %w", err)
	}

	return &RetornoStatusServico{
		RetornoSEFAZ: result.Ret.RetornoSEFAZ,
		DhRetorno:    result.Ret.DhRetorno,
		TMed:         result.Ret.TMed,
	}, nil
}
