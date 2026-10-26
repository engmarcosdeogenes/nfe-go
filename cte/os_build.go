package cte

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

var (
	rePlaca = regexp.MustCompile(`^([A-Z]{2,3}[0-9]{4}|[A-Z]{3,4}[0-9]{3}|[A-Z0-9]{7})$`)
	reTAF   = regexp.MustCompile(`^[0-9]{12}$`)
	reNRE   = regexp.MustCompile(`^[0-9]{25}$`)
)

// BuildOS monta o CT-e OS (modelo 67; <CTeOS>, sem assinatura e sem <?xml?>)
// e a chave. Regras com a fonte; "não confirmado" = o MOC não pôde ser
// consultado (sem rede), o número da rejeição não é citado:
//   - tpServ 6/7/8 e tpEmis 1/2/7/8: enums do TCTeOS (cteTiposBasico_v4.00.xsd);
//   - toma obrigatório em pessoas e valores, opcional em excesso de bagagem
//     (documentação do XSD); infModal obrigatório em pessoas e bagagem (idem);
//   - rodoOS: TAF (12 dígitos) ou NroRegEstadual (25), exclusivos (XSD);
//   - infFretamento só em pessoas; dhViagem só no eventual (documentação do XSD);
//   - refCTeCanc só em valores (XSD); infGTVe: sem restrição de tpServ no XSD;
//   - ICMS sem CST 60 (TImpOS não tem ICMS60); CRT 3 em homologação exige
//     IBSCBS (NT 2026.002, rej 310);
//   - CFOP de serviço 5/6/7 (tabela completa de CFOP por tpServ: não confirmado).
//
// infTribFed (PIS/COFINS/IR/INSS/CSLL) sai em imp, entre ICMSUFFim e IBSCBS.
// Modais 02 a 04: constam no enum de ide/modal, mas o XSD do OS só define o
// leiaute de infModal para o rodoviário (xs:any; sem XSD aéreo/aquaviário/
// ferroviário de OS) e o MOC não pôde ser consultado: não implementados.
func BuildOS(e EntradaOS) (xmlOut []byte, chave string, err error) {
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
	if !inSet(e.TpCTe, "0", "1", "3") {
		g.fail("tpCTe %q inválido (0 normal, 1 complemento, 3 substituição)", e.TpCTe)
	}
	if !inSet(e.TpServ, "6", "7", "8") {
		g.fail("tpServ %q inválido no CT-e OS (6 pessoas, 7 valores, 8 excesso de bagagem)", e.TpServ)
	}
	if !inSet(e.TpImp, "1", "2") {
		g.fail("tpImp %q inválido (1 ou 2)", e.TpImp)
	}
	if !inSet(e.TpEmis, "1", "2", "7", "8") {
		g.fail("tpEmis %q inválido no CT-e OS (1, 2, 7 ou 8)", e.TpEmis)
	}
	switch e.Modal {
	case "01":
	case "02", "03", "04":
		g.fail("modal %s do CT-e OS não implementado: o XSD do OS só define infModal rodoviário (01); leiaute dos demais não confirmado", e.Modal)
	default:
		g.fail("modal %q inválido no CT-e OS (01 a 04)", e.Modal)
	}
	if e.TpEmis != "1" {
		if e.DhCont == nil || len(e.XJust) < 15 {
			g.fail("tpEmis %s exige dhCont e xJust (15 a 256 caracteres)", e.TpEmis)
		}
	} else if e.DhCont != nil || e.XJust != "" {
		g.fail("dhCont/xJust só existem com tpEmis diferente de 1")
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		g.fail("contingência offline (tpEmis 2) exige QRSign")
	}
	if !reCFOP.MatchString(e.CFOP) || e.CFOP[0] < '5' {
		g.fail("CFOP %q inválido para prestação de serviço (5, 6 ou 7)", e.CFOP)
	}

	em := e.Emitente
	g.uf("UF do emitente", em.End.UF)
	serie := g.inteiro("serie", e.Serie, 0, 999)
	nct := g.inteiro("nCT", e.NCT, 1, 999999999)
	if em.CPF != "" || em.CNPJ == "" {
		g.fail("emitente do CT-e OS: só CNPJ (TCnpj); CPF não existe no grupo emit")
	}
	idTag, idEmit := g.docID("emitente", em.CNPJ)
	if idTag != "CNPJ" {
		g.fail("emitente do CT-e OS: CNPJ de 14 posições")
	}
	if em.IE == "" || strings.EqualFold(strings.TrimSpace(em.IE), "ISENTO") {
		g.fail("emitente: IE numérica obrigatória (TIe)")
	}
	if !inSet(em.CRT, "1", "2", "3", "4") {
		g.fail("CRT %q inválido (1, 2, 3 ou 4)", em.CRT)
	}
	if g.err != nil {
		return nil, "", g.err
	}
	dh := e.DhEmi.In(builder.FusoUF(em.End.UF))
	ch := builder.NovaChave(em.End.UF, idEmit, serie, nct, e.TpEmis, "67", dh)
	if e.CCT != "" {
		ch = comCNF(ch, e.CCT, g)
	}
	chave = ch.String()

	g.raw(`<CTeOS xmlns="` + nsCTe + `" versao="4.00"><infCte versao="4.00" Id="CTe` + chave + `">`)
	g.ideOS(e, ch, serie, nct, dh)
	if c := e.Compl; c != nil {
		if c.XOrig != "" || c.XDest != "" || c.XRota != "" || len(c.Pass) > 0 || c.Entrega != nil || c.OrigCalc != "" || c.DestCalc != "" {
			g.fail("compl do CT-e OS só tem xCaracAd, xCaracSer, xEmi, xObs, ObsCont e ObsFisco")
		}
		g.compl(c)
	}
	g.emit(em, idTag, idEmit)
	g.tomaOS(e)
	g.vPrest(Entrada{VTPrest: e.VTPrest, VTPrestLiq: e.VTPrestLiq, VRec: e.VRec, Comp: e.Comp})
	if e.ICMS.CST == "60" && !e.ICMS.SN && !e.ICMS.OutraUF {
		g.fail("ICMS CST 60 não existe no CT-e OS (TImpOS: 00, 20, 40/41/51, 90, OutraUF, SN)")
	}
	g.tribFed = e.TribFed
	g.imp(Entrada{TpAmb: e.TpAmb, Emitente: em, ICMS: e.ICMS, VTotTrib: e.VTotTrib, InfAdFisco: e.InfAdFisco,
		ICMSUFFim: e.ICMSUFFim, IBSCBS: e.IBSCBS, VTotDFe: e.VTotDFe})
	g.pgtoVinc(e.PgtoVinc)
	if e.TpCTe == "1" {
		g.complementoOS(e)
	} else {
		g.normOS(e)
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
		if t, v := g.docID("respTec", rt.CNPJ); t != "CNPJ" {
			g.fail("infRespTec exige CNPJ")
		} else {
			g.el("CNPJ", v)
		}
		g.el("xContato", g.txt("xContato", rt.Contato, 2, 60))
		g.el("email", g.email("email do responsável técnico", rt.Email))
		f := digitos(rt.Fone)
		if !reFoneRT.MatchString(f) {
			g.fail("fone do responsável técnico: 7 a 12 dígitos")
		}
		g.el("fone", f)
		g.end("infRespTec")
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
	g.raw("</CTeOS>")

	if g.err != nil {
		return nil, "", g.err
	}
	return []byte(g.b.String()), chave, nil
}

// ChaveOS devolve a chave que BuildOS(e) vai gerar (exige CCT, como Chave).
func ChaveOS(e EntradaOS) (string, error) {
	if e.CCT == "" {
		return "", fmt.Errorf("cte: ChaveOS exige CCT (sem ele o cNF é aleatório)")
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		e.QRSign = "pendente"
	}
	_, chave, err := BuildOS(e)
	return chave, err
}

func (g *gen) ideOS(e EntradaOS, ch builder.ChaveAcesso, serie, nct string, dh time.Time) {
	g.open("ide")
	g.el("cUF", ch.CUF)
	g.el("cCT", ch.CNF)
	g.el("CFOP", e.CFOP)
	g.el("natOp", g.txt("natOp", e.NatOp, 1, 60))
	g.el("mod", "67")
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
	c, x, u := g.local("município de envio", e.Envio)
	g.el("cMunEnv", c)
	g.el("xMunEnv", x)
	g.el("UFEnv", u)
	g.el("modal", e.Modal)
	g.el("tpServ", e.TpServ)
	if !inSet(e.IndIEToma, "1", "2", "9") {
		g.fail("indIEToma %q inválido (1, 2 ou 9)", e.IndIEToma)
	}
	g.el("indIEToma", e.IndIEToma)
	if (e.Inicio == nil) != (e.Fim == nil) {
		g.fail("início e fim da prestação vêm juntos (ou nenhum)")
	}
	if e.Inicio != nil && e.Fim != nil {
		c, x, u = g.local("município de início", *e.Inicio)
		g.el("cMunIni", c)
		g.el("xMunIni", x)
		g.el("UFIni", u)
		c, x, u = g.local("município de fim", *e.Fim)
		g.el("cMunFim", c)
		g.el("xMunFim", x)
		g.el("UFFim", u)
	}
	if len(e.Percurso) > 25 {
		g.fail("infPercurso: no máximo 25 UF")
	}
	for _, uf := range e.Percurso {
		g.uf("UFPer", uf)
		g.open("infPercurso")
		g.el("UFPer", uf)
		g.end("infPercurso")
	}
	if e.TpEmis != "1" && e.DhCont != nil {
		g.el("dhCont", e.DhCont.In(dh.Location()).Format(layout))
		g.el("xJust", g.txt("xJust", e.XJust, 15, 256))
	}
	if cg := e.CompraGov; cg != nil {
		if !inSet(cg.TpEnteGov, "1", "2", "3", "4", "5", "6") || !inSet(cg.TpOperGov, "1", "2", "3", "4") {
			g.fail("gCompraGov: tpEnteGov 1-6 e tpOperGov 1-4")
		}
		if len(cg.RefDFeAnt) > 99 {
			g.fail("refDFeAnt: no máximo 99")
		}
		g.open("gCompraGov")
		g.el("tpEnteGov", cg.TpEnteGov)
		g.el("pRedutor", g.prtc("pRedutor", cg.PRedutor))
		g.el("tpOperGov", cg.TpOperGov)
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

// tomaOS escreve o grupo toma e confere indIEToma contra a IE do tomador.
func (g *gen) tomaOS(e EntradaOS) {
	t := e.Toma
	if t == nil {
		if e.TpServ != "8" {
			g.fail("toma (tomador do serviço) obrigatório em pessoas e valores; só o excesso de bagagem o dispensa")
		}
		if e.IndIEToma != "9" {
			g.fail("sem toma, indIEToma deve ser 9")
		}
		return
	}
	if t.Nome == "" || t.End.Logradouro == "" || t.End.Numero == "" || t.End.Bairro == "" ||
		t.End.CodigoMun == "" || t.End.Municipio == "" || t.End.UF == "" {
		g.fail("toma exige nome e endereço completos (logradouro, número, bairro, município, UF)")
	}
	g.open("toma")
	g.parteCorpo("enderToma", "tomador", t.CNPJ, t.CPF, t.IE, t.Nome, t.Fantasia, t.Fone, t.Email, "", t.End, true)
	g.end("toma")
	ie := strings.ToUpper(strings.TrimSpace(t.IE))
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
	}
}

func (g *gen) complementoOS(e EntradaOS) {
	if n := len(e.InfCteComp); n < 1 || n > 10 {
		g.fail("complemento (tpCTe 1) exige 1 a 10 chaves em InfCteComp, veio %d", n)
	}
	if e.Servico != nil || len(e.DocRef) > 0 || len(e.Seg) > 0 || e.RodoOS != nil || e.InfCteSub != "" ||
		e.RefCTeCanc != "" || e.Cobr != nil || len(e.GTVe) > 0 {
		g.fail("complemento (tpCTe 1) não leva infCTeNorm (serviço, documentos, modal, cobrança, GTV-e)")
	}
	vistas := map[string]bool{}
	for _, k := range e.InfCteComp {
		if vistas[k] {
			g.fail("chave %s repetida em InfCteComp", k)
		}
		vistas[k] = true
		g.open("infCteComp")
		g.el("chCTe", g.chave("infCteComp", k))
		g.end("infCteComp")
	}
}

func (g *gen) normOS(e EntradaOS) {
	if len(e.InfCteComp) > 0 {
		g.fail("InfCteComp só no complemento (tpCTe 1)")
	}
	switch {
	case e.TpCTe == "3" && e.InfCteSub == "":
		g.fail("substituição (tpCTe 3) exige InfCteSub (chave do CT-e OS substituído)")
	case e.TpCTe != "3" && e.InfCteSub != "":
		g.fail("InfCteSub só na substituição (tpCTe 3)")
	}
	if e.RefCTeCanc != "" && e.TpServ != "7" {
		g.fail("refCTeCanc só no transporte de valores (tpServ 7)")
	}
	g.open("infCTeNorm")
	s := e.Servico
	if s == nil {
		g.fail("infServico obrigatório no CT-e OS normal/substituição")
		s = &InfServicoOS{}
	}
	g.open("infServico")
	g.el("xDescServ", g.txt("xDescServ", s.XDescServ, 1, 30))
	if s.QCarga != nil {
		g.open("infQ")
		g.el("qCarga", g.qtd4("qCarga", *s.QCarga))
		g.end("infQ")
	}
	g.end("infServico")
	for _, d := range e.DocRef {
		g.open("infDocRef")
		switch {
		case d.ChBPe != "" && d.NDoc != "":
			g.fail("infDocRef: chBPe ou nDoc, não os dois")
		case d.ChBPe != "":
			g.el("chBPe", g.chave("chBPe", d.ChBPe))
		default:
			g.el("nDoc", g.txt("infDocRef nDoc", d.NDoc, 1, 20))
			g.opt("serie", "infDocRef serie", d.Serie, 1, 3)
			g.opt("subserie", "infDocRef subserie", d.Subserie, 1, 3)
			g.el("dEmi", g.data("infDocRef dEmi", d.DEmi))
			if d.VDoc != nil {
				g.el("vDoc", g.m("vDoc", *d.VDoc))
			}
		}
		g.end("infDocRef")
	}
	for _, sg := range e.Seg {
		if !inSet(sg.RespSeg, "4", "5") {
			g.fail("seg respSeg %q inválido (4 emitente, 5 tomador)", sg.RespSeg)
		}
		g.open("seg")
		g.el("respSeg", sg.RespSeg)
		g.opt("xSeg", "xSeg", sg.XSeg, 1, 30)
		g.opt("nApol", "nApol", sg.NApol, 1, 20)
		g.end("seg")
	}
	g.rodoOS(e)
	if e.InfCteSub != "" {
		g.open("infCteSub")
		g.el("chCte", g.chave("infCteSub", e.InfCteSub))
		g.end("infCteSub")
	}
	if e.RefCTeCanc != "" {
		g.el("refCTeCanc", g.chave("refCTeCanc", e.RefCTeCanc))
	}
	if cb := e.Cobr; cb != nil {
		g.open("cobr")
		if cb.NFat != "" || cb.VOrig != nil || cb.VDesc != nil || cb.VLiq != nil {
			g.open("fat")
			g.opt("nFat", "nFat", cb.NFat, 1, 60)
			for _, c := range []struct {
				t string
				v *float64
			}{{"vOrig", cb.VOrig}, {"vDesc", cb.VDesc}, {"vLiq", cb.VLiq}} {
				if c.v != nil {
					g.el(c.t, g.mpos(c.t, *c.v))
				}
			}
			g.end("fat")
		}
		for _, d := range cb.Dup {
			g.open("dup")
			g.opt("nDup", "nDup", d.NDup, 1, 60)
			if d.DVenc != "" {
				g.el("dVenc", g.data("dVenc", d.DVenc))
			}
			if d.VDup != nil {
				g.el("vDup", g.mpos("vDup", *d.VDup))
			}
			g.end("dup")
		}
		g.end("cobr")
	}
	for _, v := range e.GTVe {
		if len(v.Comp) == 0 {
			g.fail("infGTVe exige ao menos um Comp")
		}
		g.open("infGTVe")
		g.el("chCTe", g.chave("infGTVe", v.Chave))
		for _, c := range v.Comp {
			if !inSet(c.TpComp, "1", "2", "3", "4", "5", "6") {
				g.fail("infGTVe tpComp %q inválido (1 a 6)", c.TpComp)
			}
			g.open("Comp")
			g.el("tpComp", c.TpComp)
			g.el("vComp", g.m("vComp GTV-e", c.Valor))
			g.opt("xComp", "xComp", c.XComp, 1, 15)
			g.end("Comp")
		}
		g.end("infGTVe")
	}
	g.end("infCTeNorm")
}

// rodoOS escreve infModal/rodoOS (cteModalRodoviarioOS_v4.00.xsd).
func (g *gen) rodoOS(e EntradaOS) {
	r := e.RodoOS
	if r == nil {
		if e.TpServ != "7" {
			g.fail("infModal (rodoOS) obrigatório em pessoas e excesso de bagagem")
		}
		return
	}
	reg := func(nome, taf, nre string) (string, string) {
		switch {
		case (taf == "") == (nre == ""):
			g.fail("%s: informe TAF ou NroRegEstadual, exatamente um", nome)
		case taf != "" && !reTAF.MatchString(taf):
			g.fail("%s: TAF precisa de 12 dígitos", nome)
		case nre != "" && !reNRE.MatchString(nre):
			g.fail("%s: NroRegEstadual precisa de 25 dígitos", nome)
		}
		if taf != "" {
			return "TAF", taf
		}
		return "NroRegEstadual", nre
	}
	g.raw(`<infModal versaoModal="4.00"><rodoOS>`)
	t, v := reg("rodoOS", r.TAF, r.NroRegEstadual)
	g.el(t, v)
	if vc := r.Veic; vc != nil {
		g.open("veic")
		pl := strings.ToUpper(vc.Placa)
		if !rePlaca.MatchString(pl) {
			g.fail("veic placa %q inválida", vc.Placa)
		}
		g.el("placa", pl)
		if vc.RENAVAM != "" {
			g.el("RENAVAM", g.txt("RENAVAM", vc.RENAVAM, 9, 11))
		}
		if p := vc.Prop; p != nil {
			g.open("prop")
			switch {
			case (p.CNPJ == "") == (p.CPF == ""):
				g.fail("prop: informe CNPJ ou CPF, exatamente um")
			case p.CPF != "":
				_, c := g.docID("prop", p.CPF)
				g.el("CPF", c)
			default:
				_, c := g.docID("prop", p.CNPJ)
				g.el("CNPJ", c)
			}
			t, v := reg("prop", p.TAF, p.NroRegEstadual)
			g.el(t, v)
			g.el("xNome", g.txt("prop xNome", p.Nome, 2, 60))
			if (p.IE == "") != (p.UF == "") {
				g.fail("prop: IE e UF vêm juntos")
			}
			if p.IE != "" {
				g.el("IE", g.ie("prop IE", p.IE, true))
				g.uf("prop UF", p.UF)
				g.el("UF", p.UF)
			}
			if !inSet(p.TpProp, "0", "1", "2") {
				g.fail("prop tpProp %q inválido (0, 1 ou 2)", p.TpProp)
			}
			g.el("tpProp", p.TpProp)
			g.end("prop")
		}
		if vc.UF != "" {
			g.uf("veic UF", vc.UF)
			g.el("UF", vc.UF)
		}
		g.end("veic")
	}
	if f := r.Fretamento; f != nil {
		if e.TpServ != "6" {
			g.fail("infFretamento só no transporte de pessoas (tpServ 6)")
		}
		if !inSet(f.TpFretamento, "1", "2") {
			g.fail("tpFretamento %q inválido (1 eventual, 2 contínuo)", f.TpFretamento)
		}
		if (f.TpFretamento == "1") != (f.DhViagem != nil) {
			g.fail("dhViagem é obrigatório no fretamento eventual (1) e proibido no contínuo (2)")
		}
		g.open("infFretamento")
		g.el("tpFretamento", f.TpFretamento)
		if f.DhViagem != nil {
			g.el("dhViagem", f.DhViagem.In(builder.FusoUF(e.Emitente.End.UF)).Format(layout))
		}
		g.end("infFretamento")
	}
	g.raw(`</rodoOS></infModal>`)
}
