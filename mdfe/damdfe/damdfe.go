// Package damdfe gera o DAMDFE (Documento Auxiliar do MDF-e, modelo 58,
// leiaute 3.00) em PDF A4 retrato a partir do XML mdfeProc (autorizado) ou do
// MDFe assinado (sem protocolo = prévia sem valor fiscal).
//
// Uso:
//
//	pdf, err := damdfe.Gerar(mdfeXML, damdfe.Opcoes{})
//
// Base normativa: MOC MDF-e online, página DAMDFE (Anexo II 3.00b),
// http://moc.sped.fazenda.pr.gov.br/MDFe/DAMDFE.html, citada como "MOC DAMDFE §n".
// As imagens de modelo (§6) não são texto: a posição exata dos quadros segue o
// padrão de mercado e está marcada "não confirmado no MOC".
package damdfe

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/png"
	"regexp"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

// Opcoes: o XML autorizado não muda com cancelamento/encerramento (eventos à
// parte), então o chamador informa o estado atual para o papel refletir.
type Opcoes struct{ Cancelado, Encerrado bool }

// compressao só é desligada nos testes, para buscar texto no PDF.
var compressao = true

// ── parse (encoding/xml casa pelo nome local; o namespace é o do schema mdfe) ──

type xmlRaiz struct {
	XMLName xml.Name
	MDFe    xmlMDFe `xml:"MDFe"`
	Prot    struct {
		NProt    string `xml:"nProt"`
		DhRecbto string `xml:"dhRecbto"`
	} `xml:"protMDFe>infProt"`
	Info xmlInf  `xml:"infMDFe"`     // quando a raiz é o próprio MDFe
	Supl xmlSupl `xml:"infMDFeSupl"` // idem
}

type xmlMDFe struct {
	Info xmlInf  `xml:"infMDFe"`
	Supl xmlSupl `xml:"infMDFeSupl"`
}

type xmlSupl struct {
	QR string `xml:"qrCodMDFe"`
}

type xmlProp struct {
	XNome string `xml:"xNome"`
	CPF   string `xml:"CPF"`
	CNPJ  string `xml:"CNPJ"`
	RNTRC string `xml:"RNTRC"`
}

type xmlInf struct {
	ID  string `xml:"Id,attr"`
	Ide struct {
		TpAmb    string   `xml:"tpAmb"`
		TpEmit   string   `xml:"tpEmit"`
		TpTransp string   `xml:"tpTransp"`
		TpEmis   string   `xml:"tpEmis"`
		Mod      string   `xml:"mod"`
		Serie    string   `xml:"serie"`
		NMDF     string   `xml:"nMDF"`
		DhEmi    string   `xml:"dhEmi"`
		UFIni    string   `xml:"UFIni"`
		UFFim    string   `xml:"UFFim"`
		MunCarr  []string `xml:"infMunCarrega>xMunCarrega"`
		Percurso []string `xml:"infPercurso>UFPer"`
	} `xml:"ide"`
	Emit struct {
		CNPJ  string `xml:"CNPJ"`
		CPF   string `xml:"CPF"`
		IE    string `xml:"IE"`
		XNome string `xml:"xNome"`
		End   struct {
			XLgr    string `xml:"xLgr"`
			Nro     string `xml:"nro"`
			XCpl    string `xml:"xCpl"`
			XBairro string `xml:"xBairro"`
			XMun    string `xml:"xMun"`
			CEP     string `xml:"CEP"`
			UF      string `xml:"UF"`
		} `xml:"enderEmit"`
	} `xml:"emit"`
	Rodo struct {
		RNTRC string `xml:"infANTT>RNTRC"`
		CIOT  []struct {
			CIOT string `xml:"CIOT"`
			CPF  string `xml:"CPF"`
			CNPJ string `xml:"CNPJ"`
		} `xml:"infANTT>infCIOT"`
		Vale []struct {
			CNPJForn string `xml:"CNPJForn"`
			CNPJPg   string `xml:"CNPJPg"`
			CPFPg    string `xml:"CPFPg"`
			NCompra  string `xml:"nCompra"`
			VVale    string `xml:"vValePed"`
		} `xml:"infANTT>valePed>disp"`
		Contratantes []struct {
			XNome    string `xml:"xNome"`
			CPF      string `xml:"CPF"`
			CNPJ     string `xml:"CNPJ"`
			IDEstr   string `xml:"idEstrangeiro"`
			Contrato string `xml:"infContrato>NroContrato"`
			VGlobal  string `xml:"infContrato>vContratoGlobal"`
		} `xml:"infANTT>infContratante"`
		Pagamentos []struct {
			XNome     string `xml:"xNome"`
			CPF       string `xml:"CPF"`
			CNPJ      string `xml:"CNPJ"`
			VContrato string `xml:"vContrato"`
			IndPag    string `xml:"indPag"`
		} `xml:"infANTT>infPag"`
		Tracao struct {
			Placa string  `xml:"placa"`
			UF    string  `xml:"UF"`
			Prop  xmlProp `xml:"prop"`
			Cond  []struct {
				XNome string `xml:"xNome"`
				CPF   string `xml:"CPF"`
			} `xml:"condutor"`
		} `xml:"veicTracao"`
		Reboques []struct {
			Placa string  `xml:"placa"`
			UF    string  `xml:"UF"`
			Prop  xmlProp `xml:"prop"`
		} `xml:"veicReboque"`
	} `xml:"infModal>rodo"`
	Munic []struct {
		XMun string   `xml:"xMunDescarga"`
		CTe  []string `xml:"infCTe>chCTe"`
		NFe  []string `xml:"infNFe>chNFe"`
		MDFe []string `xml:"infMDFeTransp>chMDFe"`
	} `xml:"infDoc>infMunDescarga"`
	Seg []struct {
		RespSeg  string   `xml:"infResp>respSeg"`
		RespCNPJ string   `xml:"infResp>CNPJ"`
		RespCPF  string   `xml:"infResp>CPF"`
		XSeg     string   `xml:"infSeg>xSeg"`
		SegCNPJ  string   `xml:"infSeg>CNPJ"`
		NApol    string   `xml:"nApol"`
		NAver    []string `xml:"nAver"`
	} `xml:"seg"`
	Prod struct {
		TpCarga string `xml:"tpCarga"`
		XProd   string `xml:"xProd"`
		NCM     string `xml:"NCM"`
		CEAN    string `xml:"cEAN"`
	} `xml:"prodPred"`
	Tot struct {
		QCTe   string `xml:"qCTe"`
		QNFe   string `xml:"qNFe"`
		QMDFe  string `xml:"qMDFe"`
		VCarga string `xml:"vCarga"`
		CUnid  string `xml:"cUnid"`
		QCarga string `xml:"qCarga"`
	} `xml:"tot"`
	AdFisco string `xml:"infAdic>infAdFisco"`
	Cpl     string `xml:"infAdic>infCpl"`
}

type protocolo struct{ nProt, dh string }

var reChave = regexp.MustCompile(`^\d{44}$`)

func parse(b []byte) (inf *xmlInf, prot protocolo, qr string, err error) {
	var r xmlRaiz
	if err = xml.Unmarshal(b, &r); err != nil {
		return nil, prot, "", fmt.Errorf("damdfe: XML inválido: %w", err)
	}
	switch r.XMLName.Local {
	case "mdfeProc":
		inf, qr = &r.MDFe.Info, strings.TrimSpace(r.MDFe.Supl.QR)
		prot = protocolo{strings.TrimSpace(r.Prot.NProt), r.Prot.DhRecbto}
	case "MDFe":
		inf, qr = &r.Info, strings.TrimSpace(r.Supl.QR)
	default:
		return nil, prot, "", fmt.Errorf("damdfe: raiz <%s> não é mdfeProc nem MDFe", r.XMLName.Local)
	}
	if !reChave.MatchString(strings.TrimPrefix(inf.ID, "MDFe")) {
		return nil, prot, "", fmt.Errorf("damdfe: chave de acesso ausente ou inválida em infMDFe/@Id")
	}
	return inf, prot, qr, nil
}

// ── formatação ───────────────────────────────────────────────────────────────

func dataHora(s string) string {
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(l, strings.TrimSpace(s)); err == nil {
			return t.Format("02/01/2006 15:04:05")
		}
	}
	return s
}

func grupos4(s string) string {
	var p []string
	for i := 0; i < len(s); i += 4 {
		p = append(p, s[i:min(i+4, len(s))])
	}
	return strings.Join(p, " ")
}

func cnpjCpf(cnpj, cpf string) string {
	if len(cnpj) == 14 {
		return fmt.Sprintf("%s.%s.%s/%s-%s", cnpj[:2], cnpj[2:5], cnpj[5:8], cnpj[8:12], cnpj[12:])
	}
	if len(cpf) == 11 {
		return fmt.Sprintf("%s.%s.%s-%s", cpf[:3], cpf[3:6], cpf[6:9], cpf[9:])
	}
	return cnpj + cpf
}

func virgula(v string) string { return strings.Replace(strings.TrimSpace(v), ".", ",", 1) }
func moeda(v string) string   { return "R$ " + virgula(v) }

// ── geração ──────────────────────────────────────────────────────────────────

const (
	mg = 8.0   // margem
	lw = 194.0 // largura útil A4
)

// Gerar produz o PDF. Devolve erro (nunca panic) para XML malformado ou sem chave.
func Gerar(mdfeXML []byte, op Opcoes) ([]byte, error) {
	inf, prot, qr, err := parse(mdfeXML)
	if err != nil {
		return nil, err
	}
	chave := strings.TrimPrefix(inf.ID, "MDFe")

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(compressao)
	pdf.SetMargins(mg, mg, mg)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AliasNbPages("{nb}")
	// fonte core espera cp1252, não UTF-8 (mesma ideia do wrapper Doc do danfe)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	txt := func(w, h float64, s, borda, al string) { pdf.CellFormat(w, h, tr(s), borda, 0, al, false, 0, "") }
	fonte := func(st string, sz float64) { pdf.SetFont("Helvetica", st, sz) }

	// imagens registradas uma vez, reaproveitadas em cada folha
	var okBar, okQR bool
	if bc, e := code128.Encode(chave); e == nil { // 44 dígitos => subconjunto C
		if sc, e := barcode.Scale(bc, 800, 60); e == nil {
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

	homolog := inf.Ide.TpAmb == "2"
	contingencia := inf.Ide.TpEmis == "2" // MOC DAMDFE §4.1
	semProt := prot.nProt == "" && !contingencia

	// Cabeçalho repetido em toda folha: emitente, título, modelo/série/número/
	// folha, chave, código de barras, QR e protocolo. Posições: não confirmado no MOC.
	pdf.SetHeaderFuncMode(func() {
		pdf.SetDrawColor(0, 0, 0)
		e := inf.Emit
		pdf.Rect(mg, mg, 124, 24, "D")
		pdf.Rect(mg+124, mg, lw-124, 24, "D")
		pdf.SetXY(mg+1, mg+1)
		fonte("B", 9.5)
		txt(122, 4.5, e.XNome, "", "L")
		fonte("", 7.5)
		linhas := []string{
			"CNPJ/CPF: " + cnpjCpf(e.CNPJ, e.CPF) + "   IE: " + e.IE,
			strings.TrimSpace(e.End.XLgr + ", " + e.End.Nro + " " + e.End.XCpl + " - " + e.End.XBairro),
			strings.TrimSpace(e.End.XMun + " - " + e.End.UF + "   CEP: " + e.End.CEP),
		}
		for i, l := range linhas {
			pdf.SetXY(mg+1, mg+6+float64(i)*4.2)
			txt(122, 4, l, "", "L")
		}
		pdf.SetXY(mg+125, mg+1)
		fonte("B", 8)
		txt(68, 4, fmt.Sprintf("Modelo %s  Série %s  Nº %s", inf.Ide.Mod, inf.Ide.Serie, inf.Ide.NMDF), "", "L")
		fonte("", 7.5)
		for i, l := range []string{
			fmt.Sprintf("Folha %d/{nb}", pdf.PageNo()),
			"Emissão: " + dataHora(inf.Ide.DhEmi),
			"UF carregamento: " + inf.Ide.UFIni,
			"UF descarregamento: " + inf.Ide.UFFim,
		} {
			pdf.SetXY(mg+125, mg+6+float64(i)*4.4)
			txt(68, 4, l, "", "L")
		}
		pdf.SetXY(mg, mg+24)
		fonte("B", 9)
		txt(lw, 7, "DAMDFE - Documento Auxiliar de Manifesto Eletrônico de Documentos Fiscais", "1", "C")
		y := mg + 31
		pdf.Rect(mg, y, lw, 34, "D")
		if okBar {
			// MOC DAMDFE §3: área 3 cm x 9 cm, barras entre 1,5 e 2,5 cm de altura
			pdf.Image("bar", mg+3, y+2, 90, 20, false, "", 0, "")
		}
		fonte("B", 7)
		pdf.SetXY(mg+1, y+23)
		txt(120, 4, "Chave de acesso", "", "L")
		fonte("B", 9)
		pdf.SetXY(mg+1, y+27)
		txt(95, 4.5, grupos4(chave), "", "L")
		if okQR {
			// MOC DAMDFE §5: mínimo 25x25 mm, já com 3 mm de margem segura (quiet zone)
			pdf.Image("qr", mg+lw-31, y+3, 28, 28, false, "", 0, "")
		}
		// MOC DAMDFE §2 (só o protocolo pode ser acrescentado ao XML) e §4.1
		// (contingência: "EMISSÃO EM CONTINGÊNCIA" no local do protocolo).
		fonte("", 8)
		pdf.SetXY(mg+98, y+4)
		switch {
		case prot.nProt != "":
			pdf.MultiCell(58, 4, tr("Protocolo de autorização:\n"+prot.nProt+"\n"+dataHora(prot.dh)), "", "L", false)
		case contingencia:
			fonte("B", 9)
			pdf.MultiCell(58, 4.5, tr("EMISSÃO EM CONTINGÊNCIA"), "", "L", false)
		}
		pdf.SetY(mg + 66)
		pdf.SetTextColor(180, 0, 0)
		fonte("B", 9)
		for _, f := range []struct {
			on bool
			s  string
		}{{semProt, "SEM VALOR FISCAL - PRÉVIA"}, // não confirmado no MOC (prévia não existe no manual)
			{homolog, "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL"}} { // MOC DAMDFE §4.2
			if f.on {
				pdf.SetX(mg)
				pdf.CellFormat(lw, 6, tr(f.s), "1", 1, "C", false, 0, "")
			}
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(2)
		if op.Cancelado { // marca d'água diagonal
			pdf.TransformBegin()
			pdf.TransformRotate(45, 105, 160)
			pdf.SetTextColor(235, 150, 150)
			fonte("B", 80)
			pdf.Text(25, 170, "CANCELADO")
			pdf.TransformEnd()
			pdf.SetTextColor(0, 0, 0)
		}
		if op.Encerrado { // carimbo
			pdf.TransformBegin()
			pdf.TransformRotate(4, 132, 69)
			pdf.SetTextColor(0, 90, 160)
			pdf.SetDrawColor(0, 90, 160)
			fonte("B", 14)
			pdf.Rect(112, 64, 40, 10, "D") // área livre entre protocolo e QR; posição não confirmada no MOC
			pdf.Text(115, 71, "ENCERRADO")
			pdf.TransformEnd()
			pdf.SetTextColor(0, 0, 0)
			pdf.SetDrawColor(0, 0, 0)
		}
	}, false)
	pdf.AddPage()

	secao := func(t string) {
		pdf.SetFillColor(225, 225, 225)
		fonte("B", 8)
		pdf.SetX(mg)
		pdf.CellFormat(lw, 5, tr(t), "1", 1, "L", true, 0, "")
		fonte("", 8)
	}
	linha := func(s string) {
		pdf.SetX(mg)
		pdf.MultiCell(lw, 4.2, tr(s), "LR", "L", false)
	}
	fecha := func() { pdf.Line(mg, pdf.GetY(), mg+lw, pdf.GetY()); pdf.Ln(2) }

	// Blocos abaixo: ordem e rótulos não confirmados no MOC (o §6 do DAMDFE só
	// traz imagens); os campos vêm de mdfeTiposBasico/mdfeModalRodoviario v3.00.
	t := inf.Tot
	var nc, nn, nm int
	for _, m := range inf.Munic {
		nc, nn, nm = nc+len(m.CTe), nn+len(m.NFe), nm+len(m.MDFe)
	}
	qtd := func(tot string, n int) string {
		if tot != "" {
			return tot
		}
		return fmt.Sprint(n)
	}
	secao("Totais da carga")
	linha(fmt.Sprintf("Qtde CT-e: %s     Qtde NF-e: %s     Qtde MDF-e: %s",
		qtd(t.QCTe, nc), qtd(t.QNFe, nn), qtd(t.QMDFe, nm)))
	un := map[string]string{"01": "KG", "02": "TON"}[t.CUnid]
	linha(fmt.Sprintf("Peso bruto total: %s %s     Valor total da carga: %s", virgula(t.QCarga), un, moeda(t.VCarga)))
	fecha()

	secao("Viagem")
	linha(fmt.Sprintf("Tipo de emitente: %s     Tipo de transportador: %s",
		map[string]string{"1": "1 - Prestador de serviço de transporte", "2": "2 - Transportador de carga própria",
			"3": "3 - Prestador com CT-e globalizado"}[inf.Ide.TpEmit],
		map[string]string{"1": "1 - ETC", "2": "2 - TAC", "3": "3 - CTC"}[inf.Ide.TpTransp]))
	if len(inf.Ide.MunCarr) > 0 {
		linha("Municípios de carregamento: " + strings.Join(inf.Ide.MunCarr, ", "))
	}
	if len(inf.Ide.Percurso) > 0 {
		linha("UFs de percurso: " + strings.Join(inf.Ide.Percurso, ", "))
	}
	if p := inf.Prod; p.XProd != "" {
		linha(fmt.Sprintf("Produto predominante: %s   Tipo de carga: %s   NCM: %s   GTIN: %s", p.XProd, p.TpCarga, p.NCM, p.CEAN))
	}
	fecha()

	r := inf.Rodo
	secao("Modal rodoviário")
	if r.RNTRC != "" {
		linha("RNTRC do emitente: " + r.RNTRC)
	}
	prop := func(p xmlProp) string {
		if p.XNome == "" && p.CPF == "" && p.CNPJ == "" {
			return ""
		}
		return fmt.Sprintf("   Proprietário: %s - %s   RNTRC: %s", p.XNome, cnpjCpf(p.CNPJ, p.CPF), p.RNTRC)
	}
	linha(fmt.Sprintf("Veículo de tração - Placa: %s   UF: %s%s", r.Tracao.Placa, r.Tracao.UF, prop(r.Tracao.Prop)))
	for _, b := range r.Reboques {
		linha(fmt.Sprintf("Reboque - Placa: %s   UF: %s%s", b.Placa, b.UF, prop(b.Prop)))
	}
	for _, c := range r.Tracao.Cond {
		linha(fmt.Sprintf("Condutor: %s   CPF: %s", c.XNome, cnpjCpf("", c.CPF)))
	}
	for _, c := range r.CIOT {
		linha(fmt.Sprintf("CIOT: %s   Responsável: %s", c.CIOT, cnpjCpf(c.CNPJ, c.CPF)))
	}
	for _, v := range r.Vale {
		linha(fmt.Sprintf("Vale-pedágio - Fornecedora CNPJ: %s   Responsável pgto: %s   Comprovante: %s   Valor: %s",
			cnpjCpf(v.CNPJForn, ""), cnpjCpf(v.CNPJPg, v.CPFPg), v.NCompra, moeda(v.VVale)))
	}
	for _, c := range r.Contratantes {
		id := cnpjCpf(c.CNPJ, c.CPF)
		if id == "" {
			id = c.IDEstr
		}
		s := fmt.Sprintf("Contratante: %s - %s", c.XNome, id)
		if c.Contrato != "" {
			s += fmt.Sprintf("   Contrato: %s   Valor global: %s", c.Contrato, moeda(c.VGlobal))
		}
		linha(s)
	}
	for _, p := range r.Pagamentos { // resumido: contrato + forma de pagamento
		forma := map[string]string{"0": "à vista", "1": "a prazo"}[p.IndPag]
		linha(fmt.Sprintf("Pagamento do frete - %s - %s   Valor do contrato: %s   Forma: %s",
			p.XNome, cnpjCpf(p.CNPJ, p.CPF), moeda(p.VContrato), forma))
	}
	fecha()

	if len(inf.Seg) > 0 {
		secao("Seguro da carga")
		for _, s := range inf.Seg {
			resp := map[string]string{"1": "emitente", "2": "contratante"}[s.RespSeg]
			linha(fmt.Sprintf("Responsável: %s %s   Seguradora: %s CNPJ %s   Apólice: %s   Averbação: %s",
				resp, cnpjCpf(s.RespCNPJ, s.RespCPF), s.XSeg, cnpjCpf(s.SegCNPJ, ""), s.NApol, strings.Join(s.NAver, ", ")))
		}
		fecha()
	}

	// Documentos por município de descarregamento, 2 colunas; a quebra de
	// página é automática e o cabeçalho se repete (SetHeaderFuncMode).
	for _, m := range inf.Munic {
		secao("Município de descarregamento: " + m.XMun)
		doc := func(tipo string, chs []string) {
			for i := 0; i < len(chs); i += 2 {
				pdf.SetX(mg)
				for j := i; j < i+2 && j < len(chs); j++ {
					pdf.CellFormat(lw/2, 4.2, tr(tipo+" "+grupos4(strings.TrimSpace(chs[j]))), "", 0, "L", false, 0, "")
				}
				pdf.Ln(4.2)
			}
		}
		doc("NF-e", m.NFe)
		doc("CT-e", m.CTe)
		doc("MDF-e", m.MDFe)
		pdf.Ln(2)
	}

	if inf.AdFisco != "" || inf.Cpl != "" {
		secao("Observações")
		if inf.AdFisco != "" {
			linha("Fisco: " + inf.AdFisco)
		}
		if inf.Cpl != "" {
			linha(inf.Cpl)
		}
		fecha()
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("damdfe: %w", err)
	}
	return out.Bytes(), nil
}
