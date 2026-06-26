package danfe

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// DadosDANFE reúne todos os dados extraídos do XML da NF-e para renderização.
type DadosDANFE struct {
	// Identificação
	ChaveAcesso string
	NumeroNota  string
	Serie       string
	DataEmissao string
	TipoNF      string // "0"=entrada "1"=saída
	NatOp       string
	TpAmb       string // "1"=produção "2"=homologação
	FinNFe      string

	// Protocolo (preenchido se vier nfeProc)
	NumProtocolo string
	DataAutorizacao string

	// Emitente
	EmitNome    string
	EmitFantasia string
	EmitCNPJ    string
	EmitIE      string
	EmitCRT     string
	EmitEnd     enderecoDANFE

	// Destinatário
	DestNome  string
	DestCNPJ  string
	DestCPF   string
	DestIE    string
	DestEnd   enderecoDANFE

	// Itens
	Itens []itemDANFE

	// Totais
	VBC      float64
	VICMS    float64
	VBCST    float64
	VST      float64
	VProd    float64
	VFrete   float64
	VSeg     float64
	VDesc    float64
	VIPI     float64
	VPIS     float64
	VCOFINS  float64
	VOutro   float64
	VNF      float64

	// Transporte
	ModFrete  string
	TranspNome string
	TranspCNPJ string
	TranspIE  string
	TranspEnd string
	TranspMun string
	TranspUF  string
	Volumes   []volDANFE

	// Pagamento
	Pagamentos []pagtoDANFE

	// Cobrança / Duplicatas
	Duplicatas []duplicataDANFE

	// Info adicional
	InfCpl  string
	InfAdFisco string
}

type enderecoDANFE struct {
	Logradouro string
	Numero     string
	Complemento string
	Bairro     string
	Municipio  string
	UF         string
	CEP        string
	Fone       string
}

type itemDANFE struct {
	Num       int
	CProd     string
	XProd     string
	NCM       string
	CST       string // CSOSN para SN, CST para RN
	CFOP      string
	Unidade   string
	Qtd       float64
	VUnit     float64
	VDesc     float64
	VProd     float64
	VBC       float64
	ICMS      float64
	AliqICMS  float64
	IPI       float64
	AliqIPI   float64
}

type pagtoDANFE struct {
	Forma string
	Valor float64
}

type volDANFE struct {
	Quantidade float64
	Especie    string
	Marca      string
	Numeracao  string
	PesoBruto  float64
	PesoLiq    float64
}

type duplicataDANFE struct {
	Num        string
	Vencimento string
	Valor      float64
}

// ── Parser XML ────────────────────────────────────────────────────────────────

// ParseNFeXML extrai os dados de um XML de NF-e (simples ou nfeProc).
func ParseNFeXML(xmlBytes []byte) (*DadosDANFE, error) {
	// Suporta tanto <NFe> direto quanto <nfeProc> (com protocolo)
	type xmlProtocolo struct {
		NProtocolo string `xml:"infProt>nProt"`
		DhRecbto   string `xml:"infProt>dhRecbto"`
	}
	type xmlEndereco struct {
		XLgr    string `xml:"xLgr"`
		Nro     string `xml:"nro"`
		XCpl    string `xml:"xCpl"`
		XBairro string `xml:"xBairro"`
		XMun    string `xml:"xMun"`
		UF      string `xml:"UF"`
		CEP     string `xml:"CEP"`
		Fone    string `xml:"fone"`
	}
	type xmlImposto struct {
		ICMS struct {
			ICMS00 *struct {
				PCST   string `xml:"pICMS"`
				VBC    string `xml:"vBC"`
				VICMS  string `xml:"vICMS"`
			} `xml:"ICMS00"`
			ICMS20 *struct {
				PCST   string `xml:"pICMS"`
				VBC    string `xml:"vBC"`
				VICMS  string `xml:"vICMS"`
			} `xml:"ICMS20"`
			ICMS40 *struct{} `xml:"ICMS40"`
			ICMSSN102 *struct{} `xml:"ICMSSN102"`
			ICMSSN500 *struct{} `xml:"ICMSSN500"`
		} `xml:"ICMS"`
		IPI *struct {
			PIPI  string `xml:"IPITrib>pIPI"`
			VIPI  string `xml:"IPITrib>vIPI"`
		} `xml:"IPI"`
	}
	type xmlDet struct {
		NItem string `xml:"nItem,attr"`
		Prod  struct {
			CProd  string `xml:"cProd"`
			XProd  string `xml:"xProd"`
			NCM    string `xml:"NCM"`
			CFOP   string `xml:"CFOP"`
			UCom   string `xml:"uCom"`
			QCom   string `xml:"qCom"`
			VUnCom string `xml:"vUnCom"`
			VDesc  string `xml:"vDesc"`
			VProd  string `xml:"vProd"`
		} `xml:"prod"`
		Imposto xmlImposto `xml:"imposto"`
	}
	type xmlTransp struct {
		ModFrete   string `xml:"modFrete"`
		Transporta struct {
			XNome  string `xml:"xNome"`
			CNPJ   string `xml:"CNPJ"`
			CPF    string `xml:"CPF"`
			IE     string `xml:"IE"`
			XEnder string `xml:"xEnder"`
			XMun   string `xml:"xMun"`
			UF     string `xml:"UF"`
		} `xml:"transporta"`
		Vol []struct {
			QVol  string `xml:"qVol"`
			Esp   string `xml:"esp"`
			Marca string `xml:"marca"`
			NVol  string `xml:"nVol"`
			PesoB string `xml:"pesoB"`
			PesoL string `xml:"pesoL"`
		} `xml:"vol"`
	}
	type xmlPag struct {
		DetPag []struct {
			TPag string `xml:"tPag"`
			VPag string `xml:"vPag"`
		} `xml:"detPag"`
	}
	type xmlNFe struct {
		InfNFe struct {
			ID   string `xml:"Id,attr"`
			Ide  struct {
				NNF     string `xml:"nNF"`
				Serie   string `xml:"serie"`
				DhEmi   string `xml:"dhEmi"`
				TpNF    string `xml:"tpNF"`
				NatOp   string `xml:"natOp"`
				TpAmb   string `xml:"tpAmb"`
				FinNFe  string `xml:"finNFe"`
			} `xml:"ide"`
			Emit struct {
				CNPJ   string `xml:"CNPJ"`
				XNome  string `xml:"xNome"`
				XFant  string `xml:"xFant"`
				IE     string `xml:"IE"`
				CRT    string `xml:"CRT"`
				Ender  xmlEndereco `xml:"enderEmit"`
			} `xml:"emit"`
			Dest struct {
				CNPJ      string `xml:"CNPJ"`
				CPF       string `xml:"CPF"`
				XNome     string `xml:"xNome"`
				IE        string `xml:"IE"`
				Ender     xmlEndereco `xml:"enderDest"`
			} `xml:"dest"`
			Det   []xmlDet `xml:"det"`
			Total struct {
				ICMSTot struct {
					VBC     string `xml:"vBC"`
					VICMS   string `xml:"vICMS"`
					VBCST   string `xml:"vBCST"`
					VST     string `xml:"vST"`
					VProd   string `xml:"vProd"`
					VFrete  string `xml:"vFrete"`
					VSeg    string `xml:"vSeg"`
					VDesc   string `xml:"vDesc"`
					VIPI    string `xml:"vIPI"`
					VPIS    string `xml:"vPIS"`
					VCOFINS string `xml:"vCOFINS"`
					VOutro  string `xml:"vOutro"`
					VNF     string `xml:"vNF"`
				} `xml:"ICMSTot"`
			} `xml:"total"`
			Transp xmlTransp `xml:"transp"`
			Pag    xmlPag    `xml:"pag"`
			Cobr   struct {
				Dup []struct {
					NDup  string `xml:"nDup"`
					DVenc string `xml:"dVenc"`
					VDup  string `xml:"vDup"`
				} `xml:"dup"`
			} `xml:"cobr"`
			InfAdic struct {
				InfCpl    string `xml:"infCpl"`
				InfAdFisco string `xml:"infAdFisco"`
			} `xml:"infAdic"`
		} `xml:"infNFe"`
	}
	type xmlNFeProc struct {
		NFe     xmlNFe       `xml:"NFe"`
		ProtNFe xmlProtocolo `xml:"protNFe"`
	}

	// Tenta parsear como nfeProc primeiro
	var proc xmlNFeProc
	var nfe xmlNFe
	if err := xml.Unmarshal(xmlBytes, &proc); err == nil && proc.NFe.InfNFe.ID != "" {
		nfe = proc.NFe
	} else if err := xml.Unmarshal(xmlBytes, &nfe); err != nil {
		return nil, fmt.Errorf("danfe: parse XML: %w", err)
	}

	inf := nfe.InfNFe
	d := &DadosDANFE{
		ChaveAcesso:     strings.TrimPrefix(inf.ID, "NFe"),
		NumeroNota:      inf.Ide.NNF,
		Serie:           inf.Ide.Serie,
		DataEmissao:     formatarDataHora(inf.Ide.DhEmi),
		TipoNF:          inf.Ide.TpNF,
		NatOp:           inf.Ide.NatOp,
		TpAmb:           inf.Ide.TpAmb,
		FinNFe:          inf.Ide.FinNFe,
		NumProtocolo:    proc.ProtNFe.NProtocolo,
		DataAutorizacao: formatarDataHora(proc.ProtNFe.DhRecbto),
		EmitNome:        inf.Emit.XNome,
		EmitFantasia:    inf.Emit.XFant,
		EmitCNPJ:        formatarCNPJ(inf.Emit.CNPJ),
		EmitIE:          inf.Emit.IE,
		EmitCRT:         inf.Emit.CRT,
		EmitEnd:         converterEndereco(inf.Emit.Ender),
		DestNome:        inf.Dest.XNome,
		DestCNPJ:        formatarCNPJ(inf.Dest.CNPJ),
		DestCPF:         formatarCPF(inf.Dest.CPF),
		DestIE:          inf.Dest.IE,
		DestEnd:         converterEndereco(inf.Dest.Ender),
		InfCpl:          inf.InfAdic.InfCpl,
		InfAdFisco:      inf.InfAdic.InfAdFisco,
	}

	// Totais
	d.VBC = parseFloat(inf.Total.ICMSTot.VBC)
	d.VICMS = parseFloat(inf.Total.ICMSTot.VICMS)
	d.VBCST = parseFloat(inf.Total.ICMSTot.VBCST)
	d.VST = parseFloat(inf.Total.ICMSTot.VST)
	d.VProd = parseFloat(inf.Total.ICMSTot.VProd)
	d.VFrete = parseFloat(inf.Total.ICMSTot.VFrete)
	d.VSeg = parseFloat(inf.Total.ICMSTot.VSeg)
	d.VDesc = parseFloat(inf.Total.ICMSTot.VDesc)
	d.VIPI = parseFloat(inf.Total.ICMSTot.VIPI)
	d.VPIS = parseFloat(inf.Total.ICMSTot.VPIS)
	d.VCOFINS = parseFloat(inf.Total.ICMSTot.VCOFINS)
	d.VOutro = parseFloat(inf.Total.ICMSTot.VOutro)
	d.VNF = parseFloat(inf.Total.ICMSTot.VNF)

	// Transporte
	d.ModFrete = inf.Transp.ModFrete
	d.TranspNome = inf.Transp.Transporta.XNome
	if inf.Transp.Transporta.CNPJ != "" {
		d.TranspCNPJ = formatarCNPJ(inf.Transp.Transporta.CNPJ)
	} else {
		d.TranspCNPJ = formatarCPF(inf.Transp.Transporta.CPF)
	}
	d.TranspIE = inf.Transp.Transporta.IE
	d.TranspEnd = inf.Transp.Transporta.XEnder
	d.TranspMun = inf.Transp.Transporta.XMun
	d.TranspUF = inf.Transp.Transporta.UF
	for _, v := range inf.Transp.Vol {
		d.Volumes = append(d.Volumes, volDANFE{
			Quantidade: parseFloat(v.QVol),
			Especie:    v.Esp,
			Marca:      v.Marca,
			Numeracao:  v.NVol,
			PesoBruto:  parseFloat(v.PesoB),
			PesoLiq:    parseFloat(v.PesoL),
		})
	}

	// Pagamentos
	for _, dp := range inf.Pag.DetPag {
		d.Pagamentos = append(d.Pagamentos, pagtoDANFE{
			Forma: descricaoFormaPagto(dp.TPag),
			Valor: parseFloat(dp.VPag),
		})
	}

	// Duplicatas
	for _, dup := range inf.Cobr.Dup {
		d.Duplicatas = append(d.Duplicatas, duplicataDANFE{
			Num:        dup.NDup,
			Vencimento: formatarData(dup.DVenc),
			Valor:      parseFloat(dup.VDup),
		})
	}

	// Itens
	for _, det := range inf.Det {
		num, _ := strconv.Atoi(det.NItem)
		item := itemDANFE{
			Num:     num,
			CProd:   det.Prod.CProd,
			XProd:   det.Prod.XProd,
			NCM:     det.Prod.NCM,
			CFOP:    det.Prod.CFOP,
			Unidade: det.Prod.UCom,
			Qtd:     parseFloat(det.Prod.QCom),
			VUnit:   parseFloat(det.Prod.VUnCom),
			VDesc:   parseFloat(det.Prod.VDesc),
			VProd:   parseFloat(det.Prod.VProd),
		}
		imp := det.Imposto
		if imp.ICMS.ICMS00 != nil {
			item.VBC = parseFloat(imp.ICMS.ICMS00.VBC)
			item.ICMS = parseFloat(imp.ICMS.ICMS00.VICMS)
			item.AliqICMS = parseFloat(imp.ICMS.ICMS00.PCST)
		} else if imp.ICMS.ICMS20 != nil {
			item.VBC = parseFloat(imp.ICMS.ICMS20.VBC)
			item.ICMS = parseFloat(imp.ICMS.ICMS20.VICMS)
			item.AliqICMS = parseFloat(imp.ICMS.ICMS20.PCST)
		}
		if imp.IPI != nil {
			item.IPI = parseFloat(imp.IPI.VIPI)
			item.AliqIPI = parseFloat(imp.IPI.PIPI)
		}
		d.Itens = append(d.Itens, item)
	}

	return d, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func formatarDataHora(s string) string {
	// Entrada: "2026-06-25T10:00:00-03:00"
	// Saída:   "25/06/2026 10:00"
	if len(s) < 16 {
		return s
	}
	return s[8:10] + "/" + s[5:7] + "/" + s[0:4] + " " + s[11:16]
}

func formatarData(s string) string {
	// Entrada: "2026-07-25"
	// Saída:   "25/07/2026"
	if len(s) < 10 {
		return s
	}
	return s[8:10] + "/" + s[5:7] + "/" + s[0:4]
}

func formatarCNPJ(s string) string {
	if len(s) != 14 {
		return s
	}
	return s[0:2] + "." + s[2:5] + "." + s[5:8] + "/" + s[8:12] + "-" + s[12:14]
}

func formatarCPF(s string) string {
	if len(s) != 11 {
		return s
	}
	return s[0:3] + "." + s[3:6] + "." + s[6:9] + "-" + s[9:11]
}

func converterEndereco(e struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl"`
	XBairro string `xml:"xBairro"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	Fone    string `xml:"fone"`
}) enderecoDANFE {
	return enderecoDANFE{
		Logradouro:  e.XLgr,
		Numero:      e.Nro,
		Complemento: e.XCpl,
		Bairro:      e.XBairro,
		Municipio:   e.XMun,
		UF:          e.UF,
		CEP:         formatarCEP(e.CEP),
		Fone:        e.Fone,
	}
}

func formatarCEP(s string) string {
	if len(s) != 8 {
		return s
	}
	return s[0:5] + "-" + s[5:8]
}

func descricaoFormaPagto(codigo string) string {
	switch codigo {
	case "01":
		return "Dinheiro"
	case "02":
		return "Cheque"
	case "03":
		return "Cartão de Crédito"
	case "04":
		return "Cartão de Débito"
	case "05":
		return "Crédito Loja"
	case "10":
		return "Vale Alimentação"
	case "11":
		return "Vale Refeição"
	case "12":
		return "Vale Presente"
	case "13":
		return "Vale Combustível"
	case "15":
		return "Boleto Bancário"
	case "16":
		return "Depósito Bancário"
	case "17":
		return "Pix"
	case "18":
		return "Transferência Bancária"
	case "19":
		return "Fidelidade/Cashback"
	case "90":
		return "Sem Pagamento"
	case "99":
		return "Outros"
	default:
		return "Outros"
	}
}

func descricaoModFrete(codigo string) string {
	switch codigo {
	case "0":
		return "0-Emitente"
	case "1":
		return "1-Destinatário"
	case "2":
		return "2-Terceiros"
	case "3":
		return "3-Próprio Emit."
	case "4":
		return "4-Próprio Dest."
	case "9":
		return "9-Sem Transporte"
	default:
		return codigo
	}
}

func formatarChave(chave string) string {
	// Formata como "0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000"
	if len(chave) != 44 {
		return chave
	}
	var sb strings.Builder
	for i := 0; i < 44; i += 4 {
		if i > 0 {
			sb.WriteByte(' ')
		}
		end := i + 4
		if end > 44 {
			end = 44
		}
		sb.WriteString(chave[i:end])
	}
	return sb.String()
}
