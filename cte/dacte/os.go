package dacte

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

// DACTE OS (CT-e Outros Serviços, modelo 67, leiaute 4.00).
//
// Base: o MOC CT-e 4.00 (http://moc.sped.fazenda.pr.gov.br/CTe/) tem o
// leiaute do DACTE OS, mas ele NÃO foi consultado aqui (sem rede): seções e
// posição dos quadros do DACTE OS são "não confirmado". O desenho reaproveita
// o cabeçalho do DACTE 57 deste pacote (MOC DACTE §2.5 chave em grupos de 4,
// §2.13 cabeçalho em toda folha, §2.18 código de barras CODE-128C, §2.19 QR
// Code, §2.21 homologação, §2.22.1 cancelado) e preenche o corpo com os
// campos do TCTeOS (cteTiposBasico_v4.00.xsd).

type xTomaOS struct {
	xParty
	Email string `xml:"email"`
}

type xInfOS struct {
	ID  string `xml:"Id,attr"`
	Ide struct {
		CFOP      string   `xml:"CFOP"`
		NatOp     string   `xml:"natOp"`
		Mod       string   `xml:"mod"`
		Serie     string   `xml:"serie"`
		NCT       string   `xml:"nCT"`
		DhEmi     string   `xml:"dhEmi"`
		TpEmis    string   `xml:"tpEmis"`
		TpAmb     string   `xml:"tpAmb"`
		TpCTe     string   `xml:"tpCTe"`
		Modal     string   `xml:"modal"`
		TpServ    string   `xml:"tpServ"`
		XMunIni   string   `xml:"xMunIni"`
		UFIni     string   `xml:"UFIni"`
		XMunFim   string   `xml:"xMunFim"`
		UFFim     string   `xml:"UFFim"`
		UFPer     []string `xml:"infPercurso>UFPer"`
		DhCont    string   `xml:"dhCont"`
		XJust     string   `xml:"xJust"`
		IndIEToma string   `xml:"indIEToma"`
	} `xml:"ide"`
	Compl struct {
		XCaracAd  string  `xml:"xCaracAd"`
		XCaracSer string  `xml:"xCaracSer"`
		XEmi      string  `xml:"xEmi"`
		XObs      string  `xml:"xObs"`
		ObsCont   []xObsC `xml:"ObsCont"`
		ObsFisco  []xObsC `xml:"ObsFisco"`
	} `xml:"compl"`
	Emit struct {
		CNPJ  string `xml:"CNPJ"`
		IE    string `xml:"IE"`
		XNome string `xml:"xNome"`
		XFant string `xml:"xFant"`
		End   xEnd   `xml:"enderEmit"`
	} `xml:"emit"`
	Toma   xTomaOS `xml:"toma"`
	VPrest struct {
		VTPrest string `xml:"vTPrest"`
		VRec    string `xml:"vRec"`
		Comp    []struct {
			XNome string `xml:"xNome"`
			VComp string `xml:"vComp"`
		} `xml:"Comp"`
	} `xml:"vPrest"`
	Imp struct {
		ICMS struct {
			G []xICMS `xml:",any"`
		} `xml:"ICMS"`
		VTotTrib string `xml:"vTotTrib"`
		TribFed  struct {
			PIS    string `xml:"vPIS"`
			COFINS string `xml:"vCOFINS"`
			IR     string `xml:"vIR"`
			INSS   string `xml:"vINSS"`
			CSLL   string `xml:"vCSLL"`
		} `xml:"infTribFed"`
		InfAdFisco string `xml:"infAdFisco"`
		IBSCBS     struct {
			CST        string `xml:"CST"`
			CClassTrib string `xml:"cClassTrib"`
			VBC        string `xml:"gIBSCBS>vBC"`
			VIBS       string `xml:"gIBSCBS>vIBS"`
			VCBS       string `xml:"gIBSCBS>gCBS>vCBS"`
		} `xml:"IBSCBS"`
		VTotDFe string `xml:"vTotDFe"`
	} `xml:"imp"`
	Norm struct {
		Serv struct {
			XDescServ string `xml:"xDescServ"`
			QCarga    string `xml:"infQ>qCarga"`
		} `xml:"infServico"`
		DocRef []struct {
			NDoc     string `xml:"nDoc"`
			Serie    string `xml:"serie"`
			Subserie string `xml:"subserie"`
			DEmi     string `xml:"dEmi"`
			VDoc     string `xml:"vDoc"`
			ChBPe    string `xml:"chBPe"`
		} `xml:"infDocRef"`
		Seg []struct {
			RespSeg string `xml:"respSeg"`
			XSeg    string `xml:"xSeg"`
			NApol   string `xml:"nApol"`
		} `xml:"seg"`
		Rodo struct {
			TAF, NroRegEstadual string
			Veic                struct {
				Placa   string `xml:"placa"`
				RENAVAM string
				UF      string
				Prop    struct {
					CNPJ, CPF, TAF, NroRegEstadual, IE, UF string
					TpProp                                 string `xml:"tpProp"`
					XNome                                  string `xml:"xNome"`
				} `xml:"prop"`
			} `xml:"veic"`
			TpFretamento string `xml:"infFretamento>tpFretamento"`
			DhViagem     string `xml:"infFretamento>dhViagem"`
		} `xml:"infModal>rodoOS"`
		Sub     string `xml:"infCteSub>chCte"`
		RefCanc string `xml:"refCTeCanc"`
		NFat    string `xml:"cobr>fat>nFat"`
		VLiq    string `xml:"cobr>fat>vLiq"`
		Dup     []struct {
			NDup  string `xml:"nDup"`
			DVenc string `xml:"dVenc"`
			VDup  string `xml:"vDup"`
		} `xml:"cobr>dup"`
		GTVe []struct {
			ChCTe string `xml:"chCTe"`
			Comp  []struct {
				TpComp string `xml:"tpComp"`
				VComp  string `xml:"vComp"`
				XComp  string `xml:"xComp"`
			} `xml:"Comp"`
		} `xml:"infGTVe"`
	} `xml:"infCTeNorm"`
	Comp []string `xml:"infCteComp>chCTe"`
}

type xRaizOS struct {
	XMLName xml.Name
	CTe     struct {
		Info xInfOS `xml:"infCte"`
		Supl xSupl  `xml:"infCTeSupl"`
	} `xml:"CTeOS"`
	Prot struct {
		NProt    string `xml:"nProt"`
		DhRecbto string `xml:"dhRecbto"`
		CStat    string `xml:"cStat"`
	} `xml:"protCTe>infProt"`
	Info xInfOS `xml:"infCte"`
	Supl xSupl  `xml:"infCTeSupl"`
}

func parseOS(b []byte) (inf *xInfOS, prot protocolo, qr string, err error) {
	var r xRaizOS
	if err = xml.Unmarshal(b, &r); err != nil {
		return nil, prot, "", fmt.Errorf("dacte: XML inválido: %w", err)
	}
	switch r.XMLName.Local {
	case "cteOSProc":
		inf, qr = &r.CTe.Info, strings.TrimSpace(r.CTe.Supl.QR)
		if c := strings.TrimSpace(r.Prot.CStat); c != "" && c != "100" {
			return nil, prot, "", fmt.Errorf("dacte: protocolo com cStat %s: CT-e OS não autorizado", c)
		}
		prot = protocolo{strings.TrimSpace(r.Prot.NProt), r.Prot.DhRecbto}
	case "CTeOS":
		inf, qr = &r.Info, strings.TrimSpace(r.Supl.QR)
	default:
		return nil, prot, "", fmt.Errorf("dacte: raiz <%s> não é cteOSProc nem CTeOS", r.XMLName.Local)
	}
	if !reChave.MatchString(strings.TrimPrefix(inf.ID, "CTe")) {
		return nil, prot, "", fmt.Errorf("dacte: chave de acesso ausente ou inválida em infCte/@Id")
	}
	if m := strings.TrimSpace(inf.Ide.Mod); m != "67" {
		return nil, prot, "", fmt.Errorf("dacte: modelo %q não suportado (GerarOS só 67)", m)
	}
	return inf, prot, qr, nil
}

var (
	tpServOS = map[string]string{"6": "6 - Transporte de pessoas", "7": "7 - Transporte de valores", "8": "8 - Excesso de bagagem"}
	modaisOS = map[string]string{"01": "01 - Rodoviário", "02": "02 - Aéreo", "03": "03 - Aquaviário", "04": "04 - Ferroviário"}
	fretOS   = map[string]string{"1": "1 - Eventual", "2": "2 - Contínuo"}
	propOS   = map[string]string{"0": "0 - TAC agregado", "1": "1 - TAC independente", "2": "2 - Outros"}
	respSeg  = map[string]string{"4": "4 - Emitente do CT-e", "5": "5 - Tomador do serviço"}
	compGTVe = map[string]string{"1": "Custódia", "2": "Embarque", "3": "Tempo de espera", "4": "Malote", "5": "Ad valorem", "6": "Outros"}
)

// GerarOS produz o PDF do DACTE OS a partir do cteOSProc (autorizado) ou do
// CTeOS assinado (sem protocolo = prévia sem valor fiscal).
func GerarOS(cteOSXML []byte, op Opcoes) (pdfOut []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			pdfOut, err = nil, fmt.Errorf("dacte: %v", r)
		}
	}()
	inf, prot, qr, err := parseOS(cteOSXML)
	if err != nil {
		return nil, err
	}
	chave := strings.TrimPrefix(inf.ID, "CTe")

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(compressao)
	pdf.SetMargins(mg, mg, mg)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AliasNbPages("{nb}")
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	fonte := func(st string, sz float64) { pdf.SetFont("Helvetica", st, sz) }
	fit := func(x, y, w float64, s string, st string, sz float64) {
		s = tr(s)
		for ; sz > 5; sz -= 0.5 {
			fonte(st, sz)
			if pdf.GetStringWidth(s) <= w {
				break
			}
		}
		fonte(st, sz)
		pdf.Text(x, y, s)
	}
	cel := func(x, y, w, h float64, rot, val string) {
		pdf.Rect(x, y, w, h, "D")
		pdf.SetTextColor(90, 90, 90)
		fit(x+0.8, y+2.4, w-1.6, rot, "", 5.5)
		pdf.SetTextColor(0, 0, 0)
		fit(x+0.8, y+h-1.3, w-1.6, val, "B", 8)
	}

	var okBar, okQR bool
	if bc, e := code128.Encode(chave); e == nil {
		if sc, e := barcode.Scale(bc, 900, 60); e == nil {
			var buf bytes.Buffer
			if png.Encode(&buf, sc) == nil {
				pdf.RegisterImageOptionsReader("bar", fpdf.ImageOptions{ImageType: "PNG"}, &buf)
				okBar = true
			}
		}
	}
	if qr != "" {
		if q, e := qrcode.New(qr, qrcode.Medium); e == nil {
			if b, e := q.PNG(256); e == nil {
				pdf.RegisterImageOptionsReader("qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(b))
				okQR = true
			}
		}
	}

	i := inf.Ide
	homolog := i.TpAmb == "2"
	contingencia := i.TpEmis == "2"
	semProt := prot.nProt == "" && !contingencia

	pdf.SetHeaderFuncMode(func() {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextColor(0, 0, 0)
		e := inf.Emit
		pdf.Rect(mg, mg, 108, 28, "D")
		pdf.Rect(mg+108, mg, lw-108, 28, "D")
		fit(mg+1.5, mg+5, 105, ou(e.XNome, e.XFant), "B", 10)
		ed := e.End
		for k, l := range []string{
			"CNPJ: " + cnpjCpf(e.CNPJ, "") + "   IE: " + e.IE,
			strings.TrimSpace(ed.XLgr + ", " + ed.Nro + " " + ed.XCpl),
			strings.TrimSpace(ed.XBairro + " - " + ed.XMun + " - " + ed.UF),
			strings.TrimSpace("CEP: " + ed.CEP + "   Fone: " + ed.Fone),
		} {
			fit(mg+1.5, mg+10+float64(k)*4.4, 105, l, "", 7.5)
		}
		fit(mg+110, mg+7, 82, "DACTE OS", "B", 15)
		fit(mg+110, mg+11.5, 82, "Documento Auxiliar do CT-e para Outros Serviços", "", 6)
		fit(mg+110, mg+16, 82, fmt.Sprintf("Modelo %s   Série %s   Nº %s", i.Mod, i.Serie, i.NCT), "B", 8)
		fit(mg+110, mg+20.5, 82, fmt.Sprintf("Folha %d/{nb}     Modal: %s", pdf.PageNo(), modaisOS[i.Modal]), "", 7.5)
		fit(mg+110, mg+25, 82, "Emissão: "+dataHora(i.DhEmi), "", 7.5)

		y := mg + 28
		pdf.Rect(mg, y, lw, 34, "D")
		if okBar {
			pdf.Image("bar", mg+3, y+2, 100, 14, false, "", 0, "")
		}
		pdf.SetTextColor(90, 90, 90)
		fit(mg+1.5, y+19.5, 100, "Chave de acesso", "", 6)
		pdf.SetTextColor(0, 0, 0)
		fit(mg+1.5, y+24, 104, grupos4(chave), "B", 9)
		if okQR {
			pdf.Image("qr", mg+lw-30, y+3, 28, 28, false, "", 0, "")
		}
		pdf.SetTextColor(90, 90, 90)
		fit(mg+108, y+4, 54, "Protocolo de autorização de uso", "", 6)
		pdf.SetTextColor(0, 0, 0)
		switch {
		case prot.nProt != "":
			fit(mg+108, y+9, 54, prot.nProt+" "+dataHora(prot.dh), "B", 8)
		case contingencia:
			fit(mg+108, y+9, 54, "EMISSÃO EM CONTINGÊNCIA", "B", 8.5)
		}
		pdf.SetTextColor(90, 90, 90)
		fit(mg+108, y+16, 54, "Consulta de autenticidade (chave de acesso):", "", 6)
		fit(mg+108, y+19.5, 54, "www.cte.fazenda.gov.br/portal", "", 6.5)
		pdf.SetTextColor(0, 0, 0)
		cel(mg, y+34, lw, 9, "CFOP - Natureza da operação", i.CFOP+" - "+i.NatOp)

		pdf.SetY(y + 44)
		pdf.SetTextColor(180, 0, 0)
		fonte("B", 9)
		for _, f := range []struct {
			on bool
			s  string
		}{{semProt, "SEM VALOR FISCAL - PRÉVIA SEM PROTOCOLO DE AUTORIZAÇÃO"},
			{homolog, "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL"}} {
			if f.on {
				pdf.SetX(mg)
				pdf.CellFormat(lw, 6, tr(f.s), "1", 1, "C", false, 0, "")
			}
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(1)
		if op.Cancelado {
			pdf.TransformBegin()
			pdf.TransformRotate(45, 105, 160)
			pdf.SetTextColor(235, 150, 150)
			fonte("B", 80)
			pdf.Text(25, 170, "CANCELADO")
			pdf.TransformEnd()
			pdf.SetTextColor(0, 0, 0)
		}
	}, false)
	pdf.AddPage()

	need := func(h float64) {
		if pdf.GetY()+h > fundo {
			pdf.AddPage()
		}
	}
	secao := func(t string) {
		need(14)
		pdf.SetFillColor(225, 225, 225)
		fonte("B", 7.5)
		pdf.SetX(mg)
		pdf.CellFormat(lw, 4.5, tr(t), "1", 1, "L", true, 0, "")
	}
	linha := func(h float64, frac []float64, rv ...string) {
		need(h)
		x, y := mg, pdf.GetY()
		for k, f := range frac {
			cel(x, y, lw*f, h, rv[2*k], rv[2*k+1])
			x += lw * f
		}
		pdf.SetY(y + h)
	}
	texto := func(s string) {
		for _, ln := range pdf.SplitLines([]byte(tr(s)), lw-2) {
			need(5)
			fonte("", 7.5)
			pdf.SetX(mg)
			pdf.CellFormat(lw, 3.8, string(ln), "LR", 1, "L", false, 0, "")
		}
	}
	fecha := func() { pdf.Line(mg, pdf.GetY(), mg+lw, pdf.GetY()); pdf.Ln(1.5) }

	n := inf.Norm
	qrot := "Quantidade (volumes/malotes)"
	if i.TpServ == "6" {
		qrot = "Quantidade (passageiros)"
	}
	linha(9, []float64{.28, .22, .5},
		"Tipo do serviço", tpServOS[i.TpServ],
		"Tipo do CT-e OS", tpCTe[i.TpCTe],
		"Descrição do serviço", n.Serv.XDescServ)
	linha(9, []float64{.5, .5},
		"Início da prestação", strings.TrimSpace(i.XMunIni+" - "+i.UFIni),
		"Término da prestação", strings.TrimSpace(i.XMunFim+" - "+i.UFFim))
	linha(9, []float64{.3, .7}, qrot, num(n.Serv.QCarga), "UF do percurso", strings.Join(i.UFPer, ", "))
	for _, k := range []struct{ rot, ch string }{
		{"CT-e OS substituído (infCteSub)", n.Sub},
		{"CT-e OS cancelado referenciado (refCTeCanc)", n.RefCanc},
		{"CT-e OS complementado (infCteComp)", strings.Join(inf.Comp, "  ")},
	} {
		if strings.TrimSpace(k.ch) != "" {
			linha(8, []float64{1}, k.rot, grupos4(strings.TrimSpace(k.ch)))
		}
	}
	if contingencia && i.XJust != "" {
		linha(8, []float64{1}, "Contingência - justificativa ("+dataHora(i.DhCont)+")", i.XJust)
	}

	// Tomador do serviço (grupo toma; opcional no excesso de bagagem)
	t := inf.Toma
	if t.XNome != "" || t.CNPJ != "" || t.CPF != "" {
		secao("TOMADOR DO SERVIÇO")
		e := t.end()
		y := pdf.GetY()
		need(23)
		y = pdf.GetY()
		pdf.Rect(mg, y, lw, 18, "D")
		for k, l := range []string{
			ou(t.XNome, t.XFant),
			strings.TrimSpace(e.XLgr + ", " + e.Nro + " " + e.XCpl + " " + e.XBairro),
			strings.TrimSpace(e.XMun + " - " + e.UF + "   CEP: " + e.CEP + "   " + e.XPais),
			"CNPJ/CPF: " + cnpjCpf(t.CNPJ, t.CPF) + "   IE: " + t.IE + "   Fone: " + ou(t.Fone, e.Fone) + "   " + t.Email,
		} {
			st := ""
			if k == 0 {
				st = "B"
			}
			fit(mg+1, y+4+float64(k)*3.9, lw-2, l, st, 7.5)
		}
		pdf.SetY(y + 18)
	}

	secao("Componentes do valor da prestação do serviço")
	vp := inf.VPrest
	for k := 0; k < len(vp.Comp); k += 2 {
		need(8)
		y := pdf.GetY()
		for j := k; j < k+2 && j < len(vp.Comp); j++ {
			x := mg + float64(j-k)*lw/2
			cel(x, y, lw/2-30, 8, "Nome", vp.Comp[j].XNome)
			cel(x+lw/2-30, y, 30, 8, "Valor", num(vp.Comp[j].VComp))
		}
		pdf.SetY(y + 8)
	}
	linha(9, []float64{.5, .5}, "Valor total da prestação do serviço", moeda(vp.VTPrest), "Valor a receber", moeda(vp.VRec))

	secao("Informações relativas ao imposto")
	var g xICMS
	for _, c := range inf.Imp.ICMS.G {
		if c.CST != "" {
			g = c
			break
		}
	}
	cst := g.CST
	if g.IndSN != "" {
		cst += " - Simples Nacional"
	}
	linha(9, []float64{.28, .18, .14, .2, .2},
		"Classificação tributária do serviço (CST)", cst,
		"Base de cálculo", num(ou(g.VBC, g.VBCOUF)), "Alíquota ICMS (%)", num(ou(g.PICMS, g.PICMSOF)),
		"Valor do ICMS", num(ou(g.VICMS, g.VICMSOF)), "% Red. BC cálc.", num(ou(g.PRedBC, g.PRedOUF)))
	if t := inf.Imp; t.VTotTrib != "" || t.VTotDFe != "" {
		linha(8, []float64{.5, .5}, "Valor aproximado dos tributos", moeda(t.VTotTrib), "Valor total do DF-e (IBS/CBS)", moeda(t.VTotDFe))
	}
	if f := inf.Imp.TribFed; f != (struct {
		PIS    string `xml:"vPIS"`
		COFINS string `xml:"vCOFINS"`
		IR     string `xml:"vIR"`
		INSS   string `xml:"vINSS"`
		CSLL   string `xml:"vCSLL"`
	}{}) {
		linha(8, []float64{.2, .2, .2, .2, .2}, "PIS retido", moeda(f.PIS), "COFINS retido", moeda(f.COFINS), "IR retido", moeda(f.IR), "INSS retido", moeda(f.INSS), "CSLL retido", moeda(f.CSLL))
	}
	if b := inf.Imp.IBSCBS; b.CST != "" {
		linha(9, []float64{.14, .16, .2, .25, .25},
			"IBS/CBS - CST", b.CST, "cClassTrib", b.CClassTrib, "Base IBS/CBS", num(b.VBC),
			"IBS total (R$)", num(b.VIBS), "CBS (R$)", num(b.VCBS))
	}

	if len(n.DocRef) > 0 {
		secao("Documentos referenciados")
		for _, d := range n.DocRef {
			if d.ChBPe != "" {
				texto("BP-e: " + grupos4(strings.TrimSpace(d.ChBPe)))
			} else {
				texto(fmt.Sprintf("Documento nº %s  série %s  subsérie %s  emissão %s  valor %s", d.NDoc, d.Serie, d.Subserie, data(d.DEmi), moeda(d.VDoc)))
			}
		}
		fecha()
	}
	if len(n.Seg) > 0 {
		secao("Seguro da carga")
		for _, s := range n.Seg {
			texto(fmt.Sprintf("Responsável: %s   Seguradora: %s   Apólice: %s", respSeg[s.RespSeg], s.XSeg, s.NApol))
		}
		fecha()
	}
	if len(n.GTVe) > 0 {
		secao("GTV-e relacionadas")
		for _, v := range n.GTVe {
			texto("GTV-e: " + grupos4(strings.TrimSpace(v.ChCTe)))
			for _, c := range v.Comp {
				texto(fmt.Sprintf("   %s %s: %s", compGTVe[c.TpComp], c.XComp, moeda(c.VComp)))
			}
		}
		fecha()
	}

	// Modal rodoviário OS (cteModalRodoviarioOS_v4.00.xsd)
	if r := n.Rodo; r.TAF != "" || r.NroRegEstadual != "" {
		secao("Informações do modal rodoviário (OS)")
		reg, num2 := "TAF - Termo de Autorização de Fretamento", r.TAF
		if r.NroRegEstadual != "" {
			reg, num2 = "Nº do registro estadual", r.NroRegEstadual
		}
		linha(9, []float64{.5, .25, .25}, reg, num2, "Tipo de fretamento", fretOS[r.TpFretamento], "Data/hora da viagem", dataHora(r.DhViagem))
		if v := r.Veic; v.Placa != "" {
			linha(9, []float64{.25, .35, .4}, "Placa do veículo", v.Placa, "RENAVAM", v.RENAVAM, "UF de licenciamento", v.UF)
			if p := v.Prop; p.XNome != "" {
				texto(fmt.Sprintf("Proprietário/possuidor: %s - %s   IE: %s %s   %s: %s   Tipo: %s", p.XNome, cnpjCpf(p.CNPJ, p.CPF), p.IE, p.UF,
					map[bool]string{true: "Registro estadual", false: "TAF"}[p.NroRegEstadual != ""], ou(p.NroRegEstadual, p.TAF), propOS[p.TpProp]))
				fecha()
			}
		}
	}
	if n.NFat != "" || len(n.Dup) > 0 {
		secao("Cobrança")
		if n.NFat != "" {
			texto("Fatura " + n.NFat + "   valor líquido " + moeda(n.VLiq))
		}
		for _, d := range n.Dup {
			texto(fmt.Sprintf("Duplicata %s   vencimento %s   valor %s", d.NDup, data(d.DVenc), moeda(d.VDup)))
		}
		fecha()
	}

	c := inf.Compl
	if c.XObs != "" || c.XCaracAd != "" || c.XCaracSer != "" || c.XEmi != "" || inf.Imp.InfAdFisco != "" || len(c.ObsCont) > 0 {
		secao("Observações")
		for _, s := range []string{
			c.XObs,
			cond(c.XCaracAd, "Característica adicional do transporte: "),
			cond(c.XCaracSer, "Característica adicional do serviço: "),
			cond(c.XEmi, "Emitente: "),
			cond(inf.Imp.InfAdFisco, "Fisco: "),
		} {
			if s != "" {
				texto(s)
			}
		}
		for _, o := range c.ObsCont {
			texto("Uso do emissor - " + o.Campo + ": " + o.Texto)
		}
		fecha()
	}
	if len(c.ObsFisco) > 0 {
		secao("Reservado ao Fisco")
		for _, o := range c.ObsFisco {
			texto(o.Campo + ": " + o.Texto)
		}
		fecha()
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("dacte: %w", err)
	}
	return out.Bytes(), nil
}
