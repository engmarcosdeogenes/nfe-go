package cte

import (
	"fmt"
	"strings"
)

// servicoCTe é um webservice CT-e 4.00 (MOC CT-e 4.00 §4; nome do serviço + método SOAP).
type servicoCTe struct{ nome, metodo string }

var (
	svcRecepcaoSinc   = servicoCTe{"CTeRecepcaoSincV4", "cteRecepcao"}
	svcConsulta       = servicoCTe{"CTeConsultaV4", "cteConsultaCT"}
	svcRecepcaoEvento = servicoCTe{"CTeRecepcaoEventoV4", "cteRecepcaoEvento"}
	svcStatusServico  = servicoCTe{"CTeStatusServicoV4", "cteStatusServicoCT"}
	// Onda 4 (CT-e OS e GTV-e): só na tabela por enquanto.
	svcRecepcaoOS   = servicoCTe{"CTeRecepcaoOSV4", "cteRecepcaoOS"}
	svcRecepcaoGTVe = servicoCTe{"CTeRecepcaoGTVeV4", "cteRecepcaoGTVe"}
)

// todosServicos lista os serviços da tabela (usado pelos testes).
var todosServicos = []servicoCTe{svcRecepcaoSinc, svcConsulta, svcRecepcaoEvento, svcStatusServico, svcRecepcaoOS, svcRecepcaoGTVe}

const nsWSDLBase = "http://www.portalfiscal.inf.br/cte/wsdl/"

// nsWSDL é a escolha ISOLADA do namespace do WSDL (e base da SOAPAction).
// NÃO CONFIRMADO por WSDL (mTLS); o MOC mostra ns sem V4 no exemplo; sped-cte usa
// .../wsdl/<S> com S = nome V4. Confirmar no 1º teste com certificado; para trocar,
// mude só esta função.
func nsWSDL(s servicoCTe) string { return nsWSDLBase + s.nome }

// soapAction = <ns>/<metodo>. Não confirmada (ver nsWSDL).
func (s servicoCTe) soapAction() string { return nsWSDL(s) + "/" + s.metodo }

// autorizador é um ambiente de autorização: hosts por ambiente e padrão de caminho
// (%[1]s = nome do serviço).
type autorizador struct{ nome, prod, homolog, caminho string }

var (
	autSVRS = autorizador{"SVRS", "https://cte.svrs.rs.gov.br", "https://cte-homologacao.svrs.rs.gov.br", "/ws/%[1]s/%[1]s.asmx"}
	autSVSP = autorizador{"SVSP", "https://nfe.fazenda.sp.gov.br", "https://homologacao.nfe.fazenda.sp.gov.br", "/CTeWS/WS/%[1]s.asmx"}
	autMG   = autorizador{"MG", "https://cte.fazenda.mg.gov.br", "https://hcte.fazenda.mg.gov.br", "/cte/services/%[1]s"}
	autMS   = autorizador{"MS", "https://producao.cte.ms.gov.br", "https://homologacao.cte.ms.gov.br", "/ws/%[1]s"}
	autMT   = autorizador{"MT", "https://cte.sefaz.mt.gov.br", "https://homologacao.sefaz.mt.gov.br", "/ctews2/services/%[1]s"}
	autPR   = autorizador{"PR", "https://cte.fazenda.pr.gov.br", "https://homologacao.cte.fazenda.pr.gov.br", "/cte4/%[1]s"}
	// SVC: os hosts de contingência são os mesmos serviços da SVRS (SVC-RS) e da SVSP (SVC-SP)
	// em https://dfe-portal.svrs.rs.gov.br/Cte/Servicos (o portal não os rotula "SVC").
	autSVCRS = autorizador{"SVC-RS", autSVRS.prod, autSVRS.homolog, autSVRS.caminho}
	autSVCSP = autorizador{"SVC-SP", autSVSP.prod, autSVSP.homolog, autSVSP.caminho}
)

// autorizadorUF: pesquisa §3 (portal /Cte/Servicos). SE ausente do aviso do portal;
// SVRS por sped-cte — NÃO CONFIRMADO.
var autorizadorUF = map[string]autorizador{
	"AC": autSVRS, "AL": autSVRS, "AM": autSVRS, "BA": autSVRS, "CE": autSVRS, "DF": autSVRS,
	"ES": autSVRS, "GO": autSVRS, "MA": autSVRS, "PA": autSVRS, "PB": autSVRS, "PI": autSVRS,
	"RJ": autSVRS, "RN": autSVRS, "RO": autSVRS, "RS": autSVRS, "SC": autSVRS, "SE": autSVRS, "TO": autSVRS,
	"AP": autSVSP, "RR": autSVSP, "PE": autSVSP, "SP": autSVSP,
	"MG": autMG, "MS": autMS, "MT": autMT, "PR": autPR,
}

// svcUF: contingência SVC por UF. NÃO CONFIRMADO (a página do portal não diz quais UFs
// cada SVC atende): segue a regra usual — SVC-RS atende UFs de SVSP/próprios; SVC-SP atende UFs da SVRS.
func svcUF(uf string) (autorizador, bool) {
	a, ok := autorizadorUF[uf]
	switch {
	case !ok:
		return autorizador{}, false
	case a.nome == "SVRS":
		return autSVCSP, true
	default:
		return autSVCRS, true
	}
}

func (a autorizador) base(tpAmb string) string {
	if tpAmb == "2" {
		return a.homolog
	}
	return a.prod
}

// url devolve a URL do serviço; base != "" substitui o host (testes).
func (a autorizador) url(s servicoCTe, tpAmb, base string) string {
	if base == "" {
		base = a.base(tpAmb)
	}
	return strings.TrimRight(base, "/") + fmt.Sprintf(a.caminho, s.nome)
}
