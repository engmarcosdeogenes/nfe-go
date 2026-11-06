package mdfe

import (
	"strconv"
	"time"
)

// dec2 formata TDec_1302 (2 casas). positivo=true exige >0 (TDec_1302Opc).
func (g *gen) dec2(nome string, v float64, positivo bool) string {
	if v < 0 || (positivo && v == 0) || v > 9999999999999 {
		g.fail("%s: valor %v inválido", nome, v)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// idPessoa emite CPF ou CNPJ (um só); estrangeiro é a 3ª opção quando permitida.
func (g *gen) idPessoa(nome, cpf, cnpj, estr string, minEstr int, permiteEstr bool) {
	n := 0
	for _, s := range []string{cpf, cnpj, estr} {
		if s != "" {
			n++
		}
	}
	if n != 1 || (estr != "" && !permiteEstr) {
		g.fail("%s: informe exatamente um entre CPF, CNPJ e idEstrangeiro", nome)
		return
	}
	switch {
	case estr != "":
		g.el("idEstrangeiro", g.txt(nome+" idEstrangeiro", estr, minEstr, 20))
	case cpf != "":
		t, v := g.docID(nome, cpf)
		if t != "CPF" {
			g.fail("%s: CPF inválido", nome)
		}
		g.el("CPF", v)
	default:
		t, v := g.docID(nome, cnpj)
		if t != "CNPJ" {
			g.fail("%s: CNPJ inválido", nome)
		}
		g.el("CNPJ", v)
	}
}

// infANTT segue a ordem de mdfeModalRodoviario_v3.00.xsd: RNTRC, infCIOT,
// valePed, infContratante, infPag.
func (g *gen) infANTT(r Rodoviario, exigeCIOT, exigePag bool) {
	if r.RNTRC != "" && !reRNTRC.MatchString(r.RNTRC) {
		g.fail("RNTRC %q precisa de 8 dígitos", r.RNTRC)
	}
	if exigeCIOT { // NT 2026.001, rej 684
		ok := false
		for _, c := range r.CIOTs {
			ok = ok || c.CIOT != ""
		}
		if !ok {
			g.fail("CIOT obrigatório (NT 2026.001): informe Rodo.CIOTs com o número do CIOT")
		}
	}
	if exigePag && len(r.Pagamentos) == 0 { // NT 2025.001, rej 302/303
		g.fail("infPag obrigatório em carga lotação (NT 2025.001)")
	}
	if r.RNTRC == "" && len(r.CIOTs) == 0 && r.ValePed == nil && len(r.Contratantes) == 0 && len(r.Pagamentos) == 0 {
		return
	}
	g.open("infANTT")
	if r.RNTRC != "" {
		g.el("RNTRC", r.RNTRC)
	}
	for _, c := range r.CIOTs {
		g.open("infCIOT")
		if c.CIOT != "" {
			if len(c.CIOT) != 12 || digitos(c.CIOT) != c.CIOT {
				g.fail("CIOT %q precisa de 12 dígitos", c.CIOT)
			}
			g.el("CIOT", c.CIOT)
		}
		g.idPessoa("infCIOT", c.CPF, c.CNPJ, "", 0, false)
		g.end("infCIOT")
	}
	if v := r.ValePed; v != nil {
		g.open("valePed")
		if len(v.Disp) == 0 {
			g.fail("valePed: informe ao menos um disp")
		}
		for _, d := range v.Disp {
			g.open("disp")
			if f := digitos(d.CNPJForn); len(f) != 14 {
				g.fail("valePed CNPJForn precisa de 14 dígitos")
			} else {
				g.el("CNPJForn", f)
			}
			if d.CNPJPg != "" && d.CPFPg != "" {
				g.fail("valePed: CNPJPg ou CPFPg, não os dois")
			}
			if d.CNPJPg != "" {
				g.el("CNPJPg", digitos(d.CNPJPg))
			}
			if d.CPFPg != "" {
				g.el("CPFPg", digitos(d.CPFPg))
			}
			if d.NCompra != "" {
				if n := len(d.NCompra); n > 20 || digitos(d.NCompra) != d.NCompra {
					g.fail("valePed nCompra: 1 a 20 dígitos")
				}
				g.el("nCompra", d.NCompra)
			}
			g.el("vValePed", g.dec2("vValePed", d.VValePed, false))
			if d.TpValePed != "" {
				if !inSet(d.TpValePed, "01", "04") {
					g.fail("tpValePed %q inválido", d.TpValePed)
				}
				g.el("tpValePed", d.TpValePed)
			}
			g.end("disp")
		}
		if v.CategCombVeic != "" {
			if !inSet(v.CategCombVeic, "02", "04", "06", "07", "08", "10", "11", "12", "13", "14") {
				g.fail("categCombVeic %q inválido", v.CategCombVeic)
			}
			g.el("categCombVeic", v.CategCombVeic)
		}
		g.end("valePed")
	}
	for _, c := range r.Contratantes {
		g.open("infContratante")
		g.opt("xNome", "contratante xNome", c.Nome, 2, 60)
		g.idPessoa("contratante", c.CPF, c.CNPJ, c.IDEstrangeiro, 2, true)
		if c.NroContrato != "" {
			g.open("infContrato")
			g.el("NroContrato", g.txt("NroContrato", c.NroContrato, 2, 20))
			g.el("vContratoGlobal", g.dec2("vContratoGlobal", c.VContratoGlobal, true))
			g.end("infContrato")
		}
		g.end("infContratante")
	}
	for _, p := range r.Pagamentos {
		g.infPag(p)
	}
	g.end("infANTT")
}

func (g *gen) infPag(p Pagamento) {
	g.open("infPag")
	g.opt("xNome", "infPag xNome", p.Nome, 2, 60)
	g.idPessoa("infPag", p.CPF, p.CNPJ, p.IDEstrangeiro, 5, true)
	if len(p.Comp) == 0 {
		g.fail("infPag: precisa de ao menos um Comp")
	}
	for _, c := range p.Comp {
		if !inSet(c.Tipo, "01", "02", "03", "04", "99") {
			g.fail("tpComp %q inválido", c.Tipo)
		}
		g.open("Comp")
		g.el("tpComp", c.Tipo)
		g.el("vComp", g.dec2("vComp", c.Valor, false))
		g.opt("xComp", "xComp", c.XComp, 2, 60)
		g.end("Comp")
	}
	g.el("vContrato", g.dec2("vContrato", p.VContrato, false))
	if p.IndAltoDesemp {
		g.el("indAltoDesemp", "1")
	}
	if p.IndPag != "0" && p.IndPag != "1" {
		g.fail("indPag %q inválido (0 à vista, 1 a prazo)", p.IndPag)
	}
	g.el("indPag", p.IndPag)
	if p.VAdiant > 0 {
		g.el("vAdiant", g.dec2("vAdiant", p.VAdiant, false))
	}
	if p.IndAntecipaAdiant {
		g.el("indAntecipaAdiant", "1")
	}
	for _, pz := range p.Prazos {
		if _, err := time.Parse("2006-01-02", pz.Venc); err != nil || pz.Parcela < 1 || pz.Parcela > 999 {
			g.fail("infPrazo: parcela %d ou vencimento %q inválido", pz.Parcela, pz.Venc)
		}
		g.open("infPrazo")
		g.el("nParcela", strconv.Itoa(1000 + pz.Parcela)[1:])
		g.el("dVenc", pz.Venc)
		g.el("vParcela", g.dec2("vParcela", pz.Valor, true))
		g.end("infPrazo")
	}
	if p.TpAntecip != "" {
		if !inSet(p.TpAntecip, "0", "1", "2") {
			g.fail("tpAntecip %q inválido", p.TpAntecip)
		}
		g.el("tpAntecip", p.TpAntecip)
	}
	g.open("infBanc")
	banco := p.CodBanco != "" || p.CodAgencia != ""
	n := 0
	for _, ok := range []bool{banco, p.CNPJIPEF != "", p.PIX != ""} {
		if ok {
			n++
		}
	}
	if n != 1 {
		g.fail("infBanc: informe exatamente um entre banco+agência, CNPJIPEF e PIX")
	}
	switch {
	case banco:
		if p.CodBanco == "" || p.CodAgencia == "" {
			g.fail("infBanc: banco e agência vêm juntos")
		}
		g.el("codBanco", g.txt("codBanco", p.CodBanco, 3, 5))
		g.el("codAgencia", g.txt("codAgencia", p.CodAgencia, 1, 10))
	case p.CNPJIPEF != "":
		g.el("CNPJIPEF", apenasAlnum(p.CNPJIPEF))
	default:
		g.el("PIX", g.txt("PIX", p.PIX, 2, 60))
	}
	g.end("infBanc")
	g.end("infPag")
}

func (g *gen) seguros(segs []Seguro) {
	for _, s := range segs {
		g.open("seg")
		g.open("infResp")
		if s.RespSeg != "1" && s.RespSeg != "2" {
			g.fail("respSeg %q inválido (1 emitente, 2 contratante)", s.RespSeg)
		}
		g.el("respSeg", s.RespSeg)
		if s.RespCNPJ != "" || s.RespCPF != "" {
			g.idPessoa("seg responsável", s.RespCPF, s.RespCNPJ, "", 0, false)
		}
		g.end("infResp")
		if s.Seguradora != "" {
			g.open("infSeg")
			g.el("xSeg", g.txt("xSeg", s.Seguradora, 1, 30))
			g.el("CNPJ", apenasAlnum(s.CNPJSeg)) // obrigatório no XSD (TCnpjOpc aceita vazio)
			g.end("infSeg")
		}
		g.opt("nApol", "nApol", s.NApol, 1, 20)
		for _, a := range s.NAver {
			g.el("nAver", g.txt("nAver", a, 1, 40))
		}
		g.end("seg")
	}
}
