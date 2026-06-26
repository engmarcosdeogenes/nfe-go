// Package sefaz implementa o cliente SOAP para os webservices da SEFAZ Nacional
// e SVRS (Sefaz Virtual RS), conforme Manual de Integração NF-e v6.00.
//
// Fluxo de autorização (assíncrona):
//  1. NFeAutorizacao (envia lote)
//  2. NFeRetAutorizacao (consulta resultado do lote)
//  3. NFeConsultaProtocolo (consulta nota individual)
//
// Cancelamento: NFeRecepcaoEvento (evento 110111)
// Inutilização: NFeInutilizacao
package sefaz

// Ambiente define se o webservice alvo é produção ou homologação.
type Ambiente string

const (
	Producao    Ambiente = "1"
	Homologacao Ambiente = "2"
)

// String retorna a descrição legível do ambiente.
func (a Ambiente) String() string {
	if a == Producao {
		return "Produção"
	}
	return "Homologação"
}

// Servico identifica o webservice SEFAZ.
type Servico string

const (
	ServicoAutorizacao       Servico = "NFeAutorizacao"
	ServicoRetAutorizacao    Servico = "NFeRetAutorizacao"
	ServicoConsultaProtocolo Servico = "NFeConsultaProtocolo"
	ServicoRecepcaoEvento    Servico = "NFeRecepcaoEvento"
	ServicoInutilizacao      Servico = "NFeInutilizacao"
	ServicoStatusServico     Servico = "NFeStatusServico"
	// ServicoDistribuicaoDFe é o serviço nacional de distribuição de DF-e.
	// Endpoint único para todos os estados — ignora cUF na resolução de URL.
	ServicoDistribuicaoDFe Servico = "NFeDistribuicaoDFe"
)

// ──────────────────────────────────────────────────────────────────────────────
// Tabela de endpoints por UF / ambiente / serviço
//
// Fonte: Manual de Integração NF-e v6.00, Anexo I (2024).
// UFs que usam SVRS (Sefaz Virtual RS): AC, AL, AP, DF, ES, PB, RJ, RN, RO, RR, SC, SE, TO
// UFs com autorizador próprio: AM, BA, GO, MG, MS, MT, PE, PR, RS, SP + SVAN (CE, MA, PA, PI)
// SVAN (Sefaz Virtual Ambiente Nacional): AM usa SVAN para contingência
// ──────────────────────────────────────────────────────────────────────────────

type endpoints struct {
	prod  string
	homol string
}

// svrs = Sefaz Virtual RS (servidor substituto para vários estados)
var svrs = map[Servico]endpoints{
	ServicoAutorizacao:       {"https://nfe.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx", "https://homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx"},
	ServicoRetAutorizacao:    {"https://nfe.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx", "https://homologacao.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx"},
	ServicoConsultaProtocolo: {"https://nfe.svrs.rs.gov.br/ws/NfeConsulta2/NfeConsulta2.asmx", "https://homologacao.svrs.rs.gov.br/ws/NfeConsulta2/NfeConsulta2.asmx"},
	ServicoRecepcaoEvento:    {"https://nfe.svrs.rs.gov.br/ws/recepcaoEvento/recepcaoEvento.asmx", "https://homologacao.svrs.rs.gov.br/ws/recepcaoEvento/recepcaoEvento.asmx"},
	ServicoInutilizacao:      {"https://nfe.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao2.asmx", "https://homologacao.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao2.asmx"},
	ServicoStatusServico:     {"https://nfe.svrs.rs.gov.br/ws/NFeStatusServico/NFeStatusServico4.asmx", "https://homologacao.svrs.rs.gov.br/ws/NFeStatusServico/NFeStatusServico4.asmx"},
}

// svan = Sefaz Virtual Ambiente Nacional (usado por AM, CE, MA, PA, PI como contingência/titular)
var svan = map[Servico]endpoints{
	ServicoAutorizacao:       {"https://www.sefazvirtual.fazenda.gov.br/NFeAutorizacao4/NFeAutorizacao4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeAutorizacao4/NFeAutorizacao4.asmx"},
	ServicoRetAutorizacao:    {"https://www.sefazvirtual.fazenda.gov.br/NFeRetAutorizacao4/NFeRetAutorizacao4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeRetAutorizacao4/NFeRetAutorizacao4.asmx"},
	ServicoConsultaProtocolo: {"https://www.sefazvirtual.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
	ServicoRecepcaoEvento:    {"https://www.sefazvirtual.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx"},
	ServicoInutilizacao:      {"https://www.sefazvirtual.fazenda.gov.br/NFeInutilizacao4/NFeInutilizacao4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeInutilizacao4/NFeInutilizacao4.asmx"},
	ServicoStatusServico:     {"https://www.sefazvirtual.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx", "https://hom.sefazvirtual.fazenda.gov.br/NFeStatusServico4/NFeStatusServico4.asmx"},
}

// endpointsPorUF mapeia cUF (2 dígitos IBGE) para a tabela de endpoints daquele estado.
// Estados sem entrada própria delegam para SVRS (ver ObterURL).
var endpointsPorUF = map[string]map[Servico]endpoints{
	// AM — Amazonas (usa SVAN)
	"13": svan,

	// BA — Bahia
	"29": {
		ServicoAutorizacao:       {"https://nfe.sefaz.ba.gov.br/webservices/NFeAutorizacao4/NFeAutorizacao4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeAutorizacao4/NFeAutorizacao4.asmx"},
		ServicoRetAutorizacao:    {"https://nfe.sefaz.ba.gov.br/webservices/NFeRetAutorizacao4/NFeRetAutorizacao4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeRetAutorizacao4/NFeRetAutorizacao4.asmx"},
		ServicoConsultaProtocolo: {"https://nfe.sefaz.ba.gov.br/webservices/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeConsultaProtocolo4/NFeConsultaProtocolo4.asmx"},
		ServicoRecepcaoEvento:    {"https://nfe.sefaz.ba.gov.br/webservices/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx"},
		ServicoInutilizacao:      {"https://nfe.sefaz.ba.gov.br/webservices/NFeInutilizacao4/NFeInutilizacao4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeInutilizacao4/NFeInutilizacao4.asmx"},
		ServicoStatusServico:     {"https://nfe.sefaz.ba.gov.br/webservices/NFeStatusServico4/NFeStatusServico4.asmx", "https://hnfe.sefaz.ba.gov.br/webservices/NFeStatusServico4/NFeStatusServico4.asmx"},
	},

	// CE, MA, PA, PI — usam SVAN
	"23": svan, // CE
	"21": svan, // MA
	"15": svan, // PA
	"22": svan, // PI

	// GO — Goiás
	"52": {
		ServicoAutorizacao:       {"https://nfe.sefaz.go.gov.br/nfe/services/NFeAutorizacao4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.sefaz.go.gov.br/nfe/services/NFeRetAutorizacao4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.sefaz.go.gov.br/nfe/services/NFeConsultaProtocolo4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeConsultaProtocolo4"},
		ServicoRecepcaoEvento:    {"https://nfe.sefaz.go.gov.br/nfe/services/NFeRecepcaoEvento4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeRecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.sefaz.go.gov.br/nfe/services/NFeInutilizacao4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.sefaz.go.gov.br/nfe/services/NFeStatusServico4", "https://homolog.sefaz.go.gov.br/nfe/services/NFeStatusServico4"},
	},

	// MG — Minas Gerais
	"31": {
		ServicoAutorizacao:       {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeAutorizacao4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeRetAutorizacao4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeConsultaProtocolo4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeConsultaProtocolo4"},
		ServicoRecepcaoEvento:    {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeRecepcaoEvento4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeRecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeInutilizacao4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.fazenda.mg.gov.br/nfe2/services/NFeStatusServico4", "https://hnfe.fazenda.mg.gov.br/nfe2/services/NFeStatusServico4"},
	},

	// MS — Mato Grosso do Sul
	"50": {
		ServicoAutorizacao:       {"https://nfe.fazenda.ms.gov.br/ws/NFeAutorizacao4", "https://homologacao.nfe.ms.gov.br/ws/NFeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.fazenda.ms.gov.br/ws/NFeRetAutorizacao4", "https://homologacao.nfe.ms.gov.br/ws/NFeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.fazenda.ms.gov.br/ws/NFeConsultaProtocolo4", "https://homologacao.nfe.ms.gov.br/ws/NFeConsultaProtocolo4"},
		ServicoRecepcaoEvento:    {"https://nfe.fazenda.ms.gov.br/ws/NFeRecepcaoEvento4", "https://homologacao.nfe.ms.gov.br/ws/NFeRecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.fazenda.ms.gov.br/ws/NFeInutilizacao4", "https://homologacao.nfe.ms.gov.br/ws/NFeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.fazenda.ms.gov.br/ws/NFeStatusServico4", "https://homologacao.nfe.ms.gov.br/ws/NFeStatusServico4"},
	},

	// MT — Mato Grosso
	"51": {
		ServicoAutorizacao:       {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeAutorizacao4", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeRetAutorizacao4", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeConsulta2", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeConsulta2"},
		ServicoRecepcaoEvento:    {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/RecepcaoEvento4", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/RecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeInutilizacao4", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.sefaz.mt.gov.br/nfews/v2/services/NfeStatusServico4", "https://homologacao.sefaz.mt.gov.br/nfews/v2/services/NfeStatusServico4"},
	},

	// PE — Pernambuco
	"26": {
		ServicoAutorizacao:       {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeAutorizacao4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeRetAutorizacao4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeConsultaProtocolo4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeConsultaProtocolo4"},
		ServicoRecepcaoEvento:    {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeRecepcaoEvento4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeRecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeInutilizacao4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.sefaz.pe.gov.br/nfe-service/services/NFeStatusServico4", "https://nfehomolog.sefaz.pe.gov.br/nfe-service/services/NFeStatusServico4"},
	},

	// PR — Paraná
	"41": {
		ServicoAutorizacao:       {"https://nfe.fazenda.pr.gov.br/nfe/NFeAutorizacao4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeAutorizacao4"},
		ServicoRetAutorizacao:    {"https://nfe.fazenda.pr.gov.br/nfe/NFeRetAutorizacao4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeRetAutorizacao4"},
		ServicoConsultaProtocolo: {"https://nfe.fazenda.pr.gov.br/nfe/NFeConsultaProtocolo4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeConsultaProtocolo4"},
		ServicoRecepcaoEvento:    {"https://nfe.fazenda.pr.gov.br/nfe/NFeRecepcaoEvento4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeRecepcaoEvento4"},
		ServicoInutilizacao:      {"https://nfe.fazenda.pr.gov.br/nfe/NFeInutilizacao4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeInutilizacao4"},
		ServicoStatusServico:     {"https://nfe.fazenda.pr.gov.br/nfe/NFeStatusServico4", "https://homologacao.nfe.fazenda.pr.gov.br/nfe/NFeStatusServico4"},
	},

	// RS — Rio Grande do Sul
	"43": {
		ServicoAutorizacao:       {"https://nfe.sefaz.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx"},
		ServicoRetAutorizacao:    {"https://nfe.sefaz.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx"},
		ServicoConsultaProtocolo: {"https://nfe.sefaz.rs.gov.br/ws/NfeConsulta2/NfeConsulta2.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/NfeConsulta2/NfeConsulta2.asmx"},
		ServicoRecepcaoEvento:    {"https://nfe.sefaz.rs.gov.br/ws/recepcaoEvento/recepcaoEvento.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/recepcaoEvento/recepcaoEvento.asmx"},
		ServicoInutilizacao:      {"https://nfe.sefaz.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao2.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao2.asmx"},
		ServicoStatusServico:     {"https://nfe.sefaz.rs.gov.br/ws/NFeStatusServico/NFeStatusServico4.asmx", "https://nfe-homologacao.sefaz.rs.gov.br/ws/NFeStatusServico/NFeStatusServico4.asmx"},
	},

	// SP — São Paulo
	"35": {
		ServicoAutorizacao:       {"https://nfe.fazenda.sp.gov.br/ws/nfeautorizacao4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfeautorizacao4.asmx"},
		ServicoRetAutorizacao:    {"https://nfe.fazenda.sp.gov.br/ws/nferetautorizacao4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nferetautorizacao4.asmx"},
		ServicoConsultaProtocolo: {"https://nfe.fazenda.sp.gov.br/ws/nfeconsultaprotocolo4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfeconsultaprotocolo4.asmx"},
		ServicoRecepcaoEvento:    {"https://nfe.fazenda.sp.gov.br/ws/nferecepcaoevento4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nferecepcaoevento4.asmx"},
		ServicoInutilizacao:      {"https://nfe.fazenda.sp.gov.br/ws/nfeinutilizacao4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfeinutilizacao4.asmx"},
		ServicoStatusServico:     {"https://nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx", "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx"},
	},
}

// ObterURL retorna a URL do webservice para o estado (cUF 2 dígitos), serviço e ambiente.
// Estados não mapeados individualmente usam SVRS como fallback padrão.
// ServicoDistribuicaoDFe é nacional — cUF ignorado, URL fixa por ambiente.
func ObterURL(cUF string, srv Servico, amb Ambiente) string {
	if srv == ServicoDistribuicaoDFe {
		if amb == Producao {
			return "https://www.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
		}
		return "https://hom.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
	}
	tabela, ok := endpointsPorUF[cUF]
	if !ok {
		tabela = svrs
	}
	ep, ok := tabela[srv]
	if !ok {
		return ""
	}
	if amb == Producao {
		return ep.prod
	}
	return ep.homol
}
