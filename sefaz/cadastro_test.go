package sefaz_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

// cadastroTransport devolve uma resposta fixa de retConsCad e guarda o corpo
// enviado, pra checar request e parse na mesma volta.
type cadastroTransport struct {
	corpoEnviado []byte
	resposta     string
}

func (c *cadastroTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	c.corpoEnviado = b
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(c.resposta)),
		Header:     make(http.Header),
	}, nil
}

func envelopeConsCad(corpo string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">` +
		`<soap12:Body><nfeResultMsg>` + corpo + `</nfeResultMsg></soap12:Body></soap12:Envelope>`
}

// Resposta com o formato real devolvido pela SEFAZ-GO (conferido contra o
// servidor de homologação e produção em 15/08/2026 para o CNPJ da Business
// Consulting Assessoria).
const retConsCadHabilitado = `<retConsCad versao="2.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
	`<infCons><verAplic>GO4.0</verAplic><cStat>111</cStat>` +
	`<xMotivo>Consulta cadastro com uma ocorrência</xMotivo>` +
	`<UF>GO</UF><CNPJ>34152609000106</CNPJ>` +
	`<dhCons>2026-08-15T12:00:00-03:00</dhCons><cUF>52</cUF>` +
	`<infCad><IE>202114139</IE><CNPJ>34152609000106</CNPJ><UF>GO</UF>` +
	`<cSit>1</cSit><indCredNFe>4</indCredNFe><indCredCTe>4</indCredCTe>` +
	`<xNome>BUSINESS CONSULTING ASSESSORIA E CONSULTORIA CONTÁBIL LTDA</xNome>` +
	`</infCad></infCons></retConsCad>`

func TestConsultarCadastro_HabilitadoPodeEmitir(t *testing.T) {
	mock := &cadastroTransport{resposta: envelopeConsCad(retConsCadHabilitado)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	ret, err := cl.ConsultarCadastro(context.Background(), "GO", "34.152.609/0001-06")
	if err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}

	// CNPJ com pontuação tem que chegar limpo e no campo certo — o schema
	// escolhe entre IE/CNPJ/CPF, então mandar formatado quebra a validação.
	enviado := string(mock.corpoEnviado)
	if !strings.Contains(enviado, "<CNPJ>34152609000106</CNPJ>") {
		t.Errorf("CNPJ não foi normalizado no request: %s", enviado)
	}
	if !strings.Contains(enviado, "<xServ>CONS-CAD</xServ>") {
		t.Errorf("xServ ausente no request: %s", enviado)
	}

	if ret.CStat != "111" {
		t.Errorf("cStat = %q, esperava 111", ret.CStat)
	}
	if len(ret.Cadastros) != 1 {
		t.Fatalf("esperava 1 cadastro, veio %d", len(ret.Cadastros))
	}
	cad := ret.Cadastros[0]
	if cad.IE != "202114139" {
		t.Errorf("IE = %q", cad.IE)
	}
	if !cad.Habilitado() {
		t.Error("cSit=1 deveria ser habilitado")
	}
	// indCredNFe=4 é "a SEFAZ não fornece a informação" — é o que GO devolve
	// para todo mundo. Tratar isso como impedimento bloquearia empresa apta.
	if !cad.PodeEmitirNFe() {
		t.Error("cSit=1 + indCredNFe=4 deveria poder emitir")
	}
}

func TestConsultarCadastro_NaoHabilitadoNaoEmite(t *testing.T) {
	corpo := strings.Replace(retConsCadHabilitado, "<cSit>1</cSit>", "<cSit>0</cSit>", 1)
	mock := &cadastroTransport{resposta: envelopeConsCad(corpo)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	ret, err := cl.ConsultarCadastro(context.Background(), "GO", "34152609000106")
	if err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}
	if ret.Cadastros[0].PodeEmitirNFe() {
		t.Error("cSit=0 (inscrição baixada) não pode emitir")
	}
}

// Caso real da Business Consulting Soluções Integradas: a SEFAZ nem devolve
// infCad, responde que o CNPJ não é contribuinte na UF. Sem infCad a lista tem
// que vir vazia em vez de estourar.
func TestConsultarCadastro_CNPJNaoContribuinte(t *testing.T) {
	corpo := `<retConsCad versao="2.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<infCons><verAplic>GO4.0</verAplic><cStat>259</cStat>` +
		`<xMotivo>Rejeição: CNPJ da consulta não cadastrado como contribuinte na UF</xMotivo>` +
		`<UF>GO</UF><CNPJ>27338907000111</CNPJ>` +
		`<dhCons>2026-08-15T12:00:00-03:00</dhCons><cUF>52</cUF>` +
		`</infCons></retConsCad>`
	mock := &cadastroTransport{resposta: envelopeConsCad(corpo)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	ret, err := cl.ConsultarCadastro(context.Background(), "GO", "27338907000111")
	if err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}
	if ret.CStat != "259" {
		t.Errorf("cStat = %q, esperava 259", ret.CStat)
	}
	if len(ret.Cadastros) != 0 {
		t.Errorf("esperava lista vazia, veio %d", len(ret.Cadastros))
	}
}

func TestConsultarCadastro_IEUsaCampoIE(t *testing.T) {
	mock := &cadastroTransport{resposta: envelopeConsCad(retConsCadHabilitado)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	if _, err := cl.ConsultarCadastro(context.Background(), "go", "202114139"); err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}
	enviado := string(mock.corpoEnviado)
	if !strings.Contains(enviado, "<IE>202114139</IE>") {
		t.Errorf("esperava consulta por IE: %s", enviado)
	}
	// UF minúscula no chamador não pode virar UF minúscula no XML: TUfCons só
	// aceita as siglas maiúsculas.
	if !strings.Contains(enviado, "<UF>GO</UF>") {
		t.Errorf("UF não foi normalizada: %s", enviado)
	}
}

// Resposta com o grupo <ender>, que a SEFAZ preenche em parte das UFs — é de
// onde sai o endereço (e o código IBGE do município) pra preencher cadastro
// de cliente sem digitação.
const retConsCadComEndereco = `<retConsCad versao="2.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
	`<infCons><verAplic>GO4.0</verAplic><cStat>111</cStat>` +
	`<xMotivo>Consulta cadastro com uma ocorrência</xMotivo>` +
	`<UF>GO</UF><CNPJ>64007210000194</CNPJ>` +
	`<dhCons>2026-09-18T12:00:00-03:00</dhCons><cUF>52</cUF>` +
	`<infCad><IE>203500601</IE><CNPJ>64007210000194</CNPJ><UF>GO</UF>` +
	`<cSit>1</cSit><indCredNFe>1</indCredNFe><indCredCTe>4</indCredCTe>` +
	`<xNome>JN LATAS DISTRIBUIDORA LTDA</xNome><xFant>JN LATAS</xFant>` +
	`<ender><xLgr>AV 24 DE OUTUBRO</xLgr><nro>468</nro><xCpl>QUADRA P-89 LOTE 53</xCpl>` +
	`<xBairro>SETOR DOS FUNCIONARIOS</xBairro><cMun>5208707</cMun><xMun>GOIANIA</xMun>` +
	`<CEP>74543100</CEP></ender>` +
	`</infCad></infCons></retConsCad>`

func TestConsultarCadastro_ExtraiEndereco(t *testing.T) {
	mock := &cadastroTransport{resposta: envelopeConsCad(retConsCadComEndereco)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	ret, err := cl.ConsultarCadastro(context.Background(), "GO", "64007210000194")
	if err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}
	if len(ret.Cadastros) != 1 {
		t.Fatalf("esperava 1 cadastro, veio %d", len(ret.Cadastros))
	}
	e := ret.Cadastros[0].Endereco
	if e.Logradouro != "AV 24 DE OUTUBRO" || e.Numero != "468" {
		t.Errorf("logradouro/numero: %q / %q", e.Logradouro, e.Numero)
	}
	if e.Bairro != "SETOR DOS FUNCIONARIOS" || e.CEP != "74543100" {
		t.Errorf("bairro/CEP: %q / %q", e.Bairro, e.CEP)
	}
	// cMun é o que interessa: é o código IBGE que vai na NF-e.
	if e.CodigoMun != "5208707" || e.Municipio != "GOIANIA" {
		t.Errorf("municipio: %q / %q", e.CodigoMun, e.Municipio)
	}
}

func TestConsultarCadastro_SemEnderecoNaoQuebra(t *testing.T) {
	// Várias UFs devolvem só nome e situação. Endereço vazio é esperado.
	mock := &cadastroTransport{resposta: envelopeConsCad(retConsCadHabilitado)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)

	ret, err := cl.ConsultarCadastro(context.Background(), "GO", "34152609000106")
	if err != nil {
		t.Fatalf("ConsultarCadastro: %v", err)
	}
	if ret.Cadastros[0].Endereco.CodigoMun != "" {
		t.Errorf("esperava endereço vazio, veio %+v", ret.Cadastros[0].Endereco)
	}
}
