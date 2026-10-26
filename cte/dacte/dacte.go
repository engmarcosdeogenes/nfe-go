// Package dacte gera o DACTE (Documento Auxiliar do CT-e, modelo 57, leiaute
// 4.00) em PDF A4 retrato a partir do XML cteProc (autorizado) ou do CTe
// assinado (sem protocolo = prévia sem valor fiscal).
//
// Uso:
//
//	pdf, err := dacte.Gerar(cteXML, dacte.Opcoes{})
//
// Base normativa: MOC CT-e 4.00 online, Anexo II (DACTE),
// http://moc.sped.fazenda.pr.gov.br/CTe/DACTE.html, citada como "MOC DACTE §n"
// (§2 especificações, §3.1 correlação XML x DACTE). Os modelos de §4 são
// imagens: a posição exata dos quadros segue o padrão de mercado e está
// marcada "não confirmado no MOC".
package dacte

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

// Opcoes: o XML autorizado não muda com o cancelamento (evento à parte), então
// o chamador informa o estado atual para o papel refletir.
type Opcoes struct{ Cancelado bool }

// compressao só é desligada nos testes, para buscar texto no PDF.
var compressao = true

// ── parse (encoding/xml casa pelo nome local; o namespace é o do schema cte) ──

type xEnd struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	XMun    string `xml:"xMun"`
	CEP     string `xml:"CEP"`
	UF      string `xml:"UF"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone"`
}

// xParty serve rem, exped, receb, dest e toma4; cada papel usa um nome de
// endereço (enderReme, enderExped, enderReceb, enderDest, enderToma).
type xParty struct {
	CNPJ  string `xml:"CNPJ"`
	CPF   string `xml:"CPF"`
	IE    string `xml:"IE"`
	XNome string `xml:"xNome"`
	XFant string `xml:"xFant"`
	Fone  string `xml:"fone"`
	ISUF  string `xml:"ISUF"`
	Toma  string `xml:"toma"`
	E1    xEnd   `xml:"enderReme"`
	E2    xEnd   `xml:"enderExped"`
	E3    xEnd   `xml:"enderReceb"`
	E4    xEnd   `xml:"enderDest"`
	E5    xEnd   `xml:"enderToma"`
}

func (p xParty) end() xEnd {
	for _, e := range []xEnd{p.E1, p.E2, p.E3, p.E4, p.E5} {
		if e.XMun != "" || e.XLgr != "" {
			return e
		}
	}
	return xEnd{}
}

// xICMS reúne os campos de todos os grupos ICMS* (a escolha é um xs:choice).
type xICMS struct {
	CST     string `xml:"CST"`
	PRedBC  string `xml:"pRedBC"`
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
	VBCST   string `xml:"vBCSTRet"`
	VICMSST string `xml:"vICMSSTRet"`
	PICMSST string `xml:"pICMSSTRet"`
	PRedOUF string `xml:"pRedBCOutraUF"`
	VBCOUF  string `xml:"vBCOutraUF"`
	PICMSOF string `xml:"pICMSOutraUF"`
	VICMSOF string `xml:"vICMSOutraUF"`
	IndSN   string `xml:"indSN"`
}

type xObsC struct {
	Campo string `xml:"xCampo,attr"`
	Texto string `xml:"xTexto"`
}

type xInf struct {
	ID  string `xml:"Id,attr"`
	Ide struct {
		CFOP           string `xml:"CFOP"`
		NatOp          string `xml:"natOp"`
		Mod            string `xml:"mod"`
		Serie          string `xml:"serie"`
		NCT            string `xml:"nCT"`
		DhEmi          string `xml:"dhEmi"`
		TpEmis         string `xml:"tpEmis"`
		TpAmb          string `xml:"tpAmb"`
		TpCTe          string `xml:"tpCTe"`
		Modal          string `xml:"modal"`
		TpServ         string `xml:"tpServ"`
		XMunIni        string `xml:"xMunIni"`
		UFIni          string `xml:"UFIni"`
		XMunFim        string `xml:"xMunFim"`
		UFFim          string `xml:"UFFim"`
		IndGlobalizado string `xml:"indGlobalizado"`
		Retira         string `xml:"retira"`
		XDetRetira     string `xml:"xDetRetira"`
		DhCont         string `xml:"dhCont"`
		XJust          string `xml:"xJust"`
		Toma3          string `xml:"toma3>toma"`
		Toma4          xParty `xml:"toma4"`
	} `xml:"ide"`
	Compl struct {
		XCaracAd  string   `xml:"xCaracAd"`
		XCaracSer string   `xml:"xCaracSer"`
		XEmi      string   `xml:"xEmi"`
		XOrig     string   `xml:"fluxo>xOrig"`
		XPass     []string `xml:"fluxo>pass>xPass"`
		XDest     string   `xml:"xDest"`
		XRota     string   `xml:"xRota"`
		XObs      string   `xml:"xObs"`
		ObsCont   []xObsC  `xml:"ObsCont"`
		ObsFisco  []xObsC  `xml:"ObsFisco"`
	} `xml:"compl"`
	Emit struct {
		CNPJ  string `xml:"CNPJ"`
		CPF   string `xml:"CPF"`
		IE    string `xml:"IE"`
		XNome string `xml:"xNome"`
		XFant string `xml:"xFant"`
		End   xEnd   `xml:"enderEmit"`
	} `xml:"emit"`
	Rem    xParty `xml:"rem"`
	Exped  xParty `xml:"exped"`
	Receb  xParty `xml:"receb"`
	Dest   xParty `xml:"dest"`
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
			G []xICMS `xml:",any"` // ICMS00, ICMS20, ICMS45, ICMS60, ICMS90, ICMSOutraUF, ICMSSN
		} `xml:"ICMS"`
		VTotTrib   string `xml:"vTotTrib"`
		InfAdFisco string `xml:"infAdFisco"`
		UFFim      struct {
			VBC     string `xml:"vBCUFFim"`
			PICMS   string `xml:"pICMSUFFim"`
			PInter  string `xml:"pICMSInter"`
			VICMS   string `xml:"vICMSUFFim"`
			VICMSIn string `xml:"vICMSUFIni"`
			VFCP    string `xml:"vFCPUFFim"`
		} `xml:"ICMSUFFim"`
		IBSCBS struct {
			CST        string `xml:"CST"`
			CClassTrib string `xml:"cClassTrib"`
			VBC        string `xml:"gIBSCBS>vBC"`
			PIBSUF     string `xml:"gIBSCBS>gIBSUF>pIBSUF"`
			VIBSUF     string `xml:"gIBSCBS>gIBSUF>vIBSUF"`
			PIBSMun    string `xml:"gIBSCBS>gIBSMun>pIBSMun"`
			VIBSMun    string `xml:"gIBSCBS>gIBSMun>vIBSMun"`
			VIBS       string `xml:"gIBSCBS>vIBS"`
			PCBS       string `xml:"gIBSCBS>gCBS>pCBS"`
			VCBS       string `xml:"gIBSCBS>gCBS>vCBS"`
		} `xml:"IBSCBS"`
		VTotDFe string `xml:"vTotDFe"`
	} `xml:"imp"`
	Norm struct {
		VCarga  string `xml:"infCarga>vCarga"`
		ProPred string `xml:"infCarga>proPred"`
		XOutCat string `xml:"infCarga>xOutCat"`
		InfQ    []struct {
			CUnid  string `xml:"cUnid"`
			TpMed  string `xml:"tpMed"`
			QCarga string `xml:"qCarga"`
		} `xml:"infCarga>infQ"`
		NF []struct {
			Mod   string `xml:"mod"`
			Serie string `xml:"serie"`
			NDoc  string `xml:"nDoc"`
			DEmi  string `xml:"dEmi"`
			VNF   string `xml:"vNF"`
		} `xml:"infDoc>infNF"`
		NFe []struct {
			Chave string `xml:"chave"`
		} `xml:"infDoc>infNFe"`
		Outros []struct {
			TpDoc      string `xml:"tpDoc"`
			DescOutros string `xml:"descOutros"`
			NDoc       string `xml:"nDoc"`
			DEmi       string `xml:"dEmi"`
			VDocFisc   string `xml:"vDocFisc"`
		} `xml:"infDoc>infOutros"`
		DCe []struct {
			Chave string `xml:"chave"`
		} `xml:"infDoc>infDCe"`
		DocAnt []struct {
			CNPJ  string `xml:"CNPJ"`
			CPF   string `xml:"CPF"`
			IE    string `xml:"IE"`
			UF    string `xml:"UF"`
			XNome string `xml:"xNome"`
			Id    []struct {
				Pap []struct {
					TpDoc  string `xml:"tpDoc"`
					Serie  string `xml:"serie"`
					Subser string `xml:"subser"`
					NDoc   string `xml:"nDoc"`
					DEmi   string `xml:"dEmi"`
				} `xml:"idDocAntPap"`
				Ele []struct {
					ChCTe string `xml:"chCTe"`
				} `xml:"idDocAntEle"`
			} `xml:"idDocAnt"`
		} `xml:"docAnt>emiDocAnt"`
		Rodo struct {
			RNTRC string `xml:"RNTRC"`
			Occ   []struct {
				Serie string `xml:"serie"`
				NOcc  string `xml:"nOcc"`
				DEmi  string `xml:"dEmi"`
				CNPJ  string `xml:"emiOcc>CNPJ"`
				IE    string `xml:"emiOcc>IE"`
				UF    string `xml:"emiOcc>UF"`
				Fone  string `xml:"emiOcc>fone"`
			} `xml:"occ"`
		} `xml:"infModal>rodo"`
		Aereo struct {
			NMinu    string   `xml:"nMinu"`
			NOCA     string   `xml:"nOCA"`
			DPrev    string   `xml:"dPrevAereo"`
			XDime    string   `xml:"natCarga>xDime"`
			CInfManu []string `xml:"natCarga>cInfManu"`
			CL       string   `xml:"tarifa>CL"`
			CTar     string   `xml:"tarifa>cTar"`
			VTar     string   `xml:"tarifa>vTar"`
			Peri     []struct {
				NONU    string `xml:"nONU"`
				QTotEmb string `xml:"qTotEmb"`
				QTotP   string `xml:"infTotAP>qTotProd"`
				UniAP   string `xml:"infTotAP>uniAP"`
			} `xml:"peri"`
		} `xml:"infModal>aereo"`
		Aquav struct {
			VPrest  string   `xml:"vPrest"`
			VAFRMM  string   `xml:"vAFRMM"`
			XNavio  string   `xml:"xNavio"`
			Balsa   []string `xml:"balsa>xBalsa"`
			NViag   string   `xml:"nViag"`
			Direc   string   `xml:"direc"`
			Irin    string   `xml:"irin"`
			TpNav   string   `xml:"tpNav"`
			DetCont []struct {
				NCont  string   `xml:"nCont"`
				Lacres []string `xml:"lacre>nLacre"`
				NF     []string `xml:"infDoc>infNF>nDoc"`
				NFe    []string `xml:"infDoc>infNFe>chave"`
			} `xml:"detCont"`
		} `xml:"infModal>aquav"`
		Ferrov struct {
			TpTraf  string `xml:"tpTraf"`
			Fluxo   string `xml:"fluxo"`
			RespFat string `xml:"trafMut>respFat"`
			FerrEmi string `xml:"trafMut>ferrEmi"`
			VFrete  string `xml:"trafMut>vFrete"`
			ChOrig  string `xml:"trafMut>chCTeFerroOrigem"`
			Env     []struct {
				CNPJ string `xml:"CNPJ"`
				Nome string `xml:"xNome"`
				Mun  string `xml:"enderFerro>xMun"`
				UF   string `xml:"enderFerro>UF"`
			} `xml:"trafMut>ferroEnv"`
		} `xml:"infModal>ferrov"`
		Duto struct {
			VTar          string `xml:"vTar"`
			DIni          string `xml:"dIni"`
			DFim          string `xml:"dFim"`
			ClassDuto     string `xml:"classDuto"`
			TpContratacao string `xml:"tpContratacao"`
			Entrada       string `xml:"codPontoEntrada"`
			Saida         string `xml:"codPontoSaida"`
			NContrato     string `xml:"nContrato"`
		} `xml:"infModal>duto"`
		MM struct {
			COTM  string `xml:"COTM"`
			Negoc string `xml:"indNegociavel"`
			XSeg  string `xml:"seg>infSeg>xSeg"`
			CNPJ  string `xml:"seg>infSeg>CNPJ"`
			NApol string `xml:"seg>nApol"`
			NAver string `xml:"seg>nAver"`
		} `xml:"infModal>multimodal"`
		Sub   string `xml:"infCteSub>chCte"`
		Comp  string `xml:"infCteComp>chCTe"`
		Glob  string `xml:"infGlobalizado>xObs"`
		Multi string `xml:"infServVinc>infCTeMultimodal>chCTeMultimodal"`
	} `xml:"infCTeNorm"`
	Comp2 string `xml:"infCteComp>chCTe"` // leiaute 4.00 também admite fora de infCTeNorm
}

type xSupl struct {
	QR string `xml:"qrCodCTe"`
}

type xRaiz struct {
	XMLName xml.Name
	CTe     struct {
		Info xInf  `xml:"infCte"`
		Supl xSupl `xml:"infCTeSupl"`
	} `xml:"CTe"`
	Prot struct {
		NProt    string `xml:"nProt"`
		DhRecbto string `xml:"dhRecbto"`
		CStat    string `xml:"cStat"`
	} `xml:"protCTe>infProt"`
	Info xInf  `xml:"infCte"`     // raiz é o próprio CTe
	Supl xSupl `xml:"infCTeSupl"` // idem
}

type protocolo struct{ nProt, dh string }

var reChave = regexp.MustCompile(`^[0-9A-Z]{44}$`) // CNPJ alfanumérico: MOC DACTE §2.18

func parse(b []byte) (inf *xInf, prot protocolo, qr string, err error) {
	var r xRaiz
	if err = xml.Unmarshal(b, &r); err != nil {
		return nil, prot, "", fmt.Errorf("dacte: XML inválido: %w", err)
	}
	switch r.XMLName.Local {
	case "cteProc":
		inf, qr = &r.CTe.Info, strings.TrimSpace(r.CTe.Supl.QR)
		if c := strings.TrimSpace(r.Prot.CStat); c != "" && c != "100" { // MOC DACTE §3.1: cStat=100
			return nil, prot, "", fmt.Errorf("dacte: protocolo com cStat %s: CT-e não autorizado", c)
		}
		prot = protocolo{strings.TrimSpace(r.Prot.NProt), r.Prot.DhRecbto}
	case "CTe":
		inf, qr = &r.Info, strings.TrimSpace(r.Supl.QR)
	default:
		return nil, prot, "", fmt.Errorf("dacte: raiz <%s> não é cteProc nem CTe", r.XMLName.Local)
	}
	if !reChave.MatchString(strings.TrimPrefix(inf.ID, "CTe")) {
		return nil, prot, "", fmt.Errorf("dacte: chave de acesso ausente ou inválida em infCte/@Id")
	}
	if m := strings.TrimSpace(inf.Ide.Mod); m != "57" {
		return nil, prot, "", fmt.Errorf("dacte: modelo %q não suportado (só 57)", m)
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

func data(s string) string {
	if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); err == nil {
		return t.Format("02/01/2006")
	}
	return s
}

// grupos4: MOC DACTE §2.5, onze blocos de quatro dígitos.
func grupos4(s string) string {
	var p []string
	for i := 0; i < len(s); i += 4 {
		p = append(p, s[i:min(i+4, len(s))])
	}
	return strings.Join(p, " ")
}

func cnpjCpf(cnpj, cpf string) string {
	if len(cnpj) == 14 && !strings.ContainsAny(cnpj, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return fmt.Sprintf("%s.%s.%s/%s-%s", cnpj[:2], cnpj[2:5], cnpj[5:8], cnpj[8:12], cnpj[12:])
	}
	if len(cnpj) == 14 { // alfanumérico: mesma máscara
		return fmt.Sprintf("%s.%s.%s/%s-%s", cnpj[:2], cnpj[2:5], cnpj[5:8], cnpj[8:12], cnpj[12:])
	}
	if len(cpf) == 11 {
		return fmt.Sprintf("%s.%s.%s-%s", cpf[:3], cpf[3:6], cpf[6:9], cpf[9:])
	}
	return cnpj + cpf
}

// num formata "1234.50" como "1.234,50" (separador de milhar e vírgula).
func num(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	i, d, _ := strings.Cut(v, ".")
	neg := strings.HasPrefix(i, "-")
	i = strings.TrimPrefix(i, "-")
	var o []byte
	for k := range i {
		if k > 0 && (len(i)-k)%3 == 0 {
			o = append(o, '.')
		}
		o = append(o, i[k])
	}
	s := string(o)
	if neg {
		s = "-" + s
	}
	if d != "" {
		s += "," + d
	}
	return s
}

func moeda(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "R$ " + num(v)
}

func ou(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

var (
	tpCTe  = map[string]string{"0": "0 - Normal", "1": "1 - Complemento de valores", "3": "3 - Substituição"}
	tpServ = map[string]string{"0": "0 - Normal", "1": "1 - Subcontratação", "2": "2 - Redespacho",
		"3": "3 - Redespacho intermediário", "4": "4 - Serviço vinculado a multimodal"}
	modais = map[string]string{"01": "01 - Rodoviário", "02": "02 - Aéreo", "03": "03 - Aquaviário",
		"04": "04 - Ferroviário", "05": "05 - Dutoviário", "06": "06 - Multimodal"}
	unidades = map[string]string{"00": "M3", "01": "KG", "02": "TON", "03": "UNID", "04": "LITROS", "05": "MMBTU"}
	papeis   = map[string]string{"0": "Remetente", "1": "Expedidor", "2": "Recebedor", "3": "Destinatário", "4": "Outros"}
)

// ── geração ──────────────────────────────────────────────────────────────────

const (
	mg    = 8.0   // margem
	lw    = 194.0 // largura útil A4
	fundo = 285.0 // limite inferior do conteúdo (A4 297 - rodapé)
)

// Gerar produz o PDF. Devolve erro (nunca panic) para XML malformado ou sem chave.
func Gerar(cteXML []byte, op Opcoes) (pdfOut []byte, err error) {
	defer func() { // a fpdf não deve derrubar o chamador por dado estranho
		if r := recover(); r != nil {
			pdfOut, err = nil, fmt.Errorf("dacte: %v", r)
		}
	}()
	inf, prot, qr, err := parse(cteXML)
	if err != nil {
		return nil, err
	}
	chave := strings.TrimPrefix(inf.ID, "CTe")

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(compressao)
	pdf.SetMargins(mg, mg, mg)
	pdf.SetAutoPageBreak(false, 0) // quebra manual (need) para não partir quadros
	pdf.AliasNbPages("{nb}")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // fonte core espera cp1252

	fonte := func(st string, sz float64) { pdf.SetFont("Helvetica", st, sz) }
	// texto numa linha, encolhendo a fonte (até 5pt) para caber na largura
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
	// célula com rótulo pequeno e valor
	cel := func(x, y, w, h float64, rot, val string) {
		pdf.Rect(x, y, w, h, "D")
		pdf.SetTextColor(90, 90, 90)
		fit(x+0.8, y+2.4, w-1.6, rot, "", 5.5)
		pdf.SetTextColor(0, 0, 0)
		fit(x+0.8, y+h-1.3, w-1.6, val, "B", 8)
	}

	// imagens registradas uma vez, reaproveitadas em cada folha
	var okBar, okQR bool
	if bc, e := code128.Encode(chave); e == nil { // MOC DACTE §2.18
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

	homolog := inf.Ide.TpAmb == "2"
	contingencia := inf.Ide.TpEmis == "2" // contingência offline (MOC DACTE §2.20.2: FS-DA/EPEC revogados)
	semProt := prot.nProt == "" && !contingencia

	// Cabeçalho repetido em toda folha (MOC DACTE §2.13): emitente, "DACTE",
	// modelo/série/número/folha, código de barras, chave, natureza, protocolo e QR.
	pdf.SetHeaderFuncMode(func() {
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextColor(0, 0, 0)
		e := inf.Emit
		pdf.Rect(mg, mg, 108, 28, "D")
		pdf.Rect(mg+108, mg, lw-108, 28, "D")
		fit(mg+1.5, mg+5, 105, ou(e.XNome, e.XFant), "B", 10) // §3.1: xNome ou xFant
		ed := e.End
		for i, l := range []string{
			"CNPJ/CPF: " + cnpjCpf(e.CNPJ, e.CPF) + "   IE: " + e.IE,
			strings.TrimSpace(ed.XLgr + ", " + ed.Nro + " " + ed.XCpl),
			strings.TrimSpace(ed.XBairro + " - " + ed.XMun + " - " + ed.UF),
			strings.TrimSpace("CEP: " + ed.CEP + "   Fone: " + ed.Fone),
		} {
			fit(mg+1.5, mg+10+float64(i)*4.4, 105, l, "", 7.5)
		}
		fit(mg+110, mg+7, 82, "DACTE", "B", 15)
		fit(mg+110, mg+11.5, 82, "Documento Auxiliar do Conhecimento de Transporte Eletrônico", "", 6)
		fit(mg+110, mg+16, 82, fmt.Sprintf("Modelo %s   Série %s   Nº %s", inf.Ide.Mod, inf.Ide.Serie, inf.Ide.NCT), "B", 8)
		fit(mg+110, mg+20.5, 82, fmt.Sprintf("Folha %d/{nb}     Modal: %s", pdf.PageNo(), modais[inf.Ide.Modal]), "", 7.5)
		fit(mg+110, mg+25, 82, "Emissão: "+dataHora(inf.Ide.DhEmi), "", 7.5)

		y := mg + 28
		pdf.Rect(mg, y, lw, 34, "D")
		if okBar {
			// MOC DACTE §2.18: CODE-128C da chave, mínimo 300 dpi (PNG de 900 px em 100 mm)
			pdf.Image("bar", mg+3, y+2, 100, 14, false, "", 0, "")
		}
		pdf.SetTextColor(90, 90, 90)
		fit(mg+1.5, y+19.5, 100, "Chave de acesso", "", 6)
		pdf.SetTextColor(0, 0, 0)
		fit(mg+1.5, y+24, 104, grupos4(chave), "B", 9)
		if okQR {
			// MOC DACTE §2.19: mínimo 25x25 mm incluindo 3 mm de margem segura
			pdf.Image("qr", mg+lw-30, y+3, 28, 28, false, "", 0, "")
		}
		// MOC DACTE §2.20.1: campo 1 = URL de consulta; campo 2 = protocolo + data/hora; campo 3 = QR.
		pdf.SetTextColor(90, 90, 90)
		fit(mg+108, y+4, 54, "Protocolo de autorização de uso", "", 6)
		pdf.SetTextColor(0, 0, 0)
		switch {
		case prot.nProt != "":
			fit(mg+108, y+9, 54, prot.nProt+" "+dataHora(prot.dh), "B", 8)
		case contingencia:
			fit(mg+108, y+9, 54, "EMISSÃO EM CONTINGÊNCIA", "B", 8.5) // texto não confirmado no MOC
		}
		pdf.SetTextColor(90, 90, 90)
		fit(mg+108, y+16, 54, "Consulta de autenticidade (chave de acesso):", "", 6)
		fit(mg+108, y+19.5, 54, "www.cte.fazenda.gov.br/portal", "", 6.5) // URL exata não confirmada no MOC
		pdf.SetTextColor(0, 0, 0)
		cel(mg, y+34, lw, 9, "CFOP - Natureza da operação", ou(inf.Ide.CFOP+" - "+inf.Ide.NatOp, ""))

		pdf.SetY(y + 44)
		pdf.SetTextColor(180, 0, 0)
		fonte("B", 9)
		for _, f := range []struct {
			on bool
			s  string
		}{{semProt, "SEM VALOR FISCAL - PRÉVIA SEM PROTOCOLO DE AUTORIZAÇÃO"}, // não confirmado no MOC
			{homolog, "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL"}} { // MOC DACTE §2.21
			if f.on {
				pdf.SetX(mg)
				pdf.CellFormat(lw, 6, tr(f.s), "1", 1, "C", false, 0, "")
			}
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(1)
		if op.Cancelado { // marca d'água (MOC DACTE §2.22.1); desenhada antes do corpo
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
	// linha de células; frac soma 1
	linha := func(h float64, frac []float64, rv ...string) {
		need(h)
		x, y := mg, pdf.GetY()
		for i, f := range frac {
			cel(x, y, lw*f, h, rv[2*i], rv[2*i+1])
			x += lw * f
		}
		pdf.SetY(y + h)
	}
	// texto livre multilinha dentro de caixa lateral
	texto := func(s string) {
		for _, ln := range pdf.SplitLines([]byte(tr(s)), lw-2) {
			need(5)
			fonte("", 7.5)
			pdf.SetX(mg)
			pdf.CellFormat(lw, 3.8, string(ln), "LR", 1, "L", false, 0, "")
		}
	}
	fecha := func() { pdf.Line(mg, pdf.GetY(), mg+lw, pdf.GetY()); pdf.Ln(1.5) }

	i := inf.Ide
	// Identificação do serviço (MOC DACTE §3.1: tpCTe, tpServ, tomador)
	papel, toma := tomador(inf)
	linha(9, []float64{.2, .22, .28, .3},
		"Tipo do CT-e", tpCTe[i.TpCTe],
		"Tipo do serviço", tpServ[i.TpServ],
		"Papel do tomador do serviço", papel,
		"Indicador de globalização", map[string]string{"1": "1 - CT-e globalizado"}[i.IndGlobalizado])
	linha(9, []float64{.5, .5},
		"Início da prestação", strings.TrimSpace(i.XMunIni+" - "+i.UFIni),
		"Término da prestação", strings.TrimSpace(i.XMunFim+" - "+i.UFFim))
	// chaves de documentos relacionados ao tipo do CT-e
	for _, k := range []struct{ rot, ch string }{
		{"CT-e substituído (infCteSub)", inf.Norm.Sub},
		{"CT-e complementado (infCteComp)", ou(inf.Norm.Comp, inf.Comp2)},
		{"CT-e multimodal vinculado", inf.Norm.Multi},
	} {
		if k.ch != "" {
			linha(8, []float64{1}, k.rot, grupos4(strings.TrimSpace(k.ch)))
		}
	}
	if contingencia && i.XJust != "" {
		linha(8, []float64{1}, "Contingência - justificativa ("+dataHora(i.DhCont)+")", i.XJust) // não confirmado no MOC
	}

	// Partes (MOC DACTE §2.8 e §3.1): remetente, destinatário e, se houver,
	// expedidor/recebedor; o tomador ocupa o lugar de quadro suprimido.
	parte := func(x, y, w float64, tit string, p xParty) {
		pdf.Rect(x, y, w, 23, "D")
		pdf.SetFillColor(235, 235, 235)
		pdf.Rect(x, y, w, 4, "DF")
		fit(x+1, y+2.9, w-2, tit, "B", 7)
		e := p.end()
		nome := ou(p.XNome, p.XFant)
		for k, l := range []string{
			nome,
			strings.TrimSpace(e.XLgr + ", " + e.Nro + " " + e.XCpl + " " + e.XBairro),
			strings.TrimSpace(e.XMun + " - " + e.UF + "   CEP: " + e.CEP + "   " + e.XPais),
			"CNPJ/CPF: " + cnpjCpf(p.CNPJ, p.CPF) + "   IE: " + p.IE,
			strings.TrimSpace("Fone: " + ou(p.Fone, e.Fone) + "   " + ou(p.ISUF, "")),
		} {
			st := ""
			if k == 0 {
				st = "B"
			}
			fit(x+1, y+7.6+float64(k)*3.7, w-2, l, st, 7)
		}
	}
	var ps []struct {
		t string
		p xParty
	}
	add := func(t string, p xParty) {
		if p.XNome != "" || p.CNPJ != "" || p.CPF != "" {
			ps = append(ps, struct {
				t string
				p xParty
			}{t, p})
		}
	}
	add("REMETENTE", inf.Rem)
	add("DESTINATÁRIO", inf.Dest)
	add("EXPEDIDOR", inf.Exped)
	add("RECEBEDOR", inf.Receb)
	add("TOMADOR DO SERVIÇO ("+strings.ToUpper(papel)+")", toma)
	for k := 0; k < len(ps); k += 2 {
		need(24)
		y := pdf.GetY()
		parte(mg, y, lw/2, ps[k].t, ps[k].p)
		if k+1 < len(ps) {
			parte(mg+lw/2, y, lw/2, ps[k+1].t, ps[k+1].p)
		}
		pdf.SetY(y + 23)
	}

	// Carga (MOC DACTE §3.1: proPred, xOutCat, vCarga, infQ)
	n := inf.Norm
	linha(9, []float64{.4, .35, .25},
		"Produto predominante", n.ProPred,
		"Outras características da carga", n.XOutCat,
		"Valor total da carga", moeda(n.VCarga))
	if len(n.InfQ) > 0 {
		var cs []string
		for _, q := range n.InfQ {
			cs = append(cs, strings.TrimSpace(q.TpMed)+": "+num(q.QCarga)+" "+unidades[q.CUnid])
		}
		secao("Quantidades da carga")
		texto(strings.Join(cs, "     "))
		fecha()
	}

	// Componentes do valor da prestação (MOC DACTE §3.1)
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

	// Informações relativas ao imposto (MOC DACTE §3.1: CST, vBC*, pICMS*, vICMS*, pRedBC*)
	secao("Informações relativas ao imposto")
	g := icmsEscolhido(inf)
	bc, al, vl, red := ou(g.VBC, ou(g.VBCST, g.VBCOUF)), ou(g.PICMS, ou(g.PICMSST, g.PICMSOF)),
		ou(g.VICMS, ou(g.VICMSST, g.VICMSOF)), ou(g.PRedBC, g.PRedOUF)
	cst := g.CST
	if g.IndSN != "" {
		cst += " - Simples Nacional"
	}
	linha(9, []float64{.28, .18, .14, .2, .2},
		"Classificação tributária do serviço (CST)", cst, "Base de cálculo", num(bc), "Alíquota ICMS (%)", num(al),
		"Valor do ICMS", num(vl), "% Red. BC cálc.", num(red))
	if t := inf.Imp; t.VTotTrib != "" || t.VTotDFe != "" {
		linha(8, []float64{.5, .5}, "Valor aproximado dos tributos", moeda(t.VTotTrib), "Valor total do DF-e (IBS/CBS)", moeda(t.VTotDFe))
	}
	if u := inf.Imp.UFFim; u.VBC != "" { // partilha ICMS UF fim; rótulos não confirmados no MOC
		linha(8, []float64{.2, .16, .16, .16, .16, .16},
			"Base ICMS UF fim", num(u.VBC), "% ICMS UF fim", num(u.PICMS), "% ICMS interestadual", num(u.PInter),
			"ICMS UF fim", num(u.VICMS), "ICMS UF início", num(u.VICMSIn), "FCP UF fim", num(u.VFCP))
	}
	if b := inf.Imp.IBSCBS; b.CST != "" { // reforma tributária; rótulos não confirmados no MOC
		linha(9, []float64{.14, .16, .18, .26, .26},
			"IBS/CBS - CST", b.CST, "cClassTrib", b.CClassTrib, "Base IBS/CBS", num(b.VBC),
			"IBS UF / Mun (R$)", num(b.VIBSUF)+" / "+num(b.VIBSMun), "CBS (R$)", num(b.VCBS))
		linha(8, []float64{.14, .16, .18, .26, .26},
			"", "", "", "", "IBS total (R$)", num(b.VIBS),
			"IBS UF / Mun (%)", num(b.PIBSUF)+" / "+num(b.PIBSMun), "CBS (%)", num(b.PCBS))
	}

	// Documentos originários (MOC DACTE §3.1: mod, CNPJ/CPF emitente, série/nº)
	var docs [][4]string // tipo, emitente, série/número, chave
	for _, d := range n.NFe {
		c := strings.TrimSpace(d.Chave)
		if len(c) == 44 {
			docs = append(docs, [4]string{"NFe", cnpjCpf(c[6:20], ""), strings.TrimLeft(c[22:25], "0") + "/" + strings.TrimLeft(c[25:34], "0"), grupos4(c)})
		} else {
			docs = append(docs, [4]string{"NFe", "", "", c})
		}
	}
	for _, d := range n.NF {
		docs = append(docs, [4]string{"NF " + d.Mod, cnpjCpf(inf.Rem.CNPJ, inf.Rem.CPF), d.Serie + "/" + d.NDoc, "Emissão " + data(d.DEmi) + "  Valor " + moeda(d.VNF)})
	}
	for _, d := range n.DCe {
		docs = append(docs, [4]string{"DC-e", "", "", grupos4(strings.TrimSpace(d.Chave))})
	}
	for _, d := range n.Outros {
		docs = append(docs, [4]string{"Outros " + d.TpDoc, cnpjCpf(inf.Rem.CNPJ, inf.Rem.CPF), d.NDoc, strings.TrimSpace(d.DescOutros + " " + data(d.DEmi))})
	}
	if len(docs) > 0 {
		cab := func() {
			secao("Documentos originários")
			pdf.SetFillColor(245, 245, 245)
			fonte("B", 6.5)
			x := mg
			for k, c := range []struct {
				t string
				w float64
			}{{"Tp. doc.", 20}, {"CNPJ/CPF emitente", 36}, {"Série/Nº documento", 30}, {"Chave de acesso / descrição", 108}} {
				_ = k
				pdf.SetXY(x, pdf.GetY())
				pdf.CellFormat(c.w, 4, tr(c.t), "1", 0, "L", true, 0, "")
				x += c.w
			}
			pdf.Ln(4)
		}
		cab()
		for _, d := range docs {
			if pdf.GetY()+4 > fundo {
				pdf.AddPage()
				cab()
			}
			x, y := mg, pdf.GetY()
			for k, w := range []float64{20, 36, 30, 108} {
				fit(x+1, y+3, w-2, d[k], "", 7)
				x += w
			}
			pdf.SetY(y + 4)
			pdf.SetDrawColor(200, 200, 200)
			pdf.Line(mg, y+4, mg+lw, y+4)
			pdf.SetDrawColor(0, 0, 0)
		}
		pdf.Ln(1.5)
	}

	// Documentos de transporte anterior (MOC DACTE §2.2/§2.13)
	if len(n.DocAnt) > 0 {
		secao("Documentos de transporte anterior")
		for _, e := range n.DocAnt {
			need(5)
			texto(fmt.Sprintf("Emitente: %s - %s   IE: %s   UF: %s", e.XNome, cnpjCpf(e.CNPJ, e.CPF), e.IE, e.UF))
			for _, id := range e.Id {
				for _, p := range id.Pap {
					texto(fmt.Sprintf("   Papel: tipo %s  série %s  subsérie %s  nº %s  emissão %s", p.TpDoc, p.Serie, p.Subser, p.NDoc, data(p.DEmi)))
				}
				for _, c := range id.Ele {
					texto("   CT-e: " + grupos4(strings.TrimSpace(c.ChCTe)))
				}
			}
		}
		fecha()
	}

	// Modal rodoviário (MOC DACTE §3.1: RNTRC; ordens de coleta de cteModalRodoviario v4.00)
	if r := n.Rodo; r.RNTRC != "" || len(r.Occ) > 0 {
		secao("Informações específicas do modal rodoviário")
		linha(8, []float64{1}, "RNTRC da empresa", r.RNTRC)
		for _, o := range r.Occ {
			texto(fmt.Sprintf("Ordem de coleta %s/%s  emissão %s  emitente %s  IE %s  UF %s  fone %s",
				o.Serie, o.NOcc, data(o.DEmi), cnpjCpf(o.CNPJ, ""), o.IE, o.UF, o.Fone))
		}
		fecha()
	}

	// Informações específicas do modal (MOC DACTE §3.1, bloco do modal; texto dos
	// rótulos e ordem dos campos por modal: não confirmado, sem acesso ao MOC).
	{
		var tit string
		var ls []string
		add := func(rot, v string) {
			if strings.TrimSpace(v) != "" {
				ls = append(ls, rot+": "+v)
			}
		}
		switch {
		case n.Aereo.CL != "":
			a := n.Aereo
			tit = "Informações específicas do modal aéreo"
			add("Nº da minuta", a.NMinu)
			add("Nº operacional do conhecimento aéreo", a.NOCA)
			add("Data prevista de entrega", data(a.DPrev))
			add("Dimensão", a.XDime)
			add("Informações de manuseio", strings.Join(a.CInfManu, ", "))
			add("Classe da tarifa", a.CL)
			add("Código da tarifa", a.CTar)
			add("Valor da tarifa", moeda(a.VTar))
			for _, p := range a.Peri {
				add("Carga perigosa - nº ONU", p.NONU+"  qtde. volumes: "+p.QTotEmb+"  total: "+num(p.QTotP)+"  unid.: "+p.UniAP)
			}
		case n.Aquav.XNavio != "":
			a := n.Aquav
			tit = "Informações específicas do modal aquaviário"
			add("Valor da prestação base AFRMM", moeda(a.VPrest))
			add("AFRMM", moeda(a.VAFRMM))
			add("Navio", a.XNavio)
			add("Balsas", strings.Join(a.Balsa, ", "))
			add("Nº da viagem", a.NViag)
			add("Direção", a.Direc)
			add("Irin", a.Irin)
			add("Tipo de navegação", a.TpNav)
			for _, d := range a.DetCont {
				add("Contêiner", d.NCont+"  lacres: "+strings.Join(d.Lacres, ", ")+"  NF: "+strings.Join(d.NF, ", ")+"  NF-e: "+strings.Join(d.NFe, ", "))
			}
		case n.Ferrov.TpTraf != "" || n.Ferrov.Fluxo != "":
			f := n.Ferrov
			tit = "Informações específicas do modal ferroviário"
			add("Tipo de tráfego", f.TpTraf)
			add("Fluxo ferroviário", f.Fluxo)
			add("Responsável pelo faturamento", f.RespFat)
			add("Ferrovia emitente", f.FerrEmi)
			add("Valor do frete", moeda(f.VFrete))
			add("CT-e da ferrovia de origem", grupos4(f.ChOrig))
			for _, e := range f.Env {
				add("Ferrovia envolvida", cnpjCpf(e.CNPJ, "")+" "+e.Nome+" "+e.Mun+"/"+e.UF)
			}
		case n.Duto.DIni != "":
			d := n.Duto
			tit = "Informações específicas do modal dutoviário"
			add("Valor da tarifa", num(d.VTar))
			add("Data de início", data(d.DIni))
			add("Data de fim", data(d.DFim))
			add("Classificação do duto", d.ClassDuto)
			add("Tipo de contratação", d.TpContratacao)
			add("Ponto de entrada", d.Entrada)
			add("Ponto de saída", d.Saida)
			add("Nº do contrato", d.NContrato)
		case n.MM.COTM != "":
			m := n.MM
			tit = "Informações específicas do modal multimodal"
			add("COTM", m.COTM)
			add("Indicador negociável", map[string]string{"0": "0 - Não negociável", "1": "1 - Negociável"}[m.Negoc])
			add("Seguradora", m.XSeg+" "+cnpjCpf(m.CNPJ, ""))
			add("Nº da apólice", m.NApol)
			add("Nº da averbação", m.NAver)
		}
		if tit != "" {
			secao(tit)
			for _, l := range ls {
				texto(l)
			}
			fecha()
		}
	}

	// Observações (MOC DACTE §3.1: xObs; ObsCont = uso do emissor; ObsFisco = reservado ao fisco)
	c := inf.Compl
	if c.XObs != "" || c.XCaracAd != "" || c.XCaracSer != "" || c.XOrig != "" || c.XDest != "" || c.XRota != "" ||
		inf.Imp.InfAdFisco != "" || n.Glob != "" || len(c.ObsCont) > 0 || len(c.ObsFisco) > 0 || i.XDetRetira != "" {
		secao("Observações")
		for _, s := range []string{
			c.XObs,
			cond(c.XCaracAd, "Característica adicional do transporte: "),
			cond(c.XCaracSer, "Característica adicional do serviço: "),
			cond(n.Glob, "CT-e globalizado: "),
			cond(i.XDetRetira, "Retirada da carga: "),
			cond(c.XOrig, "Fluxo da carga - origem: "),
			cond(strings.Join(c.XPass, ", "), "Fluxo da carga - passagem: "),
			cond(c.XDest, "Fluxo da carga - destino: "),
			cond(c.XRota, "Rota: "),
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

func cond(s, pre string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return pre + s
}

func icmsEscolhido(inf *xInf) xICMS {
	for _, g := range inf.Imp.ICMS.G {
		if g.CST != "" {
			return g
		}
	}
	return xICMS{}
}

// tomador resolve toma3 (referência a rem/exped/receb/dest) ou toma4 (MOC DACTE §3.1).
func tomador(inf *xInf) (string, xParty) {
	if t4 := inf.Ide.Toma4; t4.Toma != "" || t4.XNome != "" || t4.CNPJ != "" || t4.CPF != "" {
		return papeis["4"], t4
	}
	t := strings.TrimSpace(inf.Ide.Toma3)
	p := map[string]xParty{"0": inf.Rem, "1": inf.Exped, "2": inf.Receb, "3": inf.Dest}[t]
	return papeis[t], p
}
