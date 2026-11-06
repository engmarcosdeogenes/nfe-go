package mdfe

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// dataCIOT: a partir daqui o CIOT é obrigatório (NT 2026.001, rej 684).
var dataCIOT = time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

var reGTIN = regexp.MustCompile(`^([0-9]{8}|[0-9]{12,14})$`)

// Build monta o MDF-e (sem assinatura, sem declaração <?xml?>) e a chave.
func Build(e Entrada) (xmlOut []byte, chave string, err error) {
	g := &gen{}
	if !inSet(e.TpEmit, "1", "2", "3") {
		return nil, "", fmt.Errorf("mdfe: tpEmit %q inválido (1, 2 ou 3)", e.TpEmit)
	}
	if e.TpEmis == "" {
		e.TpEmis = "1"
	}
	if !inSet(e.TpEmis, "1", "2") {
		return nil, "", fmt.Errorf("mdfe: tpEmis %q não suportado (1 normal, 2 contingência offline)", e.TpEmis)
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		return nil, "", fmt.Errorf("mdfe: contingência (tpEmis 2) exige QRSign (assinatura RSA-SHA1 da chave, base64)")
	}
	if e.TpTransp != "" && !inSet(e.TpTransp, "1", "2", "3") {
		return nil, "", fmt.Errorf("mdfe: tpTransp %q inválido (1 ETC, 2 TAC, 3 CTC)", e.TpTransp)
	}
	// Regras que dependem do conjunto: contagem de documentos e flags.
	nCTe, nNFe, nMDFe := 0, 0, 0
	for _, d := range e.Descargas {
		nCTe, nNFe, nMDFe = nCTe+len(d.CTes), nNFe+len(d.NFes), nMDFe+len(d.MDFes)
	}
	if e.TpEmit == "2" && nCTe > 0 {
		g.fail("tpEmit=2 (carga própria) não pode vincular CT-e (rej 639)")
	}
	lotacao := nCTe+nNFe+nMDFe == 1
	exigeProd := e.TpEmit != "2" || e.TpTransp != "" // rej 725/301
	exigePag := lotacao && exigeProd                 // NT 2025.001, rej 302/303
	exigeCIOT := exigePag && (e.ExigirCIOT || !e.DhEmi.Before(dataCIOT))
	if e.TpEmit != "2" && len(e.Seguros) == 0 {
		g.fail("seguro (seg) obrigatório para tpEmit 1 e 3 (rej 698/699)")
	}
	// tpTransp x proprietário da tração (MOC 3.00b Anexo I F18-F20, rej 743/744/745).
	switch pr := e.Rodo.Tracao.Prop; {
	case e.TpTransp == "":
	case pr == nil:
		g.fail("tpTransp só se informa com proprietário na tração (rej 745)")
	case pr.CPF != "" && e.TpTransp != "2":
		g.fail("proprietário CPF exige tpTransp 2 (TAC) (rej 743)")
	case pr.CNPJ != "" && e.TpTransp == "2":
		g.fail("proprietário CNPJ exige tpTransp 1 (ETC) ou 3 (CTC) (rej 744)")
	}
	if e.TpAmb != "1" && e.TpAmb != "2" {
		return nil, "", fmt.Errorf("mdfe: tpAmb %q inválido (1 ou 2)", e.TpAmb)
	}
	em := e.Emitente
	g.uf("UF do emitente", em.End.UF)
	g.uf("UFIni", e.UFIni)
	g.uf("UFFim", e.UFFim)
	serie := g.inteiro("serie", e.Serie, 0, 999)
	nmdf := g.inteiro("nMDF", e.NMDF, 1, 999999999)
	if (em.CNPJ == "") == (em.CPF == "") {
		g.fail("emitente: informe CNPJ ou CPF (um dos dois)")
	}
	idTag, idEmit := g.docID("emitente", firstNonEmpty(em.CNPJ, em.CPF))
	if g.err != nil {
		return nil, "", g.err
	}
	// Na chave o documento tem 14 posições; CPF entra com 3 zeros à esquerda
	// (composição da chave no MOC: não confirmado no XSD).
	idChave := idEmit
	if idTag == "CPF" {
		idChave = "000" + idEmit
	}
	if digitos(idChave) != idChave { // shortcut: remover quando o Id de infMDFe aceitar letras no XSD
		g.fail("emitente: CNPJ alfanumérico recusado, o Id de infMDFe no XSD 3.00 ainda é MDFe[0-9]{44}")
	}

	dh := e.DhEmi.In(builder.FusoUF(em.End.UF)) // fuso da UF do emitente: dhEmi futuro é rejeitado
	ch := builder.NovaChave(em.End.UF, idChave, serie, nmdf, e.TpEmis, "58", dh)
	if e.CMDF != "" {
		ch = comCNF(ch, e.CMDF, g)
	}
	chave = ch.String()

	g.raw(`<MDFe xmlns="` + nsMDFe + `"><infMDFe versao="3.00" Id="MDFe` + chave + `">`)
	g.open("ide")
	g.el("cUF", ch.CUF)
	g.el("tpAmb", e.TpAmb)
	g.el("tpEmit", e.TpEmit)
	if e.TpTransp != "" {
		g.el("tpTransp", e.TpTransp)
	}
	g.el("mod", "58")
	g.el("serie", serie)
	g.el("nMDF", nmdf)
	g.el("cMDF", ch.CNF)
	g.el("cDV", ch.CDV)
	g.el("modal", "1")
	g.el("dhEmi", dh.Format(layout))
	g.el("tpEmis", e.TpEmis)
	g.el("procEmi", "0")
	g.el("verProc", "nfe-go")
	g.el("UFIni", e.UFIni)
	g.el("UFFim", e.UFFim)
	if n := len(e.MunCarrega); n < 1 || n > 50 {
		g.fail("municípios de carregamento: 1 a 50, veio %d", n)
	}
	for _, m := range e.MunCarrega {
		c, x := g.mun("município de carregamento", m.CodigoMun, m.Nome)
		g.open("infMunCarrega")
		g.el("cMunCarrega", c)
		g.el("xMunCarrega", x)
		g.end("infMunCarrega")
	}
	if len(e.UFPercurso) > 25 {
		g.fail("percurso: no máximo 25 UFs")
	}
	vistas := map[string]bool{}
	for _, u := range e.UFPercurso {
		g.uf("UF de percurso", u)
		// Percurso repetindo UFIni/UFFim (ou a si mesmo) é rejeitado; o
		// código da rejeição NÃO foi confirmado no MOC.
		if u == e.UFIni || u == e.UFFim || vistas[u] {
			g.fail("percurso: UF %s repetida ou igual a UFIni/UFFim", u)
		}
		vistas[u] = true
		g.open("infPercurso")
		g.el("UFPer", u)
		g.end("infPercurso")
	}
	if e.DhIniViagem != nil {
		g.el("dhIniViagem", e.DhIniViagem.In(dh.Location()).Format(layout))
	}
	g.end("ide")

	g.open("emit")
	g.el(idTag, idEmit)
	if ie := digitos(em.IE); ie != "" {
		if len(ie) < 2 || len(ie) > 14 {
			g.fail("IE do emitente: 2 a 14 dígitos")
		}
		g.el("IE", ie)
	}
	// tpAmb=2: xNome NÃO é trocado (diferente da NF-e); não há regra no XSD
	// nem foi achada no MOC.
	g.el("xNome", g.txt("emitente xNome", em.Nome, 2, 60))
	g.opt("xFant", "emitente xFant", em.Fantasia, 1, 60)
	g.open("enderEmit")
	en := em.End
	g.el("xLgr", g.txt("xLgr", en.Logradouro, 2, 60))
	g.el("nro", g.txt("nro", en.Numero, 1, 60))
	g.opt("xCpl", "xCpl", en.Complemento, 1, 60)
	g.el("xBairro", g.txt("xBairro", en.Bairro, 2, 60))
	c, x := g.mun("município do emitente", en.CodigoMun, en.Municipio)
	g.el("cMun", c)
	g.el("xMun", x)
	if cep := builder.FormatarCEP(en.CEP); cep != "" {
		if len(cep) != 8 {
			g.fail("CEP do emitente precisa de 8 dígitos")
		}
		g.el("CEP", cep)
	}
	g.el("UF", en.UF)
	if f := digitos(en.Fone); f != "" {
		if !reFone.MatchString(f) {
			g.fail("fone do emitente: 7 a 12 dígitos")
		}
		g.el("fone", f)
	}
	if en.Email != "" {
		if mail := g.txt("email", en.Email, 6, 60); reEmail.MatchString(mail) {
			g.el("email", mail)
		} else {
			g.fail("email do emitente inválido")
		}
	}
	g.end("enderEmit")
	g.end("emit")

	g.raw(`<infModal versaoModal="3.00"><rodo>`)
	r := e.Rodo
	g.infANTT(r, exigeCIOT, exigePag)
	g.veiculo("veicTracao", r.Tracao, true, r.Condutores)
	if len(r.Reboques) > 3 {
		g.fail("reboques: no máximo 3")
	}
	for _, rb := range r.Reboques {
		g.veiculo("veicReboque", rb, false, nil)
	}
	g.raw("</rodo></infModal>")

	if n := len(e.Descargas); n < 1 || n > 1000 {
		g.fail("municípios de descarga: 1 a 1000, veio %d", n)
	}
	g.open("infDoc")
	vistas2 := map[string]bool{}
	doc := func(tag, chTag, k string, opc DocOpc, comBarra bool) {
		if !reChave.MatchString(k) {
			g.fail("chave %q precisa de 44 posições (TChNFe)", k)
		}
		if vistas2[k] {
			g.fail("documento %s repetido no manifesto", k)
		}
		vistas2[k] = true
		g.open(tag)
		g.el(chTag, k)
		if comBarra && opc.SegCodBarra != "" {
			if len(opc.SegCodBarra) != 36 || digitos(opc.SegCodBarra) != opc.SegCodBarra {
				g.fail("SegCodBarra de %s precisa de 36 dígitos", k)
			}
			g.el("SegCodBarra", opc.SegCodBarra)
		}
		if opc.IndReentrega {
			g.el("indReentrega", "1")
		}
		g.end(tag)
	}
	for _, d := range e.Descargas {
		c, x := g.mun("município de descarga", d.CodigoMun, d.Nome)
		g.open("infMunDescarga")
		g.el("cMunDescarga", c)
		g.el("xMunDescarga", x)
		if len(d.CTes)+len(d.NFes)+len(d.MDFes) < 1 {
			g.fail("descarga %s: sem documento vinculado", d.CodigoMun)
		}
		for _, k := range d.CTes {
			doc("infCTe", "chCTe", k, d.Opc[k], true)
		}
		for _, k := range d.NFes {
			doc("infNFe", "chNFe", k, d.Opc[k], true)
		}
		for _, k := range d.MDFes {
			doc("infMDFeTransp", "chMDFe", k, d.Opc[k], false)
		}
		g.end("infMunDescarga")
	}
	g.end("infDoc")
	g.seguros(e.Seguros)

	// prodPred: opcional no XSD; obrigatório com tpEmit 1/3 ou 2 com tpTransp
	// (rej 725/301, conforme pedido do produto).
	if pp := e.ProdPred; pp == nil {
		if exigeProd {
			g.fail("produto predominante (prodPred) é obrigatório para este tpEmit/tpTransp (rej 725/301)")
		}
	} else {
		g.open("prodPred")
		if !inSet(pp.TpCarga, "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12") {
			g.fail("tpCarga %q inválido (01..12)", pp.TpCarga)
		}
		g.el("tpCarga", pp.TpCarga)
		g.el("xProd", g.txt("xProd", pp.XProd, 1, 120))
		if pp.CEAN != "" {
			p := strings.ToUpper(strings.TrimSpace(pp.CEAN))
			if p != "SEM GTIN" && !reGTIN.MatchString(p) {
				g.fail("cEAN %q inválido", pp.CEAN)
			}
			g.el("cEAN", p)
		}
		if ncm := digitos(pp.NCM); ncm != "" {
			if len(ncm) != 2 && len(ncm) != 8 {
				g.fail("NCM do produto predominante: 2 ou 8 dígitos")
			}
			g.el("NCM", ncm)
		}
		// Um único documento vinculado = carga lotação: infLotacao com os
		// CEP de carga e descarga.
		if lotacao {
			cc, cd := builder.FormatarCEP(pp.CEPCarrega), builder.FormatarCEP(pp.CEPDescarrega)
			if len(cc) != 8 || len(cd) != 8 {
				g.fail("MDF-e com 1 documento é carga lotação: informe CEPCarrega e CEPDescarrega (8 dígitos)")
			}
			g.raw("<infLotacao><infLocalCarrega><CEP>" + cc + "</CEP></infLocalCarrega><infLocalDescarrega><CEP>" + cd + "</CEP></infLocalDescarrega></infLotacao>")
		}
		g.end("prodPred")
	}

	// qNFe = total real de infNFe (totalizador divergente é rejeitado,
	// 676/677 segundo o pedido, não confirmado no MOC); qCTe/qMDFe ficam de
	// fora porque esta onda só vincula NF-e.
	if e.VCarga <= 0 || e.QCarga <= 0 {
		g.fail("vCarga e qCarga precisam ser maiores que zero")
	}
	if e.CUnid != "01" && e.CUnid != "02" {
		g.fail("cUnid %q inválido (01 KG, 02 TON)", e.CUnid)
	}
	g.open("tot")
	if nCTe > 0 {
		g.el("qCTe", strconv.Itoa(nCTe))
	}
	if nNFe > 0 {
		g.el("qNFe", strconv.Itoa(nNFe))
	}
	if nMDFe > 0 {
		g.el("qMDFe", strconv.Itoa(nMDFe))
	}
	g.el("vCarga", strconv.FormatFloat(e.VCarga, 'f', 2, 64)) // TDec_1302
	g.el("cUnid", e.CUnid)
	g.el("qCarga", strconv.FormatFloat(e.QCarga, 'f', 4, 64)) // TDec_1104
	g.end("tot")

	for _, l := range e.Lacres {
		g.open("lacres")
		g.el("nLacre", g.txt("lacre", l, 1, 60))
		g.end("lacres")
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
	if e.InfAdFisco != "" || e.InfCpl != "" {
		g.open("infAdic")
		g.opt("infAdFisco", "infAdFisco", e.InfAdFisco, 1, 2000)
		g.opt("infCpl", "infCpl", e.InfCpl, 1, 5000)
		g.end("infAdic")
	}
	if rt := e.RespTec; rt != nil {
		g.open("infRespTec")
		t, v := g.docID("respTec", rt.CNPJ)
		if t != "CNPJ" {
			g.fail("infRespTec exige CNPJ")
		}
		g.el("CNPJ", v)
		g.el("xContato", g.txt("xContato", rt.Contato, 2, 60))
		if m := g.txt("email do resp. técnico", rt.Email, 6, 60); reEmail.MatchString(m) {
			g.el("email", m)
		} else {
			g.fail("email do responsável técnico inválido")
		}
		f := digitos(rt.Fone)
		if !reFone.MatchString(f) {
			g.fail("fone do responsável técnico: 7 a 12 dígitos")
		}
		g.el("fone", f)
		g.end("infRespTec")
	}
	g.end("infMDFe")
	// Sempre presente (NT 2019.001 / MOC 3.00a); a rejeição por ausência
	// (cogitadas 462/464) NÃO foi confirmada.
	g.open("infMDFeSupl")
	if e.TpEmis == "2" {
		g.el("qrCodMDFe", URLQRCodeContingencia(chave, e.TpAmb, e.QRSign))
	} else {
		g.el("qrCodMDFe", URLQRCode(chave, e.TpAmb))
	}
	g.end("infMDFeSupl")
	g.raw("</MDFe>")

	if g.err != nil {
		return nil, "", g.err
	}
	return []byte(g.b.String()), chave, nil
}

// comCNF troca o cNF sorteado pelo informado e recalcula o cDV (mesma regra de
// builder/chave.go: módulo 11, cada caractere vale ASCII-48).
func comCNF(ch builder.ChaveAcesso, cnf string, g *gen) builder.ChaveAcesso {
	if len(cnf) != 8 || digitos(cnf) != cnf || cnf == "00000000" {
		g.fail("%s: precisa de 8 dígitos, diferente de zeros (veio %q)", "CMDF", cnf)
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

// Chave devolve a chave que Build(e) vai gerar. Exige CMDF preenchido (senão o
// cNF é aleatório e a chave não seria a mesma). Não exige QRSign: serve justamente
// para obter a chave antes de assiná-la na contingência.
func Chave(e Entrada) (string, error) {
	if e.CMDF == "" {
		return "", fmt.Errorf("mdfe: Chave exige CMDF (sem ele o cNF é aleatório)")
	}
	if e.TpEmis == "2" && e.QRSign == "" {
		e.QRSign = "pendente"
	}
	_, chave, err := Build(e)
	return chave, err
}
