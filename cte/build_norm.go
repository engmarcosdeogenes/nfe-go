package cte

import "strings"

var escAttr = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// complemento: tpCTe=1. O XSD troca infCTeNorm por infCteComp (1 a 10 chaves
// dos CT-e complementados). Conferido: o MOC exige a chave; rejeição: não
// confirmada.
func (g *gen) complemento(e Entrada) {
	if n := len(e.InfCteComp); n < 1 || n > 10 {
		g.fail("complemento (tpCTe 1) exige 1 a 10 chaves em InfCteComp, veio %d", n)
	}
	if e.Carga != nil || e.Rodo != nil || e.Aereo != nil || e.Aquaviario != nil || e.Ferroviario != nil || e.Dutoviario != nil || e.Multimodal != nil || len(e.Docs.NFe)+len(e.Docs.NF)+len(e.Docs.Outros)+len(e.Docs.DCe) > 0 ||
		len(e.DocAnt) > 0 || e.InfCteSub != nil || e.Cobr != nil || len(e.VeicNovos) > 0 || len(e.InfServVinc) > 0 {
		g.fail("complemento (tpCTe 1) não leva infCTeNorm (carga, documentos, modal, cobrança, etc.)")
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

func (g *gen) unidCarga(tag string, us []UnidCarga) {
	for _, u := range us {
		if !inSet(u.Tp, "1", "2", "3", "4") || !reContainer(u.ID) {
			g.fail("infUnidCarga: tpUnidCarga 1-4 e id [A-Z0-9]+ (veio %q/%q)", u.Tp, u.ID)
		}
		g.open(tag)
		g.el("tpUnidCarga", u.Tp)
		g.el("idUnidCarga", g.txt("idUnidCarga", u.ID, 1, 20))
		g.lacres("lacUnidCarga", u.Lacres)
		g.rat(u.QtdRat)
		g.end(tag)
	}
}

func reContainer(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func (g *gen) lacres(tag string, ls []string) {
	for _, l := range ls {
		g.open(tag)
		g.el("nLacre", g.txt("nLacre", l, 1, 20))
		g.end(tag)
	}
}

func (g *gen) rat(v float64) {
	if v != 0 {
		if v < 0 || v >= 1000 {
			g.fail("qtdRat fora de 0..999.999")
		}
		g.el("qtdRat", fmtRat(v))
	}
}

// unidades: infUnidCarga OU infUnidTransp por documento (choice do XSD).
func (g *gen) unidades(uc []UnidCarga, ut []UnidTransp) {
	if len(uc) > 0 && len(ut) > 0 {
		g.fail("documento: infUnidCarga e infUnidTransp são excludentes no XSD")
	}
	g.unidCarga("infUnidCarga", uc)
	for _, u := range ut {
		if !inSet(u.Tp, "1", "2", "3", "4", "5", "6", "7") || !reContainer(u.ID) {
			g.fail("infUnidTransp: tpUnidTransp 1-7 e id [A-Z0-9]+")
		}
		g.open("infUnidTransp")
		g.el("tpUnidTransp", u.Tp)
		g.el("idUnidTransp", g.txt("idUnidTransp", u.ID, 1, 20))
		g.lacres("lacUnidTransp", u.Lacres)
		g.unidCarga("infUnidCarga", u.Carga)
		g.rat(u.QtdRat)
		g.end("infUnidTransp")
	}
}

func (g *gen) docs(d Docs) {
	tipos := 0
	for _, n := range []int{len(d.NF), len(d.NFe), len(d.Outros), len(d.DCe)} {
		if n > 0 {
			tipos++
		}
	}
	if tipos == 0 {
		return
	}
	if tipos > 1 {
		g.fail("infDoc: o XSD aceita só um tipo de documento (infNF, infNFe, infOutros ou infDCe), não uma mistura")
	}
	g.open("infDoc")
	vistas := map[string]bool{}
	for _, n := range d.NF {
		if !inSet(n.Mod, "01", "04") || !reCFOP.MatchString(n.NCFOP) {
			g.fail("infNF: mod 01 ou 04 e nCFOP válido")
		}
		g.open("infNF")
		g.opt("nRoma", "nRoma", n.NRoma, 1, 20)
		g.opt("nPed", "nPed", n.NPed, 1, 20)
		g.el("mod", n.Mod)
		g.el("serie", g.txt("infNF serie", n.Serie, 1, 3))
		g.el("nDoc", g.txt("infNF nDoc", n.NDoc, 1, 20))
		g.el("dEmi", g.data("infNF dEmi", n.DEmi))
		for _, c := range []struct {
			t string
			v float64
		}{{"vBC", n.VBC}, {"vICMS", n.VICMS}, {"vBCST", n.VBCST}, {"vST", n.VST}, {"vProd", n.VProd}, {"vNF", n.VNF}} {
			g.el(c.t, g.m("infNF "+c.t, c.v))
		}
		g.el("nCFOP", n.NCFOP)
		if n.NPeso != 0 {
			g.el("nPeso", fmtPeso(n.NPeso))
		}
		g.pinPrev(n.PIN, n.DPrev)
		g.unidades(n.UnidCarga, n.UnidTransp)
		g.end("infNF")
	}
	for _, n := range d.NFe {
		if vistas[n.Chave] {
			g.fail("NF-e %s repetida no CT-e", n.Chave)
		}
		vistas[n.Chave] = true
		g.open("infNFe")
		g.el("chave", g.chave("infNFe", n.Chave))
		g.pinPrev(n.PIN, n.DPrev)
		g.unidades(n.UnidCarga, n.UnidTransp)
		g.end("infNFe")
	}
	for _, o := range d.Outros {
		if !inSet(o.TpDoc, "00", "10", "59", "65", "99") {
			g.fail("infOutros: tpDoc %q inválido (00, 10, 59, 65, 99)", o.TpDoc)
		}
		if o.TpDoc == "99" && o.Desc == "" {
			g.fail("infOutros tpDoc 99 exige descOutros")
		}
		g.open("infOutros")
		g.el("tpDoc", o.TpDoc)
		g.opt("descOutros", "descOutros", o.Desc, 1, 100)
		g.opt("nDoc", "infOutros nDoc", o.NDoc, 1, 20)
		if o.DEmi != "" {
			g.el("dEmi", g.data("infOutros dEmi", o.DEmi))
		}
		if o.VDocFisc != 0 {
			g.el("vDocFisc", g.mpos("vDocFisc", o.VDocFisc))
		}
		if o.DPrev != "" {
			g.el("dPrev", g.data("dPrev", o.DPrev))
		}
		g.unidades(o.UnidCarga, o.UnidTransp)
		g.end("infOutros")
	}
	for _, k := range d.DCe {
		if vistas[k] {
			g.fail("DC-e %s repetido no CT-e", k)
		}
		vistas[k] = true
		g.open("infDCe")
		g.el("chave", g.chave("infDCe", k))
		g.end("infDCe")
	}
	g.end("infDoc")
}

func (g *gen) pinPrev(pin, prev string) {
	if pin != "" {
		if d := digitos(pin); d != pin || len(d) < 2 || len(d) > 9 || d[0] == '0' {
			g.fail("PIN %q inválido (2 a 9 dígitos, sem zero à esquerda)", pin)
		}
		g.el("PIN", pin)
	}
	if prev != "" {
		g.el("dPrev", g.data("dPrev", prev))
	}
}

func (g *gen) docAnt(e Entrada) {
	if len(e.DocAnt) == 0 {
		// tpServ 1, 2 e 3 trabalham sobre documento de transporte anterior
		// (MOC CT-e 4.00; rejeição específica: não confirmada).
		if inSet(e.TpServ, "1", "2", "3") {
			g.fail("tpServ %s (subcontratação/redespacho) exige docAnt com os documentos de transporte anteriores", e.TpServ)
		}
		return
	}
	g.open("docAnt")
	for _, d := range e.DocAnt {
		g.open("emiDocAnt")
		if (d.CNPJ == "") == (d.CPF == "") {
			g.fail("emiDocAnt: informe CNPJ ou CPF")
		}
		t, v := g.docID("emiDocAnt", firstNonEmpty(d.CNPJ, d.CPF))
		g.el(t, v)
		if (d.IE == "") != (d.UF == "") {
			g.fail("emiDocAnt: IE e UF vêm juntas")
		}
		if d.IE != "" {
			g.el("IE", g.ie("emiDocAnt IE", d.IE, false))
			g.uf("emiDocAnt UF", d.UF)
			g.el("UF", d.UF)
		}
		g.el("xNome", g.txt("emiDocAnt xNome", d.Nome, 1, 60))
		if len(d.Papel)+len(d.Eletronico) == 0 {
			g.fail("emiDocAnt %s: sem documento (idDocAnt)", d.Nome)
		}
		if len(d.Papel) > 0 {
			g.open("idDocAnt")
			for _, p := range d.Papel {
				if !inSet(p.TpDoc, "07", "08", "09", "10", "11", "12", "13") {
					g.fail("idDocAntPap: tpDoc %q inválido (07..13)", p.TpDoc)
				}
				g.open("idDocAntPap")
				g.el("tpDoc", p.TpDoc)
				g.el("serie", g.txt("serie", p.Serie, 1, 3))
				g.opt("subser", "subser", p.Subser, 1, 2)
				g.el("nDoc", g.txt("nDoc", p.NDoc, 1, 30))
				g.el("dEmi", g.data("dEmi", p.DEmi))
				g.end("idDocAntPap")
			}
			g.end("idDocAnt")
		}
		if len(d.Eletronico) > 0 {
			g.open("idDocAnt")
			for _, k := range d.Eletronico {
				g.open("idDocAntEle")
				g.el("chCTe", g.chave("idDocAntEle", k))
				g.end("idDocAntEle")
			}
			g.end("idDocAnt")
		}
		g.end("emiDocAnt")
	}
	g.end("docAnt")
}

// norm escreve infCTeNorm (tpCTe 0 e 3).
func (g *gen) norm(e Entrada) {
	if len(e.InfCteComp) > 0 {
		g.fail("InfCteComp só no complemento (tpCTe 1)")
	}
	// Substituição: chave do CT-e substituído obrigatória; fora dela, proibido.
	switch {
	case e.TpCTe == "3" && (e.InfCteSub == nil || e.InfCteSub.ChCte == ""):
		g.fail("substituição (tpCTe 3) exige InfCteSub.ChCte (chave do CT-e substituído)")
	case e.TpCTe != "3" && e.InfCteSub != nil:
		g.fail("InfCteSub só na substituição (tpCTe 3)")
	}
	if e.TpServ == "4" && len(e.InfServVinc) == 0 {
		g.fail("tpServ 4 (vinculado a multimodal) exige infServVinc com a chave do CT-e multimodal")
	}
	if e.TpServ != "4" && len(e.InfServVinc) > 0 {
		g.fail("infServVinc só com tpServ 4")
	}
	g.open("infCTeNorm")
	c := e.Carga
	if c == nil {
		g.fail("infCarga obrigatória no CT-e normal/substituição")
		c = &Carga{}
	}
	g.open("infCarga")
	if c.VCarga != nil {
		g.el("vCarga", g.m("vCarga", *c.VCarga))
	}
	g.el("proPred", g.txt("proPred", c.ProPred, 1, 60))
	g.opt("xOutCat", "xOutCat", c.XOutCat, 1, 30)
	if len(c.Quantidades) < 1 {
		g.fail("infCarga: infQ precisa de ao menos 1 item")
	}
	for _, q := range c.Quantidades {
		if !inSet(q.CUnid, "00", "01", "02", "03", "04", "05") {
			g.fail("infQ: cUnid %q inválido (00..05)", q.CUnid)
		}
		g.open("infQ")
		g.el("cUnid", q.CUnid)
		g.el("tpMed", g.txt("tpMed", q.TpMed, 1, 20))
		g.el("qCarga", g.qtd4("qCarga", q.Qtd))
		g.end("infQ")
	}
	if c.VCargaAverb != nil {
		g.el("vCargaAverb", g.mpos("vCargaAverb", *c.VCargaAverb))
	}
	g.end("infCarga")
	g.docs(e.Docs)
	g.docAnt(e)

	g.infModal(e)

	for _, v := range e.VeicNovos {
		if !reContainer(v.Chassi) {
			g.fail("veicNovos: chassi [A-Z0-9]+")
		}
		g.open("veicNovos")
		g.el("chassi", v.Chassi)
		g.el("cCor", g.txt("cCor", v.CCor, 1, 4))
		g.el("xCor", g.txt("xCor", v.XCor, 1, 40))
		g.el("cMod", g.txt("cMod", v.CMod, 1, 6))
		g.el("vUnit", g.m("vUnit", v.VUnit))
		g.el("vFrete", g.m("vFrete", v.VFrete))
		g.end("veicNovos")
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
	if s := e.InfCteSub; s != nil {
		g.open("infCteSub")
		g.el("chCte", g.chave("infCteSub", s.ChCte))
		if s.IndAlteraToma {
			g.el("indAlteraToma", "1")
		}
		g.end("infCteSub")
	}
	if e.InfGlobal != "" {
		g.open("infGlobalizado")
		g.el("xObs", g.txt("infGlobalizado xObs", e.InfGlobal, 15, 256))
		g.end("infGlobalizado")
	}
	if len(e.InfServVinc) > 0 {
		g.open("infServVinc")
		for _, k := range e.InfServVinc {
			g.open("infCTeMultimodal")
			g.el("chCTeMultimodal", g.chave("chCTeMultimodal", k))
			g.end("infCTeMultimodal")
		}
		g.end("infServVinc")
	}
	g.end("infCTeNorm")
}
