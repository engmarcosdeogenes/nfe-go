package sefaz

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

// Situação cadastral do contribuinte (campo cSit do retConsCad).
const (
	SituacaoNaoHabilitado = "0"
	SituacaoHabilitado    = "1"
)

// Credenciamento para emissão de NF-e (campo indCredNFe do retConsCad).
const (
	CredNFeNaoCredenciado     = "0"
	CredNFeCredenciado        = "1"
	CredNFeObrigatorioTotal   = "2"
	CredNFeObrigatorioParcial = "3"
	CredNFeSefazNaoInforma    = "4"
)

// RetornoConsultaCadastro é a resposta de ConsultarCadastro.
// A SEFAZ pode devolver mais de um cadastro para o mesmo CNPJ (uma IE por
// estabelecimento), por isso Cadastros é lista.
type RetornoConsultaCadastro struct {
	RetornoSEFAZ
	UF        string
	Cadastros []CadastroContribuinte
}

// CadastroContribuinte é um registro do cadastro estadual (grupo infCad).
// Campos opcionais no schema vêm vazios quando a UF não os informa.
type CadastroContribuinte struct {
	IE   string
	CNPJ string
	CPF  string
	UF   string
	// Situacao é o cSit: "1" habilitado, "0" não habilitado.
	Situacao string
	// IndCredNFe é o indCredNFe: "0" não credenciado, "1" credenciado,
	// "2"/"3" credenciado com obrigatoriedade, "4" a SEFAZ não informa.
	IndCredNFe      string
	IndCredCTe      string
	Nome            string
	NomeFantasia    string
	RegimeApuracao  string
	CNAE            string
	InicioAtividade string
	UltimaSituacao  string
	Baixa           string
}

// Habilitado informa se a inscrição estadual está ativa (cSit=1).
// Inscrição baixada, suspensa ou cancelada devolve false — a SEFAZ rejeita
// emissão de NF-e nesse estado.
func (c CadastroContribuinte) Habilitado() bool {
	return c.Situacao == SituacaoHabilitado
}

// PodeEmitirNFe informa se o contribuinte está apto a emitir NF-e nessa UF:
// inscrição habilitada E credenciada para NF-e.
//
// indCredNFe="4" ("a SEFAZ não fornece a informação") não bloqueia — várias
// UFs devolvem esse valor para todo mundo, então tratá-lo como impedimento
// daria alarme falso. Nesse caso só a situação cadastral pesa.
func (c CadastroContribuinte) PodeEmitirNFe() bool {
	return c.Habilitado() && c.IndCredNFe != CredNFeNaoCredenciado
}

// ConsultarCadastro consulta a situação cadastral de um contribuinte na SEFAZ
// da UF informada (serviço CadConsultaCadastro, leiaute consCad 2.00).
//
// documento é o CNPJ (14 dígitos), CPF (11) ou a inscrição estadual — o tipo é
// deduzido pelo tamanho, como o schema exige a escolha de um dos três.
//
// Serve para checar antes de emitir se a IE está habilitada e credenciada,
// em vez de descobrir pela rejeição da SEFAZ. Não gera documento fiscal, então
// pode ser chamado em produção sem efeito tributário.
func (cl *Cliente) ConsultarCadastro(ctx context.Context, uf, documento string) (*RetornoConsultaCadastro, error) {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	doc := apenasDigitos(documento)
	if doc == "" {
		return nil, fmt.Errorf("sefaz: consultar cadastro: documento vazio")
	}

	var campo string
	switch len(doc) {
	case 14:
		campo = "CNPJ"
	case 11:
		campo = "CPF"
	default:
		campo = "IE"
	}

	soapBody := fmt.Sprintf(
		`<nfeDadosMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/CadConsultaCadastro4">`+
			`<ConsCad versao="2.00" xmlns="http://www.portalfiscal.inf.br/nfe">`+
			`<infCons>`+
			`<xServ>CONS-CAD</xServ>`+
			`<UF>%s</UF>`+
			`<%s>%s</%s>`+
			`</infCons>`+
			`</ConsCad>`+
			`</nfeDadosMsg>`,
		uf, campo, doc, campo,
	)

	respBody, err := cl.chamar(ctx, ServicoConsultaCadastro, soapBody)
	if err != nil {
		return nil, err
	}

	inner, err := extrairEnvelope(respBody)
	if err != nil {
		return nil, err
	}

	type xmlInfCad struct {
		IE         string `xml:"IE"`
		CNPJ       string `xml:"CNPJ"`
		CPF        string `xml:"CPF"`
		UF         string `xml:"UF"`
		CSit       string `xml:"cSit"`
		IndCredNFe string `xml:"indCredNFe"`
		IndCredCTe string `xml:"indCredCTe"`
		XNome      string `xml:"xNome"`
		XFant      string `xml:"xFant"`
		XRegApur   string `xml:"xRegApur"`
		CNAE       string `xml:"CNAE"`
		DIniAtiv   string `xml:"dIniAtiv"`
		DUltSit    string `xml:"dUltSit"`
		DBaixa     string `xml:"dBaixa"`
	}
	type xmlInfCons struct {
		RetornoSEFAZ
		UF     string      `xml:"UF"`
		InfCad []xmlInfCad `xml:"infCad"`
	}
	type xmlResult struct {
		Ret struct {
			InfCons xmlInfCons `xml:"infCons"`
		} `xml:"retConsCad"`
	}
	var result xmlResult
	if err := xml.Unmarshal(inner, &result); err != nil {
		return nil, fmt.Errorf("sefaz: parse retConsCad: %w", err)
	}

	info := result.Ret.InfCons
	saida := &RetornoConsultaCadastro{RetornoSEFAZ: info.RetornoSEFAZ, UF: info.UF}
	for _, c := range info.InfCad {
		saida.Cadastros = append(saida.Cadastros, CadastroContribuinte{
			IE:              c.IE,
			CNPJ:            c.CNPJ,
			CPF:             c.CPF,
			UF:              c.UF,
			Situacao:        c.CSit,
			IndCredNFe:      c.IndCredNFe,
			IndCredCTe:      c.IndCredCTe,
			Nome:            c.XNome,
			NomeFantasia:    c.XFant,
			RegimeApuracao:  c.XRegApur,
			CNAE:            c.CNAE,
			InicioAtividade: c.DIniAtiv,
			UltimaSituacao:  c.DUltSit,
			Baixa:           c.DBaixa,
		})
	}
	return saida, nil
}

func apenasDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
