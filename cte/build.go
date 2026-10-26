package cte

import (
	"fmt"
	"strings"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Build monta o CT-e (sem assinatura, sem declaração <?xml?>) e a chave.
func Build(e Entrada) (xmlOut []byte, chave string, err error) {
	g := &gen{}
	if e.TpAmb != "1" && e.TpAmb != "2" {
		return nil, "", fmt.Errorf("cte: tpAmb %q inválido (1 ou 2)", e.TpAmb)
	}
	if e.TpEmis == "" {
		e.TpEmis = "1"
	}
	if e.TpImp == "" {
		e.TpImp = "1"
	}
	if e.Modal == "" {
		e.Modal = "01"
	}
	if e.VerProc == "" {
		e.VerProc = "nfe-go"
	}
	switch {
	case e.TpCTe == "2":
		g.fail("tpCTe 2 (anulação) foi extinto no leiaute 4.00; o XSD aceita 0, 1 e 3")
	case !inSet(e.TpCTe, "0", "1", "3"):
		g.fail("tpCTe %q inválido (0 normal, 1 complemento, 3 substituição)", e.TpCTe)
	}
	if !inSet(e.TpServ, "0", "1", "2", "3", "4") {
		g.fail("tpServ %q inválido (0..4)", e.TpServ)
	}
	if !inSet(e.TpImp, "1", "2") {
		g.fail("tpImp %q inválido (1 ou 2)", e.TpImp)
	}
	if !inSet(e.TpEmis, "1", "2", "3", "4", "7", "8") {
		g.fail("tpEmis %q inválido (1, 2, 3, 4, 7 ou 8)", e.TpEmis)
	}
	if !inSet(e.Modal, "01", "02", "03", "04", "05", "06") {
		g.fail("modal %q inválido (01..06)", e.Modal)
	}
	// Contingência: dhCont/xJust vêm juntos e só fora da emissão normal
	// (MOC CT-e 4.00, grupo ide; rejeição específica: não confirmado).
	if e.TpEmis != "1" {
		if e.DhCont == nil || len(e.XJust) < 15 {
			g.fail("tpEmis %s exige dhCont e xJust (15 a 256 caracteres)", e.TpEmis)
		}
	} else if e.DhCont != nil || e.XJust != "" {
		g.fail("dhCont/xJust só existem com tpEmis diferente de 1")
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		g.fail("contingência offline (tpEmis 2) exige QRSign (assinatura RSA-SHA1 da chave, base64)")
	}
	if e.InfSolicNFF != "" && e.TpEmis != "3" {
		g.fail("infSolicNFF só com tpEmis 3 (regime especial NFF)")
	}
	if !reCFOP.MatchString(e.CFOP) {
		g.fail("CFOP %q inválido", e.CFOP)
	}

	em := e.Emitente
	g.uf("UF do emitente", em.End.UF)
	serie := g.inteiro("serie", e.Serie, 0, 999)
	nct := g.inteiro("nCT", e.NCT, 1, 999999999)
	if (em.CNPJ == "") == (em.CPF == "") {
		g.fail("emitente: informe CNPJ ou CPF (um dos dois)")
	}
	idTag, idEmit := g.docID("emitente", firstNonEmpty(em.CNPJ, em.CPF))
	if !inSet(em.CRT, "1", "2", "3", "4") {
		g.fail("CRT %q inválido (1, 2, 3 ou 4)", em.CRT)
	}
	if g.err != nil {
		return nil, "", g.err
	}
	idChave := idEmit
	if idTag == "CPF" {
		idChave = "000" + idEmit // chave tem 14 posições para o documento
	}
	dh := e.DhEmi.In(builder.FusoUF(em.End.UF)) // dhEmi futuro é rejeitado
	ch := builder.NovaChave(em.End.UF, idChave, serie, nct, e.TpEmis, "57", dh)
	if e.CCT != "" {
		ch = comCNF(ch, e.CCT, g)
	}
	chave = ch.String()

	g.raw(`<CTe xmlns="` + nsCTe + `"><infCte versao="4.00" Id="CTe` + chave + `">`)
	g.ide(e, ch, serie, nct, dh)
	g.compl(e.Compl)
	g.emit(em, idTag, idEmit)
	if e.Rem != nil {
		p := e.Rem
		g.parte("rem", "enderReme", "remetente", p.CNPJ, p.CPF, p.IE, p.Nome, p.Fantasia, p.Fone, p.Email, "", p.End, true)
	}
	if e.Exped != nil {
		p := e.Exped
		g.parte("exped", "enderExped", "expedidor", p.CNPJ, p.CPF, p.IE, p.Nome, p.Fantasia, p.Fone, p.Email, "", p.End, false)
	}
	if e.Receb != nil {
		p := e.Receb
		g.parte("receb", "enderReceb", "recebedor", p.CNPJ, p.CPF, p.IE, p.Nome, p.Fantasia, p.Fone, p.Email, "", p.End, false)
	}
	if e.Dest != nil {
		p := e.Dest
		g.parte("dest", "enderDest", "destinatário", p.CNPJ, p.CPF, p.IE, p.Nome, p.Fantasia, p.Fone, p.Email, p.ISUF, p.End, false)
	}
	g.vPrest(e)
	g.imp(e)
	g.pgtoVinc(e.PgtoVinc)
	if e.TpCTe == "1" {
		g.complemento(e)
	} else {
		g.norm(e)
	}
	if len(e.AutXML) > 10 {
		g.fail("autXML: no máximo 10")
	}
	for _, a := range e.AutXML {
		t, v := g.docID("autXML", a)
		g.open("autXML")
		g.el(t, v)
		g.end("autXML")
	}
	if rt := e.RespTec; rt != nil {
		g.open("infRespTec")
		t, v := g.docID("respTec", rt.CNPJ)
		if t != "CNPJ" {
			g.fail("infRespTec exige CNPJ")
		}
		g.el("CNPJ", v)
		g.el("xContato", g.txt("xContato", rt.Contato, 2, 60))
		g.el("email", g.email("email do responsável técnico", rt.Email))
		f := digitos(rt.Fone)
		if !reFoneRT.MatchString(f) {
			g.fail("fone do responsável técnico: 7 a 12 dígitos")
		}
		g.el("fone", f)
		g.end("infRespTec")
	}
	if e.InfSolicNFF != "" {
		g.open("infSolicNFF")
		g.el("xSolic", g.txt("xSolic", e.InfSolicNFF, 2, 8000))
		g.end("infSolicNFF")
	}
	g.end("infCte")

	base := e.URLQRCode
	if base == "" {
		base = baseQR
	}
	qr := urlQR(base, chave, e.TpAmb)
	if e.TpEmis == "2" {
		qr += "&sign=" + e.QRSign
	}
	g.open("infCTeSupl")
	g.el("qrCodCTe", qr)
	g.end("infCTeSupl")
	g.raw("</CTe>")

	if g.err != nil {
		return nil, "", g.err
	}
	return []byte(g.b.String()), chave, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (g *gen) local(nome string, l Local) (c, x, uf string) {
	c, x = g.mun(nome, l.CodigoMun, l.Nome)
	g.uf(nome+" UF", l.UF)
	return c, x, l.UF
}

func (g *gen) ide(e Entrada, ch builder.ChaveAcesso, serie, nct string, dh time.Time) {
	g.open("ide")
	g.el("cUF", ch.CUF)
	g.el("cCT", ch.CNF)
	g.el("CFOP", e.CFOP)
	g.el("natOp", g.txt("natOp", e.NatOp, 1, 60))
	g.el("mod", "57")
	g.el("serie", serie)
	g.el("nCT", nct)
	g.el("dhEmi", dh.Format(layout))
	g.el("tpImp", e.TpImp)
	g.el("tpEmis", e.TpEmis)
	g.el("cDV", ch.CDV)
	g.el("tpAmb", e.TpAmb)
	g.el("tpCTe", e.TpCTe)
	g.el("procEmi", "0")
	g.el("verProc", g.txt("verProc", e.VerProc, 1, 20))
	if e.IndGlobalizado != (e.InfGlobal != "") {
		g.fail("indGlobalizado e infGlobalizado/xObs vêm juntos (ou nenhum)")
	}
	if e.IndGlobalizado {
		g.el("indGlobalizado", "1")
	}
	c, x, u := g.local("município de envio", e.Envio)
	g.el("cMunEnv", c)
	g.el("xMunEnv", x)
	g.el("UFEnv", u)
	g.el("modal", e.Modal)
	g.el("tpServ", e.TpServ)
	c, x, u = g.local("município de início", e.Inicio)
	g.el("cMunIni", c)
	g.el("xMunIni", x)
	g.el("UFIni", u)
	c, x, u = g.local("município de fim", e.Fim)
	g.el("cMunFim", c)
	g.el("xMunFim", x)
	g.el("UFFim", u)
	if e.Retira {
		g.el("retira", "0") // TCTe: 0 = sim, 1 = não
		g.opt("xDetRetira", "xDetRetira", e.XDetRetira, 1, 160)
	} else {
		g.el("retira", "1")
		if e.XDetRetira != "" {
			g.fail("xDetRetira só com Retira=true")
		}
	}
	g.el("indIEToma", e.IndIEToma)
	g.tomador(e)
	if e.TpEmis != "1" && e.DhCont != nil {
		g.el("dhCont", e.DhCont.In(dh.Location()).Format(layout))
		g.el("xJust", g.txt("xJust", e.XJust, 15, 256))
	}
	if cg := e.CompraGov; cg != nil {
		g.open("gCompraGov")
		if !inSet(cg.TpEnteGov, "1", "2", "3", "4", "5", "6") || !inSet(cg.TpOperGov, "1", "2", "3", "4") {
			g.fail("gCompraGov: tpEnteGov 1-6 e tpOperGov 1-4")
		}
		g.el("tpEnteGov", cg.TpEnteGov)
		g.el("pRedutor", g.prtc("pRedutor", cg.PRedutor))
		g.el("tpOperGov", cg.TpOperGov)
		if len(cg.RefDFeAnt) > 99 {
			g.fail("refDFeAnt: no máximo 99")
		}
		for _, k := range cg.RefDFeAnt {
			g.el("refDFeAnt", g.chave("refDFeAnt", k))
		}
		g.end("gCompraGov")
	}
	if e.TpPagAnt != "" {
		if !inSet(e.TpPagAnt, "1", "3") {
			g.fail("tpPagAnt %q inválido (1 ou 3)", e.TpPagAnt)
		}
		g.el("tpPagAnt", e.TpPagAnt)
	}
	if n := len(e.PagAntecipado); n > 0 {
		if n > 99 {
			g.fail("gPagAntecipado: no máximo 99")
		}
		g.open("gPagAntecipado")
		for _, k := range e.PagAntecipado {
			g.el("chDFePagAnt", g.chave("chDFePagAnt", k))
		}
		g.end("gPagAntecipado")
	}
	g.end("ide")
}

// tomador escreve toma3/toma4 e confere indIEToma contra a IE do tomador
// (MOC CT-e 4.00: 1 contribuinte com IE, 2 isento, 9 não contribuinte; a
// rejeição de incoerência, citada como 709 no pedido: não confirmado).
func (g *gen) tomador(e Entrada) {
	if (e.Toma3 == "") == (e.Toma4 == nil) {
		g.fail("tomador: informe Toma3 (0 a 3) ou Toma4, exatamente um")
		return
	}
	var ie string
	if e.Toma4 != nil {
		t := e.Toma4
		if t.Nome == "" || t.End.Logradouro == "" || t.End.Numero == "" || t.End.Bairro == "" ||
			t.End.CodigoMun == "" || t.End.Municipio == "" || t.End.UF == "" {
			g.fail("toma4 exige dados completos: nome e endereço (logradouro, número, bairro, município, UF)")
		}
		ie = t.IE
		g.open("toma4")
		g.el("toma", "4")
		g.parteCorpo("enderToma", "tomador", t.CNPJ, t.CPF, t.IE, t.Nome, t.Fantasia, t.Fone, t.Email, "", t.End, true)
		g.end("toma4")
	} else {
		var p *Parte
		switch e.Toma3 {
		case "0":
			p = e.Rem
		case "1":
			p = e.Exped
		case "2":
			p = e.Receb
		case "3":
			p = e.Dest
		default:
			g.fail("toma3 %q inválido (0 remetente, 1 expedidor, 2 recebedor, 3 destinatário)", e.Toma3)
			return
		}
		if p == nil {
			g.fail("toma3=%s mas o participante correspondente não foi informado", e.Toma3)
			return
		}
		ie = p.IE
		g.open("toma3")
		g.el("toma", e.Toma3)
		g.end("toma3")
	}
	ie = strings.ToUpper(strings.TrimSpace(ie))
	switch e.IndIEToma {
	case "1":
		if ie == "" || ie == "ISENTO" {
			g.fail("indIEToma=1 (contribuinte) exige IE numérica do tomador")
		}
	case "2":
		if ie != "" && ie != "ISENTO" {
			g.fail("indIEToma=2 (isento) não combina com IE numérica no tomador")
		}
	case "9":
		if ie != "" {
			g.fail("indIEToma=9 (não contribuinte) não combina com IE no tomador")
		}
	default:
		g.fail("indIEToma %q inválido (1, 2 ou 9)", e.IndIEToma)
	}
}

// comCNF troca o cNF sorteado pelo informado e recalcula o cDV (mesma regra de
// builder/chave.go: módulo 11, cada caractere vale ASCII-48).
func comCNF(ch builder.ChaveAcesso, cnf string, g *gen) builder.ChaveAcesso {
	if len(cnf) != 8 || digitos(cnf) != cnf || cnf == "00000000" {
		g.fail("%s: precisa de 8 dígitos, diferente de zeros (veio %q)", "CCT", cnf)
		return ch
	}
	ch.CNF = cnf
	base := ch.String()[:43]
	soma := 0
	for i, j := len(base)-1, 0; i >= 0; i, j = i-1, j+1 {
		soma += int(base[i]-'0') * (2 + j%8)
	}
	ch.CDV = "0"
	if r := soma % 11; r >= 2 {
		ch.CDV = string(rune('0' + 11 - r))
	}
	return ch
}

// Chave devolve a chave que Build(e) vai gerar. Exige CCT preenchido (senão o
// cNF é aleatório e a chave não seria a mesma). Não exige QRSign: serve justamente
// para obter a chave antes de assiná-la na contingência.
func Chave(e Entrada) (string, error) {
	if e.CCT == "" {
		return "", fmt.Errorf("cte: Chave exige CCT (sem ele o cNF é aleatório)")
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		e.QRSign = "pendente"
	}
	_, chave, err := Build(e)
	return chave, err
}
