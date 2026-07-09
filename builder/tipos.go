// Package builder gera o XML da NF-e 4.0 conforme schema da SEFAZ.
package builder

import "encoding/xml"

const (
	NsNFe    = "http://www.portalfiscal.inf.br/nfe"
	VersaoNFe = "4.00"
	ModeloNFe = "55"  // NF-e
	ModeloNFCe = "65" // NFC-e
)

// ── Raiz ─────────────────────────────────────────────────────────────────────

type NFe struct {
	XMLName    xml.Name    `xml:"NFe"`
	Xmlns      string      `xml:"xmlns,attr"`
	InfNFe     InfNFe      `xml:"infNFe"`
	InfNFeSupl *InfNFeSupl `xml:"infNFeSupl,omitempty"` // NFC-e: QR Code e URL de consulta
}

// InfNFeSupl — informações suplementares presentes apenas na NFC-e (mod=65).
// Posição no XML: filho de <NFe>, após <infNFe>, antes de <Signature>.
type InfNFeSupl struct {
	QrCode   string `xml:"qrCode"`
	UrlChave string `xml:"urlChave"`
}

// ATENÇÃO: Id vem antes de Versao no struct para que encoding/xml produza
// atributos em ordem alfabética (Id < versao), conforme exige C14N 1.0.
type InfNFe struct {
	Id     string `xml:"Id,attr"`
	Versao string `xml:"versao,attr"`

	Ide      Ide          `xml:"ide"`
	Emit     Emitente     `xml:"emit"`
	Dest     *Destinatario `xml:"dest,omitempty"`
	Det      []Detalhe    `xml:"det"`
	Total    Total     `xml:"total"`
	Transp   Transporte `xml:"transp"`
	Cobr     *Cobranca  `xml:"cobr,omitempty"`
	Pag      Pagamento  `xml:"pag"`
	InfAdic  *InfAdic   `xml:"infAdic,omitempty"`
}

// ── Identificação ─────────────────────────────────────────────────────────────

type Ide struct {
	CUF      string `xml:"cUF"`      // código IBGE do estado emitente
	CNF      string `xml:"cNF"`      // 8 dígitos aleatórios
	NatOp    string `xml:"natOp"`    // natureza da operação
	Mod      string `xml:"mod"`      // 55=NF-e, 65=NFC-e
	Serie    string `xml:"serie"`    // série (000-889)
	NNF      string `xml:"nNF"`      // número da nota (1-999999999)
	DhEmi    string `xml:"dhEmi"`    // data/hora emissão ISO8601
	DhSaiEnt string `xml:"dhSaiEnt,omitempty"` // data/hora saída/entrada
	TpNF     string `xml:"tpNF"`     // 0=entrada, 1=saída
	IdDest   string `xml:"idDest"`   // 1=interna, 2=interestadual, 3=exterior
	CMunFG   string `xml:"cMunFG"`   // código IBGE do município fato gerador
	TpImp    string `xml:"tpImp"`    // 1=DANFE retrato, 2=paisagem, 5=NFC-e
	TpEmis   string `xml:"tpEmis"`   // 1=normal, 3=contingência SCAN, etc.
	CDV      string `xml:"cDV"`      // dígito verificador da chave de acesso
	TpAmb    string `xml:"tpAmb"`    // 1=produção, 2=homologação
	FinNFe   string `xml:"finNFe"`   // 1=normal, 2=complementar, 3=ajuste, 4=devolução
	IndFinal string `xml:"indFinal"` // 0=não consumidor final, 1=consumidor final
	IndPres  string `xml:"indPres"`  // 1=presencial, 2=internet, 9=outros
	IndIntermed string `xml:"indIntermed,omitempty"` // 0=sem intermediador, 1=com
	ProcEmi  string   `xml:"procEmi"`              // 0=aplicativo contrib.
	VerProc  string   `xml:"verProc"`              // versão do processo de emissão
	DhCont   string   `xml:"dhCont,omitempty"`     // data/hora entrada contingência (tpEmis≠1)
	XJust    string   `xml:"xJust,omitempty"`      // justificativa contingência ≥15 chars
	NFref    []NFref  `xml:"NFref,omitempty"`      // chaves das NF-e referenciadas (finNFe 2/4)
}

// NFref referencia a NF-e original em notas complementares (finNFe=2) e de devolução (finNFe=4).
type NFref struct {
	RefNFe string `xml:"refNFe,omitempty"` // chave de acesso 44 dígitos
}

// ── Emitente ─────────────────────────────────────────────────────────────────

type Emitente struct {
	CNPJ      string         `xml:"CNPJ"`
	XNome     string         `xml:"xNome"`
	XFant     string         `xml:"xFant,omitempty"`
	EnderEmit EnderecoEmit   `xml:"enderEmit"`
	IE        string         `xml:"IE"`
	IEST      string         `xml:"IEST,omitempty"`
	CRT       string         `xml:"CRT"` // 1=Simples, 2=Simples Excesso, 3=Normal
}

type EnderecoEmit struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	CPais   string `xml:"cPais"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone,omitempty"`
}

// ── Destinatário ─────────────────────────────────────────────────────────────

type Destinatario struct {
	CNPJ      string          `xml:"CNPJ,omitempty"`
	CPF       string          `xml:"CPF,omitempty"`
	XNome     string          `xml:"xNome"`
	EnderDest EnderecoDest    `xml:"enderDest"`
	IndIEDest string          `xml:"indIEDest"` // 1=contribuinte, 2=isento, 9=não contrib.
	IE        string          `xml:"IE,omitempty"`
	Email     string          `xml:"email,omitempty"`
}

type EnderecoDest struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP"`
	CPais   string `xml:"cPais"`
	XPais   string `xml:"xPais"`
	Fone    string `xml:"fone,omitempty"`
}

// ── Detalhe (produto + impostos) ─────────────────────────────────────────────

type Detalhe struct {
	NItem   string   `xml:"nItem,attr"`
	Prod    Produto  `xml:"prod"`
	Imposto Imposto  `xml:"imposto"`
}

type Produto struct {
	CProd      string `xml:"cProd"`
	CEAN       string `xml:"cEAN"`       // "SEM GTIN" se não houver
	XProd      string `xml:"xProd"`
	NCM        string `xml:"NCM"`
	CEST       string `xml:"CEST,omitempty"`
	CFOP       string `xml:"CFOP"`
	UCom       string `xml:"uCom"`
	QCom       string `xml:"qCom"`
	VUnCom     string `xml:"vUnCom"`
	VProd      string `xml:"vProd"`
	CEANTrib   string `xml:"cEANTrib"`   // "SEM GTIN" se não houver
	UTrib      string `xml:"uTrib"`
	QTrib      string `xml:"qTrib"`
	VUnTrib    string `xml:"vUnTrib"`
	VFrete     string `xml:"vFrete,omitempty"`
	VSeg       string `xml:"vSeg,omitempty"`
	VDesc      string `xml:"vDesc,omitempty"`
	VOutro     string `xml:"vOutro,omitempty"`
	IndTot     string `xml:"indTot"` // 1=compõe total da NF-e
	XPed       string `xml:"xPed,omitempty"`
	NItemPed   string `xml:"nItemPed,omitempty"`
}

// ── Impostos ─────────────────────────────────────────────────────────────────

type Imposto struct {
	VTotTrib string    `xml:"vTotTrib,omitempty"`
	ICMS     *ICMS     `xml:"ICMS,omitempty"`
	IPI      *IPI      `xml:"IPI,omitempty"`
	PIS      PIS       `xml:"PIS"`
	COFINS   COFINS    `xml:"COFINS"`
}

// ICMS — envelope que contém exatamente um dos grupos abaixo
type ICMS struct {
	ICMS00    *ICMS00    `xml:"ICMS00,omitempty"`
	ICMS10    *ICMS10    `xml:"ICMS10,omitempty"`
	ICMS20    *ICMS20    `xml:"ICMS20,omitempty"`
	ICMS40    *ICMS40    `xml:"ICMS40,omitempty"`
	ICMS60    *ICMS60    `xml:"ICMS60,omitempty"`
	ICMS90    *ICMS90    `xml:"ICMS90,omitempty"`
	ICMSSN101 *ICMSSN101 `xml:"ICMSSN101,omitempty"`
	ICMSSN102 *ICMSSN102 `xml:"ICMSSN102,omitempty"`
	ICMSSN201 *ICMSSN201 `xml:"ICMSSN201,omitempty"`
	ICMSSN202 *ICMSSN202 `xml:"ICMSSN202,omitempty"`
	ICMSSN500 *ICMSSN500 `xml:"ICMSSN500,omitempty"`
	ICMSSN900 *ICMSSN900 `xml:"ICMSSN900,omitempty"`
}

// Regime Normal
type ICMS00 struct {
	Orig    string `xml:"orig"`
	CST     string `xml:"CST"`     // 00=tributado integralmente
	ModBC   string `xml:"modBC"`   // 3=valor da operação
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
}

type ICMS10 struct {
	Orig     string `xml:"orig"`
	CST      string `xml:"CST"`    // 10=tributado + ST
	ModBC    string `xml:"modBC"`
	VBC      string `xml:"vBC"`
	PICMS    string `xml:"pICMS"`
	VICMS    string `xml:"vICMS"`
	ModBCST  string `xml:"modBCST"`
	PMVAST   string `xml:"pMVAST"`
	VBCST    string `xml:"vBCST"`
	PICMSST  string `xml:"pICMSST"`
	VICMSST  string `xml:"vICMSST"`
}

type ICMS20 struct {
	Orig    string `xml:"orig"`
	CST     string `xml:"CST"`    // 20=com redução de BC
	ModBC   string `xml:"modBC"`
	PRedBC  string `xml:"pRedBC"`
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
}

type ICMS40 struct {
	Orig    string `xml:"orig"`
	CST     string `xml:"CST"`    // 40=isento, 41=não tributado, 50=suspensão
	VICMSDeson  string `xml:"vICMSDeson,omitempty"`
	MotDesICMS  string `xml:"motDesICMS,omitempty"`
}

type ICMS60 struct {
	Orig        string `xml:"orig"`
	CST         string `xml:"CST"`    // 60=cobrado por ST anteriormente
	VBCSTRet    string `xml:"vBCSTRet"`
	PSTRet      string `xml:"pSTRet"`
	VICMSSTRet  string `xml:"vICMSSTRet"`
}

type ICMS90 struct {
	Orig    string `xml:"orig"`
	CST     string `xml:"CST"`    // 90=outros
	ModBC   string `xml:"modBC"`
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
}

// Simples Nacional
type ICMSSN101 struct {
	Orig       string `xml:"orig"`
	CSOSN      string `xml:"CSOSN"`    // 101=permite crédito
	PCredSN    string `xml:"pCredSN"`
	VCredICMSSN string `xml:"vCredICMSSN"`
}

type ICMSSN102 struct {
	Orig  string `xml:"orig"`
	CSOSN string `xml:"CSOSN"` // 102=sem crédito, 103, 300, 400
}

type ICMSSN201 struct {
	Orig       string `xml:"orig"`
	CSOSN      string `xml:"CSOSN"`   // 201=com ST + crédito SN
	ModBCST    string `xml:"modBCST"`
	PMVAST     string `xml:"pMVAST"`
	VBCST      string `xml:"vBCST"`
	PICMSST    string `xml:"pICMSST"`
	VICMSST    string `xml:"vICMSST"`
	PCredSN    string `xml:"pCredSN"`
	VCredICMSSN string `xml:"vCredICMSSN"`
}

type ICMSSN202 struct {
	Orig    string `xml:"orig"`
	CSOSN   string `xml:"CSOSN"`   // 202=com ST sem crédito
	ModBCST string `xml:"modBCST"`
	PMVAST  string `xml:"pMVAST"`
	VBCST   string `xml:"vBCST"`
	PICMSST string `xml:"pICMSST"`
	VICMSST string `xml:"vICMSST"`
}

type ICMSSN500 struct {
	Orig        string `xml:"orig"`
	CSOSN       string `xml:"CSOSN"`      // 500=ST anteriormente retido
	VBCSTRet    string `xml:"vBCSTRet"`
	PSTRet      string `xml:"pSTRet"`
	VICMSSTRet  string `xml:"vICMSSTRet"`
}

type ICMSSN900 struct {
	Orig    string `xml:"orig"`
	CSOSN   string `xml:"CSOSN"` // 900=outros SN
	ModBC   string `xml:"modBC"`
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
}

// IPI
type IPI struct {
	CEnq    string   `xml:"cEnq"` // código de enquadramento legal
	IPINT   *IPINT   `xml:"IPINT,omitempty"`   // não tributado
	IPITrib *IPITrib `xml:"IPITrib,omitempty"` // tributado
}

type IPINT struct {
	CST string `xml:"CST"` // 01, 02, 03, 04, 05, 51, 52, 53, 54, 55
}

type IPITrib struct {
	CST   string `xml:"CST"` // 00, 49, 50, 99
	VBC   string `xml:"vBC,omitempty"`
	PIPI  string `xml:"pIPI,omitempty"`
	QUNID string `xml:"qUnid,omitempty"`
	VUnid string `xml:"vUnid,omitempty"`
	VIPI  string `xml:"vIPI"`
}

// PIS
type PIS struct {
	PISAliq *PISAliq `xml:"PISAliq,omitempty"` // CST 01, 02
	PISNt   *PISNt   `xml:"PISNT,omitempty"`   // CST 04-09 (Simples)
	PISOutr *PISOutr `xml:"PISOutr,omitempty"` // CST 99
}

type PISAliq struct {
	CST  string `xml:"CST"`
	VBC  string `xml:"vBC"`
	PPIS string `xml:"pPIS"`
	VPIS string `xml:"vPIS"`
}

type PISNt struct {
	CST string `xml:"CST"` // 07=operação isenta, 08=sem incidência, 09=com suspensão
}

type PISOutr struct {
	CST  string `xml:"CST"`
	VBC  string `xml:"vBC,omitempty"`
	PPIS string `xml:"pPIS,omitempty"`
	QBCPROD string `xml:"qBCProd,omitempty"`
	VAliqProd string `xml:"vAliqProd,omitempty"`
	VPIS string `xml:"vPIS"`
}

// COFINS — mesma estrutura do PIS
type COFINS struct {
	COFINSAliq *COFINSAliq `xml:"COFINSAliq,omitempty"`
	COFINSNt   *COFINSNt   `xml:"COFINSNT,omitempty"`
	COFINSOutr *COFINSOutr `xml:"COFINSOutr,omitempty"`
}

type COFINSAliq struct {
	CST      string `xml:"CST"`
	VBC      string `xml:"vBC"`
	PCOFINS  string `xml:"pCOFINS"`
	VCOFINS  string `xml:"vCOFINS"`
}

type COFINSNt struct {
	CST string `xml:"CST"`
}

type COFINSOutr struct {
	CST      string `xml:"CST"`
	VBC      string `xml:"vBC,omitempty"`
	PCOFINS  string `xml:"pCOFINS,omitempty"`
	QBCPROD  string `xml:"qBCProd,omitempty"`
	VAliqProd string `xml:"vAliqProd,omitempty"`
	VCOFINS  string `xml:"vCOFINS"`
}

// ── Total ─────────────────────────────────────────────────────────────────────

type Total struct {
	ICMSTot ICMSTot `xml:"ICMSTot"`
}

type ICMSTot struct {
	VBC      string `xml:"vBC"`
	VICMS    string `xml:"vICMS"`
	VICMSDeson string `xml:"vICMSDeson"`
	VFCP     string `xml:"vFCP"`
	VBCST    string `xml:"vBCST"`
	VST      string `xml:"vST"`
	VFCPST   string `xml:"vFCPST"`
	VFCPSTRet string `xml:"vFCPSTRet"`
	VProd    string `xml:"vProd"`
	VFrete   string `xml:"vFrete"`
	VSeg     string `xml:"vSeg"`
	VDesc    string `xml:"vDesc"`
	VII      string `xml:"vII"`
	VIPI     string `xml:"vIPI"`
	VIPIDevol string `xml:"vIPIDevol"`
	VPIS     string `xml:"vPIS"`
	VCOFINS  string `xml:"vCOFINS"`
	VOutro   string `xml:"vOutro"`
	VNF      string `xml:"vNF"`
	VTotTrib string `xml:"vTotTrib"`
}

// ── Transporte ────────────────────────────────────────────────────────────────

type Transporte struct {
	ModFrete string    `xml:"modFrete"` // 0=CIF, 1=FOB, 2=terceiros, 9=sem frete
	Transp   *Transportadora `xml:"transporta,omitempty"`
	Vol      []Volume  `xml:"vol,omitempty"`
}

type Transportadora struct {
	CNPJ   string `xml:"CNPJ,omitempty"`
	CPF    string `xml:"CPF,omitempty"`
	XNome  string `xml:"xNome,omitempty"`
	IE     string `xml:"IE,omitempty"`
	XEnder string `xml:"xEnder,omitempty"`
	XMun   string `xml:"xMun,omitempty"`
	UF     string `xml:"UF,omitempty"`
}

type Volume struct {
	QVol   string `xml:"qVol,omitempty"`
	Esp    string `xml:"esp,omitempty"`
	Marca  string `xml:"marca,omitempty"`
	NVol   string `xml:"nVol,omitempty"`
	PesoL  string `xml:"pesoL,omitempty"`
	PesoB  string `xml:"pesoB,omitempty"`
}

// ── Cobrança ─────────────────────────────────────────────────────────────────

type Cobranca struct {
	Fat *Fatura    `xml:"fat,omitempty"`
	Dup []Duplicata `xml:"dup,omitempty"`
}

type Fatura struct {
	NFat  string `xml:"nFat,omitempty"`
	VOrig string `xml:"vOrig,omitempty"`
	VDesc string `xml:"vDesc,omitempty"`
	VLiq  string `xml:"vLiq"`
}

type Duplicata struct {
	NDup  string `xml:"nDup,omitempty"`
	DVenc string `xml:"dVenc,omitempty"`
	VDup  string `xml:"vDup"`
}

// ── Pagamento ─────────────────────────────────────────────────────────────────

type Pagamento struct {
	DetPag []DetalhePag `xml:"detPag"`
	VTroco string       `xml:"vTroco,omitempty"`
}

type DetalhePag struct {
	IndPag string `xml:"indPag,omitempty"` // 0=à vista, 1=a prazo
	TPag   string `xml:"tPag"`
	// 01=dinheiro, 02=cheque, 03=cartão crédito, 04=cartão débito,
	// 05=crédito loja, 10=vale alimentação, 11=vale refeição,
	// 12=vale presente, 13=vale combustível, 15=boleto, 90=sem pagamento, 99=outros
	XPag string `xml:"xPag,omitempty"` // obrigatório quando tPag=99
	VPag string `xml:"vPag"`
}

// ── Informações Adicionais ────────────────────────────────────────────────────

type InfAdic struct {
	InfAdFisco string `xml:"infAdFisco,omitempty"` // informações para o fisco
	InfCpl     string `xml:"infCpl,omitempty"`     // informações complementares
}
