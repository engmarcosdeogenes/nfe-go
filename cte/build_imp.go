package cte

import "strings"

func (g *gen) compl(c *Compl) {
	if c == nil {
		return
	}
	g.open("compl")
	g.opt("xCaracAd", "xCaracAd", c.XCaracAd, 1, 15)
	g.opt("xCaracSer", "xCaracSer", c.XCaracSer, 1, 30)
	g.opt("xEmi", "xEmi", c.XEmi, 1, 20)
	if c.XOrig != "" || c.XDest != "" || c.XRota != "" || len(c.Pass) > 0 {
		g.open("fluxo")
		g.opt("xOrig", "xOrig", c.XOrig, 1, 60)
		for _, p := range c.Pass {
			g.open("pass")
			g.el("xPass", g.txt("xPass", p, 1, 15))
			g.end("pass")
		}
		g.opt("xDest", "xDest", c.XDest, 1, 60)
		g.opt("xRota", "xRota", c.XRota, 1, 10)
		g.end("fluxo")
	}
	if en := c.Entrega; en != nil {
		g.open("Entrega")
		switch en.TpPer {
		case "0":
			g.raw("<semData><tpPer>0</tpPer></semData>")
		case "1", "2", "3":
			g.raw("<comData><tpPer>" + en.TpPer + "</tpPer><dProg>" + g.data("dProg", en.DProg) + "</dProg></comData>")
		case "4":
			g.raw("<noPeriodo><tpPer>4</tpPer><dIni>" + g.data("dIni", en.DIni) + "</dIni><dFim>" + g.data("dFim", en.DFim) + "</dFim></noPeriodo>")
		default:
			g.fail("Entrega tpPer %q inválido (0..4)", en.TpPer)
		}
		hora := func(nome, v string) string {
			if !reHora.MatchString(v) {
				g.fail("Entrega %s %q inválida (HH:MM:SS)", nome, v)
			}
			return v
		}
		switch en.TpHor {
		case "0":
			g.raw("<semHora><tpHor>0</tpHor></semHora>")
		case "1", "2", "3":
			g.raw("<comHora><tpHor>" + en.TpHor + "</tpHor><hProg>" + hora("hProg", en.HProg) + "</hProg></comHora>")
		case "4":
			g.raw("<noInter><tpHor>4</tpHor><hIni>" + hora("hIni", en.HIni) + "</hIni><hFim>" + hora("hFim", en.HFim) + "</hFim></noInter>")
		default:
			g.fail("Entrega tpHor %q inválido (0..4)", en.TpHor)
		}
		g.end("Entrega")
	}
	g.opt("origCalc", "origCalc", c.OrigCalc, 2, 40)
	g.opt("destCalc", "destCalc", c.DestCalc, 2, 40)
	g.opt("xObs", "xObs", c.XObs, 1, 2000)
	if len(c.ObsCont) > 10 || len(c.ObsFisco) > 10 {
		g.fail("ObsCont e ObsFisco: no máximo 10 cada")
	}
	for _, o := range c.ObsCont {
		g.raw(`<ObsCont xCampo="` + escAttr.Replace(g.txt("ObsCont xCampo", o.Campo, 1, 20)) + `">`)
		g.el("xTexto", g.txt("ObsCont xTexto", o.Texto, 1, 160))
		g.end("ObsCont")
	}
	for _, o := range c.ObsFisco {
		g.raw(`<ObsFisco xCampo="` + escAttr.Replace(g.txt("ObsFisco xCampo", o.Campo, 1, 20)) + `">`)
		g.el("xTexto", g.txt("ObsFisco xTexto", o.Texto, 1, 60))
		g.end("ObsFisco")
	}
	g.end("compl")
}

func (g *gen) emit(em Emitente, idTag, idEmit string) {
	g.open("emit")
	g.el(idTag, idEmit)
	for _, c := range []struct{ t, v string }{{"IE", em.IE}, {"IEST", em.IEST}} {
		if v := g.ie("emitente "+c.t, c.v, false); v != "" {
			g.el(c.t, v)
		}
	}
	g.el("xNome", g.txt("emitente xNome", em.Nome, 2, 60))
	g.opt("xFant", "emitente xFant", em.Fantasia, 2, 60)
	g.endereco("enderEmit", "emitente", em.End, true)
	g.el("CRT", em.CRT)
	if em.ISUF != "" {
		if d := digitos(em.ISUF); len(d) < 8 || len(d) > 9 {
			g.fail("ISUF do emitente: 8 a 9 dígitos")
		} else {
			g.el("ISUFEmit", d)
		}
	}
	g.end("emit")
}

// vPrest: vRec <= vTPrest e soma dos Comp == vTPrest. Rejeições do MOC
// (citadas como 531 etc. no pedido): números não confirmados.
func (g *gen) vPrest(e Entrada) {
	if cents(e.VRec) > cents(e.VTPrest) {
		g.fail("vRec (%.2f) maior que vTPrest (%.2f)", e.VRec, e.VTPrest)
	}
	if len(e.Comp) > 0 {
		var soma int64
		for _, c := range e.Comp {
			soma += cents(c.Valor)
		}
		if soma != cents(e.VTPrest) {
			g.fail("soma dos Comp (%.2f) difere de vTPrest (%.2f)", float64(soma)/100, e.VTPrest)
		}
	}
	if e.VTPrestLiq != nil && cents(*e.VTPrestLiq) > cents(e.VTPrest) {
		g.fail("vTPrestLiq maior que vTPrest")
	}
	g.open("vPrest")
	g.el("vTPrest", g.m("vTPrest", e.VTPrest))
	if e.VTPrestLiq != nil {
		g.el("vTPrestLiq", g.m("vTPrestLiq", *e.VTPrestLiq))
	}
	g.el("vRec", g.m("vRec", e.VRec))
	for _, c := range e.Comp {
		g.open("Comp")
		g.el("xNome", g.txt("Comp xNome", c.Nome, 1, 15))
		g.el("vComp", g.m("vComp", c.Valor))
		g.end("Comp")
	}
	g.end("vPrest")
}

func (g *gen) pgtoVinc(ps []PagVinc) {
	if len(ps) == 0 {
		return
	}
	if len(ps) > 99 {
		g.fail("pgtoVinc: no máximo 99")
	}
	g.open("pgtoVinc")
	for _, p := range ps {
		if !reNPag(p.NPag) || len(p.IDTransacao) < 2 || len(p.IDTransacao) > 35 || !soDigitos(p.TpMeioPgto, 2) {
			g.fail("pgto: nPag 1-990, idTransacao 2-35 e tpMeioPgto 2 dígitos")
		}
		if !reCNPJ.MatchString(limpa(p.CNPJReceb)) || len(limpa(p.CNPJBasePSP)) != 8 {
			g.fail("pgto: CNPJReceb (14) e CNPJBasePSP (8) inválidos")
		}
		g.raw(`<pgto nPag="` + p.NPag + `" idTransacao="` + escAttr.Replace(g.txt("idTransacao", p.IDTransacao, 2, 35)) + `">`)
		g.el("tpMeioPgto", p.TpMeioPgto)
		g.el("CNPJReceb", limpa(p.CNPJReceb))
		g.el("CNPJBasePSP", limpa(p.CNPJBasePSP))
		g.end("pgto")
	}
	g.end("pgtoVinc")
}

func reNPag(s string) bool {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return s != "" && s[0] != '0' && n >= 1 && n <= 990
}

// imp escreve o grupo de impostos. Regras (MOC CT-e 4.00 / NT 2025.001 RTC):
//   - CRT 1 e 4 usam ICMSSN; regime normal não pode (rejeição por grupo
//     incompatível com o CRT: número não confirmado);
//   - vICMS = vBC*pICMS/100, tolerância de 1 centavo (tolerância: não confirmada);
//   - CRT 3 em homologação exige IBSCBS (NT 2026.002, rej 310).
func (g *gen) imp(e Entrada) {
	ic := e.ICMS
	crt := e.Emitente.CRT
	simples := crt == "1" || crt == "4"
	switch {
	case ic.Previsto != nil && ic.CST == "" && !ic.SN:
	case ic.SN && !simples:
		g.fail("ICMSSN só para CRT 1 ou 4; CRT %s usa os grupos ICMS00..ICMS90", crt)
	case !ic.SN && simples:
		g.fail("CRT %s (Simples Nacional/MEI) deve usar ICMSSN (SN=true, CST 90, indSN=1)", crt)
	}
	if crt == "3" && e.TpAmb == "2" && e.IBSCBS == nil {
		g.fail("IBSCBS obrigatório para CRT 3 em homologação (NT 2026.002, rej 310)")
	}
	g.open("imp")
	g.open("ICMS")
	conf := func(nome string, base, aliq, valor float64) {
		if !perto(base*aliq/100, valor, 1) {
			g.fail("%s: %.2f difere de base %.2f x alíquota %.2f%% (tolerância 0,01)", nome, valor, base, aliq)
		}
	}
	deson := func() {
		if ic.CBenef != "" {
			if n := len(ic.CBenef); (n != 8 && n != 10) || strings.ContainsAny(ic.CBenef, " 	") {
				g.fail("cBenef: 8 ou 10 caracteres, sem espaço")
			}
			g.el("vICMSDeson", g.m("vICMSDeson", ic.VICMSDeson))
			g.el("cBenef", g.txt("cBenef", ic.CBenef, 8, 10))
		} else if ic.VICMSDeson != 0 {
			g.fail("vICMSDeson exige cBenef")
		}
	}
	cst := func(set ...string) {
		if !inSet(ic.CST, set...) {
			g.fail("CST %q incompatível com este grupo de ICMS (%s)", ic.CST, strings.Join(set, "/"))
		}
		g.el("CST", ic.CST)
	}
	switch {
	case ic.SN:
		if ic.CST != "" && ic.CST != "90" {
			g.fail("ICMSSN usa CST 90")
		}
		g.raw("<ICMSSN><CST>90</CST><indSN>1</indSN></ICMSSN>")
	case ic.Previsto != nil && ic.CST == "":
		g.open("gICMSPrevistoPagtoAntecip")
		g.el("vICMSPrevisto", g.m("vICMSPrevisto", *ic.Previsto))
		g.end("gICMSPrevistoPagtoAntecip")
	case ic.OutraUF:
		g.open("ICMSOutraUF")
		cst("90")
		if ic.PRedBC != 0 {
			g.el("pRedBCOutraUF", g.ppos("pRedBC", ic.PRedBC))
		}
		g.el("vBCOutraUF", g.m("vBC", ic.VBC))
		g.el("pICMSOutraUF", g.p("pICMS", ic.PICMS))
		g.el("vICMSOutraUF", g.m("vICMS", ic.VICMS))
		conf("vICMSOutraUF", ic.VBC, ic.PICMS, ic.VICMS)
		deson()
		g.end("ICMSOutraUF")
	case ic.CST == "00":
		g.open("ICMS00")
		cst("00")
		g.el("vBC", g.m("vBC", ic.VBC))
		g.el("pICMS", g.p("pICMS", ic.PICMS))
		g.el("vICMS", g.m("vICMS", ic.VICMS))
		conf("vICMS", ic.VBC, ic.PICMS, ic.VICMS)
		g.end("ICMS00")
	case ic.CST == "20":
		g.open("ICMS20")
		cst("20")
		g.el("pRedBC", g.ppos("pRedBC", ic.PRedBC))
		g.el("vBC", g.m("vBC", ic.VBC))
		g.el("pICMS", g.p("pICMS", ic.PICMS))
		g.el("vICMS", g.m("vICMS", ic.VICMS))
		conf("vICMS", ic.VBC, ic.PICMS, ic.VICMS)
		deson()
		g.end("ICMS20")
	case inSet(ic.CST, "40", "41", "51"):
		g.open("ICMS45")
		cst("40", "41", "51")
		deson()
		g.end("ICMS45")
	case ic.CST == "60":
		g.open("ICMS60")
		cst("60")
		g.el("vBCSTRet", g.m("vBCSTRet", ic.VBCSTRet))
		g.el("vICMSSTRet", g.m("vICMSSTRet", ic.VICMSSTRet))
		g.el("pICMSSTRet", g.p("pICMSSTRet", ic.PICMSSTRet))
		conf("vICMSSTRet", ic.VBCSTRet, ic.PICMSSTRet, ic.VICMSSTRet)
		if ic.VCred != nil {
			g.el("vCred", g.m("vCred", *ic.VCred))
		}
		deson()
		g.end("ICMS60")
	case ic.CST == "90":
		g.open("ICMS90")
		cst("90")
		if ic.PRedBC != 0 {
			g.el("pRedBC", g.ppos("pRedBC", ic.PRedBC))
		}
		g.el("vBC", g.m("vBC", ic.VBC))
		g.el("pICMS", g.p("pICMS", ic.PICMS))
		g.el("vICMS", g.m("vICMS", ic.VICMS))
		conf("vICMS", ic.VBC, ic.PICMS, ic.VICMS)
		if ic.VCred != nil {
			g.el("vCred", g.m("vCred", *ic.VCred))
		}
		deson()
		g.end("ICMS90")
	default:
		g.fail("ICMS: CST %q sem grupo (00, 20, 40, 41, 51, 60, 90, OutraUF, SN ou Previsto)", ic.CST)
	}
	g.end("ICMS")
	if e.VTotTrib != nil {
		g.el("vTotTrib", g.m("vTotTrib", *e.VTotTrib))
	}
	g.opt("infAdFisco", "infAdFisco", e.InfAdFisco, 1, 2000)
	if u := e.ICMSUFFim; u != nil {
		g.open("ICMSUFFim")
		g.el("vBCUFFim", g.m("vBCUFFim", u.VBCUFFim))
		g.el("pFCPUFFim", g.p("pFCPUFFim", u.PFCPUFFim))
		g.el("pICMSUFFim", g.p("pICMSUFFim", u.PICMSUFFim))
		g.el("pICMSInter", g.p("pICMSInter", u.PICMSInter))
		g.el("vFCPUFFim", g.m("vFCPUFFim", u.VFCPUFFim))
		g.el("vICMSUFFim", g.m("vICMSUFFim", u.VICMSUFFim))
		g.el("vICMSUFIni", g.m("vICMSUFIni", u.VICMSUFIni))
		// FCP = base do destino x alíquota do FCP (não confirmado no MOC).
		conf("vFCPUFFim", u.VBCUFFim, u.PFCPUFFim, u.VFCPUFFim)
		g.end("ICMSUFFim")
	}
	if t := g.tribFed; t != nil {
		g.open("infTribFed")
		for _, c := range []struct {
			n string
			v *float64
		}{{"vPIS", t.VPIS}, {"vCOFINS", t.VCOFINS}, {"vIR", t.VIR}, {"vINSS", t.VINSS}, {"vCSLL", t.VCSLL}} {
			if c.v != nil {
				g.el(c.n, g.m(c.n, *c.v))
			}
		}
		g.end("infTribFed")
	}
	if e.IBSCBS != nil {
		g.ibscbs(e.IBSCBS)
	}
	if e.VTotDFe != nil {
		g.el("vTotDFe", g.m("vTotDFe", *e.VTotDFe))
	}
	g.end("imp")
}

// ibscbs escreve TTribCTe. Conferência de valores (NT 2025.001 RTC; fórmulas
// não confirmadas no MOC, tolerância de 2 centavos): v = vBC x alíquota
// efetiva (pAliqEfet se há redução, senão a nominal) - vDif - vDevTrib;
// vIBS = vIBSUF + vIBSMun.
func (g *gen) ibscbs(t *IBSCBS) {
	if !soDigitos(t.CST, 3) || !soDigitos(t.CClassTrib, 6) {
		g.fail("IBSCBS: CST (3 dígitos) e cClassTrib (6 dígitos)")
	}
	g.open("IBSCBS")
	g.el("CST", t.CST)
	g.el("cClassTrib", t.CClassTrib)
	if t.IndDoacao {
		g.el("indDoacao", "1")
	}
	if x := t.G; x != nil {
		g.open("gIBSCBS")
		g.el("vBC", g.m("IBSCBS vBC", x.VBC))
		grupo := func(tag, pTag, vTag string, a AliqRTC, extra func()) {
			g.open(tag)
			g.el(pTag, g.prtc(pTag, a.P))
			if a.PDif != nil {
				g.open("gDif")
				g.el("pDif", g.prtc("pDif", *a.PDif))
				g.el("vDif", g.m("vDif", a.VDif))
				g.end("gDif")
			}
			if a.PDev != nil {
				g.open("gDevTrib")
				g.el("pDevTrib", g.prtc("pDevTrib", *a.PDev))
				g.el("vDevTrib", g.m("vDevTrib", a.VDev))
				g.end("gDevTrib")
			}
			rate := a.P
			if (a.PRedAliq == nil) != (a.PAliqEfet == nil) {
				g.fail("%s: gRed exige pRedAliq e pAliqEfet juntos", tag)
			}
			if a.PRedAliq != nil && a.PAliqEfet != nil {
				g.open("gRed")
				g.el("pRedAliq", g.prtc("pRedAliq", *a.PRedAliq))
				g.el("pAliqEfet", g.prtc("pAliqEfet", *a.PAliqEfet))
				g.end("gRed")
				rate = *a.PAliqEfet
			}
			if extra != nil {
				extra()
			}
			g.el(vTag, g.m(vTag, a.V))
			exp := x.VBC * rate / 100
			if a.PDif != nil {
				exp -= a.VDif
			}
			if a.PDev != nil {
				exp -= a.VDev
			}
			if !perto(exp, a.V, 2) {
				g.fail("%s: %.2f difere do esperado %.2f (vBC %.2f, alíquota %.4f%%)", vTag, a.V, exp, x.VBC, rate)
			}
			g.end(tag)
		}
		grupo("gIBSUF", "pIBSUF", "vIBSUF", x.IBSUF, nil)
		grupo("gIBSMun", "pIBSMun", "vIBSMun", x.IBSMun, nil)
		g.el("vIBS", g.m("vIBS", x.VIBS))
		if !perto(x.IBSUF.V+x.IBSMun.V, x.VIBS, 2) {
			g.fail("vIBS (%.2f) difere de vIBSUF + vIBSMun (%.2f)", x.VIBS, x.IBSUF.V+x.IBSMun.V)
		}
		grupo("gCBS", "pCBS", "vCBS", x.CBS, func() {
			if z := x.ALCZFM; z != nil {
				g.open("gALCZFMCBS")
				g.el("pAliqEfetRegCBS", g.prtc("pAliqEfetRegCBS", z.PAliqEfetRegCBS))
				g.el("vTribRegCBS", g.m("vTribRegCBS", z.VTribRegCBS))
				g.end("gALCZFMCBS")
			}
		})
		if r := x.Regular; r != nil {
			if !soDigitos(r.CST, 3) || !soDigitos(r.CClassTrib, 6) {
				g.fail("gTribRegular: CSTReg (3) e cClassTribReg (6 dígitos)")
			}
			g.open("gTribRegular")
			g.el("CSTReg", r.CST)
			g.el("cClassTribReg", r.CClassTrib)
			g.el("pAliqEfetRegIBSUF", g.prtc("pAliqEfetRegIBSUF", r.PIBSUF))
			g.el("vTribRegIBSUF", g.m("vTribRegIBSUF", r.VIBSUF))
			g.el("pAliqEfetRegIBSMun", g.prtc("pAliqEfetRegIBSMun", r.PIBSMun))
			g.el("vTribRegIBSMun", g.m("vTribRegIBSMun", r.VIBSMun))
			g.el("pAliqEfetRegCBS", g.prtc("pAliqEfetRegCBS", r.PCBS))
			g.el("vTribRegCBS", g.m("vTribRegCBS", r.VCBS))
			g.end("gTribRegular")
		}
		if r := x.CompraG; r != nil {
			g.open("gTribCompraGov")
			g.el("pAliqIBSUF", g.prtc("pAliqIBSUF", r.PIBSUF))
			g.el("vTribIBSUF", g.m("vTribIBSUF", r.VIBSUF))
			g.el("pAliqIBSMun", g.prtc("pAliqIBSMun", r.PIBSMun))
			g.el("vTribIBSMun", g.m("vTribIBSMun", r.VIBSMun))
			g.el("pAliqCBS", g.prtc("pAliqCBS", r.PCBS))
			g.el("vTribCBS", g.m("vTribCBS", r.VCBS))
			g.end("gTribCompraGov")
		}
		g.end("gIBSCBS")
	}
	if t.EstornoIBS != nil || t.EstornoCBS != nil {
		if t.EstornoIBS == nil || t.EstornoCBS == nil {
			g.fail("gEstornoCred exige vIBSEstCred e vCBSEstCred")
		} else {
			g.open("gEstornoCred")
			g.el("vIBSEstCred", g.m("vIBSEstCred", *t.EstornoIBS))
			g.el("vCBSEstCred", g.m("vCBSEstCred", *t.EstornoCBS))
			g.end("gEstornoCred")
		}
	}
	g.end("IBSCBS")
}
