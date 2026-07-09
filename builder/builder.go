package builder

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// ── Input ─────────────────────────────────────────────────────────────────────
// EntradaNFe é a struct de alto nível que o caller preenche.
// O builder converte ela para NFe (schema SEFAZ).

type EntradaNFe struct {
	// Identificação
	Serie   string    // ex: "1"
	NNF     string    // número da nota ex: "42"
	DhEmi   time.Time // data/hora de emissão
	NatOp   string    // "VENDA DE MERCADORIA"
	TpAmb   string    // "1"=produção "2"=homologação
	FinNFe  string    // "1"=normal
	IndFinal string   // "1"=consumidor final
	IndPres  string   // "1"=presencial

	// Modo de emissão — padrão "1" (normal). Use "5" para FS-DA (contingência offline).
	TpEmis string // "1"=normal "5"=FS-DA
	DhCont string // data/hora entrada em contingência ISO8601 (obrigatório quando TpEmis="5")
	XJust  string // justificativa contingência ≥15 chars (obrigatório quando TpEmis="5")

	// Modelo do documento: "55"=NF-e (padrão), "65"=NFC-e
	Mod string

	// NFC-e — obrigatório quando Mod="65" (fornecido pela SEFAZ estadual)
	CSC             string // Código de Segurança do Contribuinte
	CSCId           string // identificador do CSC com zeros à esquerda (ex: "000001")
	UrlConsultaNFCe string // URL base da SEFAZ estadual (ex: "https://www.sefaz.go.gov.br/...")


	// Referência à NF-e original — obrigatório quando FinNFe="2" (complementar) ou "4" (devolução)
	ChaveNFeRef string // chave de acesso 44 dígitos da NF-e referenciada

	Emitente  EntradaEmitente
	Dest      EntradaDest
	Itens     []EntradaItem
	Frete     EntradaFrete
	Pagamento []EntradaPagamento
	InfCpl    string // informações complementares
}

type EntradaEmitente struct {
	CNPJ   string
	Nome   string
	Fantasia string
	IE     string
	CRT    string // "1", "2" ou "3"
	End    EntradaEndereco
}

type EntradaDest struct {
	CNPJ   string // preencher CNPJ ou CPF
	CPF    string
	Nome   string
	IE     string
	IndIEDest string // "1", "2" ou "9"
	Email  string
	End    EntradaEndereco
}

type EntradaEndereco struct {
	Logradouro string
	Numero     string
	Complemento string
	Bairro     string
	CodigoMun  string // código IBGE 7 dígitos
	Municipio  string
	UF         string
	CEP        string
	Pais       string // "1058" = Brasil
	NomePais   string // "Brasil"
	Fone       string
}

type EntradaItem struct {
	CProd      string  // código interno do produto
	CEAN       string  // código de barras EAN-13 (ou "SEM GTIN")
	Nome       string
	NCM        string  // ex: "73089090" para estruturas metálicas
	CFOP       string  // ex: "5102"
	Unidade    string  // "UN", "KG", "M2", etc.
	Quantidade float64
	VUnitario  float64
	VDesconto  float64
	ICMS       EntradaICMS
	IPI        *EntradaIPI  // nil = sem IPI
}

type EntradaICMS struct {
	// Regime Normal
	CST    string  // "00", "20", "40", "60", "90"
	ModBC  string  // "3" = valor da operação
	PRedBC float64 // % redução de BC (ICMS20)
	Aliq   float64 // alíquota %
	// Simples Nacional
	CSOSN  string  // "102", "400", "500", etc.
	// ST prospectivo (CST 10)
	ModBCST string
	PMVAST  float64
	AliqST  float64
	// ST retido anteriormente (CST 60)
	VBCSTRet   float64
	PSTRet     float64
	VICMSSTRet float64
	// Desoneração
	VICMSDeson float64
	MotDesICMS string
}

type EntradaIPI struct {
	CEnq string  // código de enquadramento
	CST  string  // "50"=tributado por alíq, "99"=outros
	Aliq float64
}

type EntradaFrete struct {
	Modalidade string // "0"=CIF "1"=FOB "9"=sem frete
	VFrete     float64
}

type EntradaPagamento struct {
	Forma  string  // "01"=dinheiro "15"=boleto "99"=outros
	XPag   string  // obrigatório quando Forma="99" (descrição do meio de pagamento)
	Valor  float64
	APrazo bool
}

// ── Build ─────────────────────────────────────────────────────────────────────

// Build converte uma EntradaNFe em []byte do XML pronto para assinar.
func Build(e EntradaNFe) ([]byte, ChaveAcesso, error) {
	if err := validarEntrada(e); err != nil {
		return nil, ChaveAcesso{}, fmt.Errorf("builder: %w", err)
	}

	mod := e.Mod
	if mod == "" {
		mod = ModeloNFe
	}
	chave := NovaChave(e.Emitente.End.UF, FormatarCNPJ(e.Emitente.CNPJ),
		e.Serie, e.NNF, "1", mod, e.DhEmi)

	nfe, err := montarNFe(e, chave)
	if err != nil {
		return nil, ChaveAcesso{}, fmt.Errorf("builder: %w", err)
	}

	data, err := xml.Marshal(nfe)
	if err != nil {
		return nil, ChaveAcesso{}, fmt.Errorf("builder: marshal: %w", err)
	}

	// Adiciona declaração XML e garante sem espaços extras
	xmlBytes := []byte(xml.Header + string(data))
	return xmlBytes, chave, nil
}

// ── Montagem ─────────────────────────────────────────────────────────────────

func montarNFe(e EntradaNFe, chave ChaveAcesso) (NFe, error) {
	detalhes, totais, err := montarDetalhes(e)
	if err != nil {
		return NFe{}, err
	}

	// modelo e tipo de impressão
	mod := e.Mod
	if mod == "" {
		mod = ModeloNFe
	}
	tpImp := "1"
	if mod == ModeloNFCe {
		tpImp = "4"
	}

	// dest como ponteiro — NFC-e permite consumidor não identificado (dest nil)
	var dest *Destinatario
	if e.Dest.CNPJ != "" || e.Dest.CPF != "" || e.Dest.Nome != "" {
		d := montarDest(e.Dest)
		dest = &d
	}

	// idDest: 1=interna, 2=interestadual (só compara UFs quando há dest)
	idDest := "1"
	if e.Dest.End.UF != "" && e.Emitente.End.UF != e.Dest.End.UF {
		idDest = "2"
	}

	tpAmb := e.TpAmb
	if tpAmb == "" {
		tpAmb = "2" // default homologação
	}
	finNFe := e.FinNFe
	if finNFe == "" {
		finNFe = "1"
	}

	tpEmis := e.TpEmis
	if tpEmis == "" {
		tpEmis = "1"
	}

	var nfref []NFref
	if e.ChaveNFeRef != "" {
		nfref = []NFref{{RefNFe: e.ChaveNFeRef}}
	}

	nfe := NFe{
		Xmlns: NsNFe,
		InfNFe: InfNFe{
			Versao: VersaoNFe,
			Id:     chave.ID(),
			Ide: Ide{
				CUF:      chave.CUF,
				CNF:      chave.CNF,
				NatOp:    e.NatOp,
				Mod:      mod,
				Serie:    semZerosEsquerda(chave.Serie),
				NNF:      semZerosEsquerda(chave.NNF),
				DhEmi:    e.DhEmi.Format("2006-01-02T15:04:05-07:00"),
				TpNF:     "1",
				IdDest:   idDest,
				CMunFG:   e.Emitente.End.CodigoMun,
				TpImp:    tpImp,
				TpEmis:   tpEmis,
				CDV:      chave.CDV,
				TpAmb:    tpAmb,
				FinNFe:   finNFe,
				IndFinal: e.IndFinal,
				IndPres:  e.IndPres,
				ProcEmi:  "0",
				VerProc:  "nfe-go v0.1",
				DhCont:   e.DhCont,
				XJust:    e.XJust,
				NFref:    nfref,
			},
			Emit: montarEmitente(e.Emitente),
			Dest: dest,
			Det:  detalhes,
			Total: Total{ICMSTot: totais},
			Transp: Transporte{
				ModFrete: e.Frete.Modalidade,
			},
			Pag:    montarPagamento(e.Pagamento),
			InfAdic: func() *InfAdic {
				if e.InfCpl == "" {
					return nil
				}
				return &InfAdic{InfCpl: e.InfCpl}
			}(),
		},
	}

	if mod == ModeloNFCe {
		nfe.InfNFeSupl = montarQRCode(e, chave, totais, tpAmb)
	}

	return nfe, nil
}

// montarQRCode gera o bloco infNFeSupl com QR Code e URL de consulta para NFC-e.
// signAC = SHA-1(chNFe + nVersao + tpAmb + cDest + dhEmi + vNF + vICMS + dIEDest + cIdToken + CSC).
func montarQRCode(e EntradaNFe, chave ChaveAcesso, totais ICMSTot, tpAmb string) *InfNFeSupl {
	chNFe := chave.String()
	nVersao := "100"
	cDest := e.Dest.CPF // CPF do consumidor ou vazio

	dhEmi := e.DhEmi.Format("20060102150405")
	vNF := totais.VNF
	vICMS := totais.VICMS

	dIEDest := ""
	switch e.Dest.IndIEDest {
	case "1":
		dIEDest = e.Dest.IE
	case "2":
		dIEDest = "ISENTO"
	}

	cIdToken := e.CSCId

	// signAC: SHA-1 de todos os campos + CSC, resultado em hex maiúsculo (40 chars)
	toSign := chNFe + nVersao + tpAmb + cDest + dhEmi + vNF + vICMS + dIEDest + cIdToken + e.CSC
	h := sha1.New()
	h.Write([]byte(toSign))
	signAC := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))

	// QR Code: formato pipe-delimited p= (padrão NT 2013.005 NFC-e)
	partes := strings.Join([]string{chNFe, nVersao, tpAmb, cDest, dhEmi, vNF, vICMS, dIEDest, cIdToken, signAC}, "|")
	qrCode := e.UrlConsultaNFCe + "?p=" + partes
	urlChave := e.UrlConsultaNFCe + "?chNFe=" + chNFe

	return &InfNFeSupl{QrCode: qrCode, UrlChave: urlChave}
}

func montarEmitente(e EntradaEmitente) Emitente {
	return Emitente{
		CNPJ:  FormatarCNPJ(e.CNPJ),
		XNome: e.Nome,
		XFant: e.Fantasia,
		IE:    e.IE,
		CRT:   e.CRT,
		EnderEmit: EnderecoEmit{
			XLgr:    e.End.Logradouro,
			Nro:     e.End.Numero,
			XCpl:    e.End.Complemento,
			XBairro: e.End.Bairro,
			CMun:    e.End.CodigoMun,
			XMun:    e.End.Municipio,
			UF:      e.End.UF,
			CEP:     FormatarCEP(e.End.CEP),
			CPais:   e.End.Pais,
			XPais:   e.End.NomePais,
			Fone:    e.End.Fone,
		},
	}
}

func montarDest(d EntradaDest) Destinatario {
	return Destinatario{
		CNPJ:      FormatarCNPJ(d.CNPJ),
		CPF:       d.CPF,
		XNome:     d.Nome,
		IndIEDest: d.IndIEDest,
		IE:        d.IE,
		Email:     d.Email,
		EnderDest: EnderecoDest{
			XLgr:    d.End.Logradouro,
			Nro:     d.End.Numero,
			XCpl:    d.End.Complemento,
			XBairro: d.End.Bairro,
			CMun:    d.End.CodigoMun,
			XMun:    d.End.Municipio,
			UF:      d.End.UF,
			CEP:     FormatarCEP(d.End.CEP),
			CPais:   d.End.Pais,
			XPais:   d.End.NomePais,
			Fone:    d.End.Fone,
		},
	}
}

func montarDetalhes(e EntradaNFe) ([]Detalhe, ICMSTot, error) {
	var detalhes []Detalhe
	tot := ICMSTot{}
	vProdTotal := 0.0

	for i, item := range e.Itens {
		vProd := item.Quantidade * item.VUnitario
		vProdLiq := vProd - item.VDesconto
		vProdTotal += vProdLiq

		det := Detalhe{
			NItem: fmt.Sprintf("%d", i+1),
			Prod: Produto{
				CProd:    item.CProd,
				CEAN:     ceanOuSemGTIN(item.CEAN),
				XProd:    item.Nome,
				NCM:      item.NCM,
				CFOP:     item.CFOP,
				UCom:     item.Unidade,
				QCom:     fmtQtd(item.Quantidade),
				VUnCom:   fmtVal(item.VUnitario),
				VProd:    fmtVal(vProd),
				CEANTrib: ceanOuSemGTIN(item.CEAN),
				UTrib:    item.Unidade,
				QTrib:    fmtQtd(item.Quantidade),
				VUnTrib:  fmtVal(item.VUnitario),
				VDesc:    fmtValOmitZero(item.VDesconto),
				IndTot:   "1",
			},
			Imposto: montarImposto(item, e.Emitente.CRT),
		}

		if item.VDesconto > 0 {
			v, _ := parseFloat(tot.VDesc)
			tot.VDesc = fmtVal(v + item.VDesconto)
		}
		if e.Frete.VFrete > 0 && i == 0 {
			det.Prod.VFrete = fmtVal(e.Frete.VFrete)
		}

		detalhes = append(detalhes, det)
	}

	tot.VProd = fmtVal(vProdTotal)
	tot.VFrete = fmtVal(e.Frete.VFrete)
	tot.VNF = fmtVal(vProdTotal + e.Frete.VFrete)
	tot.VBC = "0.00"
	tot.VICMS = "0.00"
	tot.VICMSDeson = "0.00"
	tot.VFCP = "0.00"
	tot.VBCST = "0.00"
	tot.VST = "0.00"
	tot.VFCPST = "0.00"
	tot.VFCPSTRet = "0.00"
	tot.VSeg = "0.00"
	tot.VII = "0.00"
	tot.VIPI = "0.00"
	tot.VIPIDevol = "0.00"
	tot.VPIS = "0.00"
	tot.VCOFINS = "0.00"
	tot.VOutro = "0.00"
	tot.VTotTrib = "0.00"
	if tot.VDesc == "" {
		tot.VDesc = "0.00"
	}

	return detalhes, tot, nil
}

func montarImposto(item EntradaItem, crt string) Imposto {
	imp := Imposto{}

	icms := &ICMS{}
	// Simples Nacional (CRT 1 ou 2)
	if crt == "1" || crt == "2" {
		csosn := item.ICMS.CSOSN
		if csosn == "" {
			csosn = "400" // isento/sem destaque (mais comum no SN)
		}
		icms.ICMSSN102 = &ICMSSN102{Orig: "0", CSOSN: csosn}
		imp.ICMS = icms
		imp.PIS = PIS{PISNt: &PISNt{CST: "07"}}
		imp.COFINS = COFINS{COFINSNt: &COFINSNt{CST: "07"}}
		return imp
	}

	// Regime Normal (CRT 3)
	cst := item.ICMS.CST
	if cst == "" {
		cst = "00"
	}
	vProd := item.Quantidade * item.VUnitario

	switch cst {
	case "00":
		vBC := vProd
		vICMS := vBC * item.ICMS.Aliq / 100
		icms.ICMS00 = &ICMS00{
			Orig: "0", CST: "00", ModBC: "3",
			VBC: fmtVal(vBC), PICMS: fmtVal(item.ICMS.Aliq), VICMS: fmtVal(vICMS),
		}
	case "10":
		vBC := vProd
		vICMS := vBC * item.ICMS.Aliq / 100
		modBCST := item.ICMS.ModBCST
		if modBCST == "" {
			modBCST = "4" // padrão: MVA ajustado
		}
		vBCST := vProd * (1 + item.ICMS.PMVAST/100)
		vICMSST := vBCST*item.ICMS.AliqST/100 - vICMS
		if vICMSST < 0 {
			vICMSST = 0
		}
		icms.ICMS10 = &ICMS10{
			Orig: "0", CST: "10", ModBC: "3",
			VBC: fmtVal(vBC), PICMS: fmtVal(item.ICMS.Aliq), VICMS: fmtVal(vICMS),
			ModBCST: modBCST, PMVAST: fmtVal(item.ICMS.PMVAST),
			VBCST: fmtVal(vBCST), PICMSST: fmtVal(item.ICMS.AliqST), VICMSST: fmtVal(vICMSST),
		}
	case "40", "41", "50":
		icms.ICMS40 = &ICMS40{Orig: "0", CST: cst}
	case "60":
		icms.ICMS60 = &ICMS60{
			Orig:       "0",
			CST:        "60",
			VBCSTRet:   fmtVal(item.ICMS.VBCSTRet),
			PSTRet:     fmtVal(item.ICMS.PSTRet),
			VICMSSTRet: fmtVal(item.ICMS.VICMSSTRet),
		}
	case "20":
		vBC := vProd * (1 - item.ICMS.PRedBC/100)
		vICMS := vBC * item.ICMS.Aliq / 100
		icms.ICMS20 = &ICMS20{
			Orig: "0", CST: "20", ModBC: "3",
			PRedBC: fmtVal(item.ICMS.PRedBC), VBC: fmtVal(vBC),
			PICMS: fmtVal(item.ICMS.Aliq), VICMS: fmtVal(vICMS),
		}
	default:
		icms.ICMS40 = &ICMS40{Orig: "0", CST: "40"}
	}

	imp.ICMS = icms
	imp.PIS = PIS{PISAliq: &PISAliq{CST: "01", VBC: fmtVal(vProd), PPIS: "0.65", VPIS: fmtVal(vProd * 0.0065)}}
	imp.COFINS = COFINS{COFINSAliq: &COFINSAliq{CST: "01", VBC: fmtVal(vProd), PCOFINS: "3.00", VCOFINS: fmtVal(vProd * 0.03)}}

	if item.IPI != nil {
		vIPI := vProd * item.IPI.Aliq / 100
		imp.IPI = &IPI{
			CEnq: item.IPI.CEnq,
			IPITrib: &IPITrib{CST: item.IPI.CST, VBC: fmtVal(vProd), PIPI: fmtVal(item.IPI.Aliq), VIPI: fmtVal(vIPI)},
		}
	}

	return imp
}

func montarPagamento(ps []EntradaPagamento) Pagamento {
	pag := Pagamento{}
	for _, p := range ps {
		indPag := "0"
		if p.APrazo {
			indPag = "1"
		}
		xPag := p.XPag
		if p.Forma == "99" && xPag == "" {
			xPag = "Outros" // xPag é obrigatório quando tPag=99
		}
		pag.DetPag = append(pag.DetPag, DetalhePag{
			IndPag: indPag,
			TPag:   p.Forma,
			XPag:   xPag,
			VPag:   fmtVal(p.Valor),
		})
	}
	if len(pag.DetPag) == 0 {
		pag.DetPag = []DetalhePag{{TPag: "90", VPag: "0.00"}} // sem pagamento
	}
	return pag
}

// ── Validação mínima ─────────────────────────────────────────────────────────

func validarEntrada(e EntradaNFe) error {
	if len(FormatarCNPJ(e.Emitente.CNPJ)) != 14 {
		return fmt.Errorf("CNPJ do emitente inválido")
	}
	if e.Emitente.End.UF == "" {
		return fmt.Errorf("UF do emitente obrigatória")
	}
	if len(e.Itens) == 0 {
		return fmt.Errorf("NF-e sem itens")
	}
	if e.NNF == "" {
		return fmt.Errorf("número da nota obrigatório")
	}
	if e.Mod == ModeloNFCe && e.Dest.CNPJ != "" {
		return fmt.Errorf("NFC-e (mod=65) não aceita destinatário com CNPJ — use CPF ou deixe o destinatário sem identificação")
	}
	if e.Mod == ModeloNFCe && e.CSC == "" {
		return fmt.Errorf("NFC-e (mod=65) exige CSC (Código de Segurança do Contribuinte) fornecido pela SEFAZ estadual")
	}
	if (e.FinNFe == "2" || e.FinNFe == "4") && e.ChaveNFeRef == "" {
		return fmt.Errorf("finNFe=%s exige ChaveNFeRef preenchida (44 dígitos da NF-e original)", e.FinNFe)
	}
	if e.TpEmis == "5" {
		if e.DhCont == "" {
			return fmt.Errorf("TpEmis=5 (FS-DA) exige DhCont preenchida (data/hora entrada em contingência)")
		}
		if len(e.XJust) < 15 {
			return fmt.Errorf("TpEmis=5 (FS-DA) exige XJust com pelo menos 15 caracteres (atual: %d)", len(e.XJust))
		}
	}
	return nil
}

// ── Helpers de formatação ─────────────────────────────────────────────────────

func fmtVal(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func fmtValOmitZero(v float64) string {
	if v == 0 {
		return ""
	}
	return fmtVal(v)
}

func fmtQtd(v float64) string {
	return fmt.Sprintf("%.4f", v)
}

func ceanOuSemGTIN(ean string) string {
	if ean == "" {
		return "SEM GTIN"
	}
	return ean
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
