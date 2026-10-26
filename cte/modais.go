package cte

import (
	"regexp"
	"strconv"
	"strings"
)

// Serialização de infCTeNorm/infModal por modal (ide/modal). O XSD do CT-e
// trata infModal como xs:any; o conteúdo vem dos cteModal*_v4.00.xsd (ordem
// dos elementos igual à do XSD). Número da rejeição do MOC CT-e 4.00 para
// estes grupos: não confirmado (sem acesso ao MOC); as mensagens citam o
// campo e o limite do XSD.

var (
	reNViag  = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)
	reNONU   = regexp.MustCompile(`^([0-9]{4}|ND)$`)
	reIEFerr = regexp.MustCompile(`^[0-9]{2,14}$`)
)

func (g *gen) infModal(e Entrada) {
	grupos := map[string]bool{"01": e.Rodo != nil, "02": e.Aereo != nil, "03": e.Aquaviario != nil,
		"04": e.Ferroviario != nil, "05": e.Dutoviario != nil, "06": e.Multimodal != nil}
	nomes := map[string]string{"01": "Rodo", "02": "Aereo", "03": "Aquaviario", "04": "Ferroviario", "05": "Dutoviario", "06": "Multimodal"}
	for m, ok := range grupos {
		if ok && m != e.Modal {
			g.fail("modal %s: o grupo %s é do modal %s; só o grupo do modal informado pode vir preenchido", e.Modal, nomes[m], m)
		}
	}
	if !grupos[e.Modal] && e.Modal != "01" {
		g.fail("modal %s exige o grupo %s", e.Modal, nomes[e.Modal])
		return
	}
	switch e.Modal {
	case "01":
		g.rodo(e.Rodo)
	case "02":
		g.aereo(e.Aereo)
	case "03":
		g.aquav(e.Aquaviario)
	case "04":
		g.ferrov(e.Ferroviario)
	case "05":
		g.duto(e.Dutoviario)
	case "06":
		// tpServ 4 é do CT-e que aponta para o multimodal; o próprio multimodal não se vincula a si. Rejeição do MOC: não confirmada.
		if e.TpServ == "4" {
			g.fail("modal 06 (multimodal) não combina com tpServ 4 (vinculado a multimodal): tpServ 4 é do CT-e que aponta para o multimodal (regra não confirmada no MOC)")
		}
		g.multimodal(e.Multimodal)
	}
}

func (g *gen) rodo(r *Rodoviario) {
	if r == nil {
		g.fail("modal 01 exige o grupo Rodo (RNTRC)")
		r = &Rodoviario{}
	}
	if !reRNTRC.MatchString(strings.ToUpper(r.RNTRC)) {
		g.fail("RNTRC %q inválido: 8 dígitos ou ISENTO (TRNTRC)", r.RNTRC)
	}
	g.raw(`<infModal versaoModal="4.00"><rodo>`)
	g.el("RNTRC", strings.ToUpper(r.RNTRC))
	if len(r.OCC) > 10 {
		g.fail("occ: no máximo 10")
	}
	for _, o := range r.OCC {
		if !reNOcc.MatchString(o.NOcc) {
			g.fail("occ nOcc %q inválido ([1-9][0-9]{0,5})", o.NOcc)
		}
		g.open("occ")
		g.opt("serie", "occ serie", o.Serie, 1, 3)
		g.el("nOcc", o.NOcc)
		g.el("dEmi", g.data("occ dEmi", o.DEmi))
		g.open("emiOcc")
		t, v := g.docID("occ emitente", o.CNPJ)
		if t != "CNPJ" {
			g.fail("emiOcc exige CNPJ")
		}
		g.el("CNPJ", v)
		g.opt("cInt", "occ cInt", o.CInt, 1, 10)
		if ie := g.ie("occ IE", o.IE, false); ie == "" {
			g.fail("emiOcc: IE obrigatória")
		} else {
			g.el("IE", ie)
		}
		g.uf("occ UF", o.UF)
		g.el("UF", o.UF)
		if f := g.fone("occ", o.Fone, reFoneT); f != "" {
			g.el("fone", f)
		}
		g.end("emiOcc")
		g.end("occ")
	}
	g.raw(`</rodo></infModal>`)
}

func (g *gen) aereo(a *Aereo) {
	g.raw(`<infModal versaoModal="4.00"><aereo>`)
	if a.NMinu != "" {
		if !soDigitos(a.NMinu, 9) {
			g.fail("aereo nMinu %q: precisa de 9 dígitos", a.NMinu)
		}
		g.el("nMinu", a.NMinu)
	}
	if a.NOCA != "" {
		if !soDigitos(a.NOCA, 11) {
			g.fail("aereo nOCA %q: precisa de 11 dígitos", a.NOCA)
		}
		g.el("nOCA", a.NOCA)
	}
	g.el("dPrevAereo", g.data("aereo dPrevAereo (obrigatório)", a.DPrevAereo))
	g.open("natCarga")
	g.opt("xDime", "aereo xDime", a.XDime, 5, 14)
	for _, c := range a.CInfManu {
		if !inSet(c, "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "99") {
			g.fail("aereo cInfManu %q inválido (01..15 ou 99)", c)
		}
		g.el("cInfManu", c)
	}
	g.end("natCarga")
	t := a.Tarifa
	if t == nil {
		g.fail("aereo: tarifa (CL, vTar) é obrigatória")
		t = &TarifaAerea{}
	}
	if !inSet(t.CL, "M", "G", "E") {
		g.fail("aereo tarifa/CL %q inválido (M, G ou E)", t.CL)
	}
	g.open("tarifa")
	g.el("CL", t.CL)
	g.opt("cTar", "aereo cTar", t.CTar, 1, 4)
	g.el("vTar", g.m("aereo tarifa/vTar", t.VTar))
	g.end("tarifa")
	for _, p := range a.Peri {
		if !reNONU.MatchString(p.NONU) {
			g.fail("aereo peri nONU %q inválido (4 dígitos ou ND)", p.NONU)
		}
		if !inSet(p.UniAP, "1", "2", "3", "4", "5") {
			g.fail("aereo peri uniAP %q inválido (1..5)", p.UniAP)
		}
		g.open("peri")
		g.el("nONU", p.NONU)
		g.el("qTotEmb", g.txt("aereo peri qTotEmb", p.QTotEmb, 1, 20))
		g.open("infTotAP")
		g.el("qTotProd", g.qtd4("aereo peri qTotProd", p.QTotProd))
		g.el("uniAP", p.UniAP)
		g.end("infTotAP")
		g.end("peri")
	}
	g.raw(`</aereo></infModal>`)
}

// unidRat: TDec_0302_0303.
func (g *gen) unidRat(nome string, v *float64) {
	if v == nil {
		return
	}
	if *v < 0 || *v >= 1000 {
		g.fail("%s: %v fora de 0..999.99", nome, *v)
	}
	g.el("unidRat", strconv.FormatFloat(*v, 'f', 2, 64))
}

func (g *gen) aquav(a *Aquaviario) {
	g.raw(`<infModal versaoModal="4.00"><aquav>`)
	g.el("vPrest", g.m("aquav vPrest", a.VPrest))
	g.el("vAFRMM", g.m("aquav vAFRMM", a.VAFRMM))
	g.el("xNavio", g.txt("aquav xNavio", a.XNavio, 1, 60))
	if len(a.Balsa) > 3 {
		g.fail("aquav: balsa no máximo 3")
	}
	for _, b := range a.Balsa {
		g.open("balsa")
		g.el("xBalsa", g.txt("aquav xBalsa", b, 1, 60))
		g.end("balsa")
	}
	if a.NViag != "" {
		if !reNViag.MatchString(a.NViag) {
			g.fail("aquav nViag %q inválido ([1-9][0-9]{0,9})", a.NViag)
		}
		g.el("nViag", a.NViag)
	}
	if !inSet(a.Direc, "N", "S", "L", "O") {
		g.fail("aquav direc %q inválido (N, S, L ou O)", a.Direc)
	}
	g.el("direc", a.Direc)
	g.el("irin", g.txt("aquav irin", a.Irin, 1, 10))
	for _, d := range a.DetCont {
		if !reContainer(d.NCont) || len(d.NCont) > 20 {
			g.fail("aquav detCont nCont %q inválido ([A-Z0-9], 1 a 20)", d.NCont)
		}
		g.open("detCont")
		g.el("nCont", d.NCont)
		if len(d.Lacres) > 3 {
			g.fail("aquav detCont: lacre no máximo 3")
		}
		for _, l := range d.Lacres {
			g.open("lacre")
			g.el("nLacre", g.txt("aquav nLacre", l, 1, 20))
			g.end("lacre")
		}
		switch {
		case len(d.NF) > 0 && len(d.NFe) > 0:
			g.fail("aquav detCont %s: infDoc aceita infNF ou infNFe, não os dois (choice do XSD)", d.NCont)
		case len(d.NF)+len(d.NFe) > 0:
			g.open("infDoc")
			for _, n := range d.NF {
				g.open("infNF")
				g.el("serie", g.txt("aquav infNF serie", n.Serie, 1, 3))
				g.el("nDoc", g.txt("aquav infNF nDoc", n.NDoc, 1, 20))
				g.unidRat("aquav infNF unidRat", n.UnidRat)
				g.end("infNF")
			}
			for _, n := range d.NFe {
				g.open("infNFe")
				g.el("chave", g.chave("aquav infNFe", n.Chave))
				g.unidRat("aquav infNFe unidRat", n.UnidRat)
				g.end("infNFe")
			}
			g.end("infDoc")
		}
		g.end("detCont")
	}
	if a.TpNav != "" {
		if !inSet(a.TpNav, "0", "1") {
			g.fail("aquav tpNav %q inválido (0 interior, 1 cabotagem)", a.TpNav)
		}
		g.el("tpNav", a.TpNav)
	}
	g.raw(`</aquav></infModal>`)
}

func (g *gen) ferrov(f *Ferroviario) {
	g.raw(`<infModal versaoModal="4.00"><ferrov>`)
	if !inSet(f.TpTraf, "0", "1", "2", "3") {
		g.fail("ferrov tpTraf %q inválido (0..3)", f.TpTraf)
	}
	g.el("tpTraf", f.TpTraf)
	if t := f.TrafMut; t != nil {
		if !inSet(t.RespFat, "1", "2") {
			g.fail("ferrov trafMut respFat %q inválido (1 ou 2)", t.RespFat)
		}
		if !inSet(t.FerrEmi, "1", "2") {
			g.fail("ferrov trafMut ferrEmi %q inválido (1 ou 2)", t.FerrEmi)
		}
		g.open("trafMut")
		g.el("respFat", t.RespFat)
		g.el("ferrEmi", t.FerrEmi)
		g.el("vFrete", g.m("ferrov trafMut vFrete", t.VFrete))
		if t.ChCTeFerroOrigem != "" {
			g.el("chCTeFerroOrigem", g.chave("ferrov chCTeFerroOrigem", t.ChCTeFerroOrigem))
		}
		for _, fe := range t.FerroEnv {
			g.open("ferroEnv")
			tag, v := g.docID("ferroEnv", fe.CNPJ)
			if tag != "CNPJ" {
				g.fail("ferroEnv exige CNPJ")
			}
			g.el("CNPJ", v)
			g.opt("cInt", "ferroEnv cInt", fe.CInt, 1, 10)
			if fe.IE != "" {
				ie := digitos(fe.IE)
				if !reIEFerr.MatchString(ie) {
					g.fail("ferroEnv IE %q inválida (2 a 14 dígitos, TIe)", fe.IE)
				}
				g.el("IE", ie)
			}
			g.el("xNome", g.txt("ferroEnv xNome", fe.Nome, 2, 60))
			n := fe.End
			g.open("enderFerro")
			g.el("xLgr", g.txt("enderFerro xLgr", n.Logradouro, 2, 255))
			g.opt("nro", "enderFerro nro", n.Numero, 1, 60)
			g.opt("xCpl", "enderFerro xCpl", n.Complemento, 1, 60)
			g.opt("xBairro", "enderFerro xBairro", n.Bairro, 2, 60)
			cm, xm := g.mun("enderFerro", n.CodigoMun, n.Municipio)
			g.el("cMun", cm)
			g.el("xMun", xm)
			g.el("CEP", g.cep("enderFerro", n.CEP))
			if n.UF == "EX" {
				g.fail("enderFerro UF: EX não é aceita (TUf)")
			}
			g.uf("enderFerro UF", n.UF)
			g.el("UF", n.UF)
			g.end("enderFerro")
			g.end("ferroEnv")
		}
		g.end("trafMut")
	}
	g.el("fluxo", g.txt("ferrov fluxo", f.Fluxo, 1, 10))
	g.raw(`</ferrov></infModal>`)
}

func (g *gen) duto(d *Dutoviario) {
	g.raw(`<infModal versaoModal="4.00"><duto>`)
	if d.VTar != nil {
		v := *d.VTar
		if v <= 0 || v >= 1e9 {
			g.fail("duto vTar %v fora de 0.000001..999999999 (TDec_0906Opc não aceita 0)", v)
		}
		g.el("vTar", strconv.FormatFloat(v, 'f', 6, 64))
	}
	g.el("dIni", g.data("duto dIni", d.DIni))
	g.el("dFim", g.data("duto dFim", d.DFim))
	if d.DFim < d.DIni { // AAAA-MM-DD compara como texto; rejeição do MOC: não confirmada
		g.fail("duto: dFim %s anterior a dIni %s (regra não confirmada no MOC)", d.DFim, d.DIni)
	}
	if d.ClassDuto != "" {
		if !inSet(d.ClassDuto, "1", "2", "3") {
			g.fail("duto classDuto %q inválido (1..3)", d.ClassDuto)
		}
		g.el("classDuto", d.ClassDuto)
	}
	if d.TpContratacao != "" {
		if !inSet(d.TpContratacao, "0", "1", "2") {
			g.fail("duto tpContratacao %q inválido (0..2)", d.TpContratacao)
		}
		g.el("tpContratacao", d.TpContratacao)
	}
	g.opt("codPontoEntrada", "duto codPontoEntrada", d.CodPontoEntrada, 2, 20)
	g.opt("codPontoSaida", "duto codPontoSaida", d.CodPontoSaida, 2, 20)
	g.opt("nContrato", "duto nContrato", d.NContrato, 2, 20)
	g.raw(`</duto></infModal>`)
}

func (g *gen) multimodal(m *Multimodal) {
	g.raw(`<infModal versaoModal="4.00"><multimodal>`)
	g.el("COTM", g.txt("multimodal COTM", m.COTM, 1, 20))
	if !inSet(m.IndNegociavel, "0", "1") {
		g.fail("multimodal indNegociavel %q inválido (0 ou 1)", m.IndNegociavel)
	}
	g.el("indNegociavel", m.IndNegociavel)
	if s := m.Seg; s != nil {
		g.open("seg")
		g.open("infSeg")
		g.el("xSeg", g.txt("multimodal xSeg", s.XSeg, 1, 30))
		if s.CNPJ == "" {
			g.el("CNPJ", "")
		} else {
			t, v := g.docID("multimodal seguradora", s.CNPJ)
			if t != "CNPJ" {
				g.fail("multimodal seg/CNPJ: precisa ser CNPJ (TCnpjOpc)")
			}
			g.el("CNPJ", v)
		}
		g.end("infSeg")
		g.el("nApol", g.txt("multimodal nApol", s.NApol, 1, 20))
		g.el("nAver", g.txt("multimodal nAver", s.NAver, 1, 20))
		g.end("seg")
	}
	g.raw(`</multimodal></infModal>`)
}
