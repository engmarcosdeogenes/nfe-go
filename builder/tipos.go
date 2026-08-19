// Package builder gera o XML da NF-e 4.0 conforme schema da SEFAZ.
package builder

import "encoding/xml"

const (
	NsNFe      = "http://www.portalfiscal.inf.br/nfe"
	VersaoNFe  = "4.00"
	ModeloNFe  = "55" // NF-e
	ModeloNFCe = "65" // NFC-e

	// XNomeDestHomologacao é o texto exigido no xNome do destinatário quando
	// tpAmb=2. Aplicado automaticamente pelo Build.
	XNomeDestHomologacao = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
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

	Ide        Ide           `xml:"ide"`
	Emit       Emitente      `xml:"emit"`
	Dest       *Destinatario `xml:"dest,omitempty"`
	AutXML     []AutXML      `xml:"autXML,omitempty"`
	Det        []Detalhe     `xml:"det"`
	Total      Total         `xml:"total"`
	Transp     Transporte    `xml:"transp"`
	Cobr       *Cobranca     `xml:"cobr,omitempty"`
	Pag        Pagamento     `xml:"pag"`
	InfAdic    *InfAdic      `xml:"infAdic,omitempty"`
	InfRespTec *InfRespTec   `xml:"infRespTec,omitempty"`
}

// AutXML autoriza terceiro (contador, transportadora) a baixar o XML da nota
// pelo portal da SEFAZ, sem precisar da chave de acesso.
type AutXML struct {
	CNPJ string `xml:"CNPJ,omitempty"`
	CPF  string `xml:"CPF,omitempty"`
}

// InfRespTec identifica o responsável técnico pelo sistema emissor —
// grupo opcional no schema (NT 2015/002), mas algumas SEFAZ estaduais
// tratam como obrigatório na prática (regra de negócio, não XSD).
type InfRespTec struct {
	CNPJ     string `xml:"CNPJ"`
	XContato string `xml:"xContato"`
	Email    string `xml:"email"`
	Fone     string `xml:"fone"`
}

// ── Identificação ─────────────────────────────────────────────────────────────

type Ide struct {
	CUF         string  `xml:"cUF"`                   // código IBGE do estado emitente
	CNF         string  `xml:"cNF"`                   // 8 dígitos aleatórios
	NatOp       string  `xml:"natOp"`                 // natureza da operação
	Mod         string  `xml:"mod"`                   // 55=NF-e, 65=NFC-e
	Serie       string  `xml:"serie"`                 // série (000-889)
	NNF         string  `xml:"nNF"`                   // número da nota (1-999999999)
	DhEmi       string  `xml:"dhEmi"`                 // data/hora emissão ISO8601
	DhSaiEnt    string  `xml:"dhSaiEnt,omitempty"`    // data/hora saída/entrada
	TpNF        string  `xml:"tpNF"`                  // 0=entrada, 1=saída
	IdDest      string  `xml:"idDest"`                // 1=interna, 2=interestadual, 3=exterior
	CMunFG      string  `xml:"cMunFG"`                // código IBGE do município fato gerador
	TpImp       string  `xml:"tpImp"`                 // 1=DANFE retrato, 2=paisagem, 5=NFC-e
	TpEmis      string  `xml:"tpEmis"`                // 1=normal, 3=contingência SCAN, etc.
	CDV         string  `xml:"cDV"`                   // dígito verificador da chave de acesso
	TpAmb       string  `xml:"tpAmb"`                 // 1=produção, 2=homologação
	FinNFe      string  `xml:"finNFe"`                // 1=normal, 2=complementar, 3=ajuste, 4=devolução
	IndFinal    string  `xml:"indFinal"`              // 0=não consumidor final, 1=consumidor final
	IndPres     string  `xml:"indPres"`               // 1=presencial, 2=internet, 9=outros
	IndIntermed string  `xml:"indIntermed,omitempty"` // 0=sem intermediador, 1=com
	ProcEmi     string  `xml:"procEmi"`               // 0=aplicativo contrib.
	VerProc     string  `xml:"verProc"`               // versão do processo de emissão
	DhCont      string  `xml:"dhCont,omitempty"`      // data/hora entrada contingência (tpEmis≠1)
	XJust       string  `xml:"xJust,omitempty"`       // justificativa contingência ≥15 chars
	NFref       []NFref `xml:"NFref,omitempty"`       // chaves das NF-e referenciadas (finNFe 2/4)
}

// NFref referencia a NF-e original em notas complementares (finNFe=2) e de devolução (finNFe=4).
type NFref struct {
	RefNFe string `xml:"refNFe,omitempty"` // chave de acesso 44 dígitos
}

// ── Emitente ─────────────────────────────────────────────────────────────────

type Emitente struct {
	CNPJ      string       `xml:"CNPJ"`
	XNome     string       `xml:"xNome"`
	XFant     string       `xml:"xFant,omitempty"`
	EnderEmit EnderecoEmit `xml:"enderEmit"`
	IE        string       `xml:"IE"` // obrigatório pelo schema (confirmado via xmllint contra nfe_v4.00.xsd)
	IEST      string       `xml:"IEST,omitempty"`
	CRT       string       `xml:"CRT"` // 1=Simples, 2=Simples Excesso, 3=Normal
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
	CNPJ  string `xml:"CNPJ,omitempty"`
	CPF   string `xml:"CPF,omitempty"`
	XNome string `xml:"xNome"`
	// EnderDest é minOccurs=0 no schema: ponteiro pra poder sair fora do XML
	// quando não há endereço. Emitir o grupo vazio viola TEndereco, cujos
	// filhos são obrigatórios uma vez que o grupo existe.
	EnderDest *EnderecoDest `xml:"enderDest,omitempty"`
	IndIEDest string        `xml:"indIEDest"` // 1=contribuinte, 2=isento, 9=não contrib.
	IE        string        `xml:"IE,omitempty"`
	Email     string        `xml:"email,omitempty"`
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
	NItem   string  `xml:"nItem,attr"`
	Prod    Produto `xml:"prod"`
	Imposto Imposto `xml:"imposto"`
}

type Produto struct {
	CProd    string `xml:"cProd"`
	CEAN     string `xml:"cEAN"` // "SEM GTIN" se não houver
	XProd    string `xml:"xProd"`
	NCM      string `xml:"NCM"`
	CEST     string `xml:"CEST,omitempty"`
	CBenef   string `xml:"cBenef,omitempty"` // código de benefício fiscal (Convênio ICMS 190/17), tabela por UF
	CFOP     string `xml:"CFOP"`
	UCom     string `xml:"uCom"`
	QCom     string `xml:"qCom"`
	VUnCom   string `xml:"vUnCom"`
	VProd    string `xml:"vProd"`
	CEANTrib string `xml:"cEANTrib"` // "SEM GTIN" se não houver
	UTrib    string `xml:"uTrib"`
	QTrib    string `xml:"qTrib"`
	VUnTrib  string `xml:"vUnTrib"`
	VFrete   string `xml:"vFrete,omitempty"`
	VSeg     string `xml:"vSeg,omitempty"`
	VDesc    string `xml:"vDesc,omitempty"`
	VOutro   string `xml:"vOutro,omitempty"`
	IndTot   string `xml:"indTot"` // 1=compõe total da NF-e
	XPed     string `xml:"xPed,omitempty"`
	NItemPed string `xml:"nItemPed,omitempty"`
}

// ── Impostos ─────────────────────────────────────────────────────────────────

type Imposto struct {
	VTotTrib   string      `xml:"vTotTrib,omitempty"`
	ICMS       *ICMS       `xml:"ICMS,omitempty"`
	IPI        *IPI        `xml:"IPI,omitempty"`
	PIS        PIS         `xml:"PIS"`
	COFINS     COFINS      `xml:"COFINS"`
	ICMSUFDest *ICMSUFDest `xml:"ICMSUFDest,omitempty"`
	IBSCBS     *IBSCBS     `xml:"IBSCBS,omitempty"`
}

// ICMSUFDest é o DIFAL (EC 87/2015, NT 2015.003) — só se aplica a venda
// interestadual pra consumidor final não contribuinte (IndIEDest=9); venda
// interestadual pra contribuinte revendedor não entra aqui. PICMSInterPart
// fixo em "100" porque a partilha gradual (2016-2018) acabou em 2019 -- hoje
// 100% do diferencial vai pro Estado de destino.
type ICMSUFDest struct {
	VBCUFDest      string `xml:"vBCUFDest"`
	VBCFCPUFDest   string `xml:"vBCFCPUFDest,omitempty"` // vazio quando o UF de destino não tem FCP pro produto
	PFCPUFDest     string `xml:"pFCPUFDest,omitempty"`
	PICMSUFDest    string `xml:"pICMSUFDest"`
	PICMSInter     string `xml:"pICMSInter"` // alíquota interestadual (4/7/12%, tabela CONFAZ)
	PICMSInterPart string `xml:"pICMSInterPart"`
	VFCPUFDest     string `xml:"vFCPUFDest,omitempty"`
	VICMSUFDest    string `xml:"vICMSUFDest"`
	VICMSUFRemet   string `xml:"vICMSUFRemet"`
}

// IBSCBS — grupo UB12/UB15 da NT 2025.002-RTC (reforma tributária, LC
// 214/2025). Cobre só a tributação regular por item (sem monofasia, redução
// de alíquota, diferimento, crédito presumido ou compra governamental --
// esses ficam nos subgrupos opcionais gDif/gRed/gDevTrib/gTribRegular/
// gTribCompraGov/gIBSCBSMono da mesma NT, ainda não implementados).
type IBSCBS struct {
	CST       string  `xml:"CST"`        // tabela CST do IBS/CBS (Portal NF-e > Documentos > Diversos)
	ClassTrib string  `xml:"cClassTrib"` // tabela cClassTrib, Anexo III da NT
	GIBSCBS   GIBSCBS `xml:"gIBSCBS"`
}

type GIBSCBS struct {
	VBC     string  `xml:"vBC"`
	GIBSUF  GIBSUF  `xml:"gIBSUF"`
	GIBSMun GIBSMun `xml:"gIBSMun"`
	VIBS    string  `xml:"vIBS"` // soma de vIBSUF + vIBSMun
	GCBS    GCBS    `xml:"gCBS"`
}

type GIBSUF struct {
	PIBSUF string `xml:"pIBSUF"`
	VIBSUF string `xml:"vIBSUF"`
}

type GIBSMun struct {
	PIBSMun string `xml:"pIBSMun"`
	VIBSMun string `xml:"vIBSMun"`
}

type GCBS struct {
	PCBS string `xml:"pCBS"`
	VCBS string `xml:"vCBS"`
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
	Orig  string `xml:"orig"`
	CST   string `xml:"CST"`   // 00=tributado integralmente
	ModBC string `xml:"modBC"` // 3=valor da operação
	VBC   string `xml:"vBC"`
	PICMS string `xml:"pICMS"`
	VICMS string `xml:"vICMS"`
}

type ICMS10 struct {
	Orig    string `xml:"orig"`
	CST     string `xml:"CST"` // 10=tributado + ST
	ModBC   string `xml:"modBC"`
	VBC     string `xml:"vBC"`
	PICMS   string `xml:"pICMS"`
	VICMS   string `xml:"vICMS"`
	ModBCST string `xml:"modBCST"`
	PMVAST  string `xml:"pMVAST"`
	VBCST   string `xml:"vBCST"`
	PICMSST string `xml:"pICMSST"`
	VICMSST string `xml:"vICMSST"`
}

type ICMS20 struct {
	Orig   string `xml:"orig"`
	CST    string `xml:"CST"` // 20=com redução de BC
	ModBC  string `xml:"modBC"`
	PRedBC string `xml:"pRedBC"`
	VBC    string `xml:"vBC"`
	PICMS  string `xml:"pICMS"`
	VICMS  string `xml:"vICMS"`
}

type ICMS40 struct {
	Orig       string `xml:"orig"`
	CST        string `xml:"CST"` // 40=isento, 41=não tributado, 50=suspensão
	VICMSDeson string `xml:"vICMSDeson,omitempty"`
	MotDesICMS string `xml:"motDesICMS,omitempty"`
}

type ICMS60 struct {
	Orig       string `xml:"orig"`
	CST        string `xml:"CST"` // 60=cobrado por ST anteriormente
	VBCSTRet   string `xml:"vBCSTRet"`
	PSTRet     string `xml:"pSTRet"`
	VICMSSTRet string `xml:"vICMSSTRet"`
}

type ICMS90 struct {
	Orig  string `xml:"orig"`
	CST   string `xml:"CST"` // 90=outros
	ModBC string `xml:"modBC"`
	VBC   string `xml:"vBC"`
	PICMS string `xml:"pICMS"`
	VICMS string `xml:"vICMS"`
}

// Simples Nacional
type ICMSSN101 struct {
	Orig        string `xml:"orig"`
	CSOSN       string `xml:"CSOSN"` // 101=permite crédito
	PCredSN     string `xml:"pCredSN"`
	VCredICMSSN string `xml:"vCredICMSSN"`
}

type ICMSSN102 struct {
	Orig  string `xml:"orig"`
	CSOSN string `xml:"CSOSN"` // 102=sem crédito, 103, 300, 400
}

type ICMSSN201 struct {
	Orig        string `xml:"orig"`
	CSOSN       string `xml:"CSOSN"` // 201=com ST + crédito SN
	ModBCST     string `xml:"modBCST"`
	PMVAST      string `xml:"pMVAST"`
	VBCST       string `xml:"vBCST"`
	PICMSST     string `xml:"pICMSST"`
	VICMSST     string `xml:"vICMSST"`
	PCredSN     string `xml:"pCredSN"`
	VCredICMSSN string `xml:"vCredICMSSN"`
}

type ICMSSN202 struct {
	Orig    string `xml:"orig"`
	CSOSN   string `xml:"CSOSN"` // 202=com ST sem crédito
	ModBCST string `xml:"modBCST"`
	PMVAST  string `xml:"pMVAST"`
	VBCST   string `xml:"vBCST"`
	PICMSST string `xml:"pICMSST"`
	VICMSST string `xml:"vICMSST"`
}

type ICMSSN500 struct {
	Orig       string `xml:"orig"`
	CSOSN      string `xml:"CSOSN"` // 500=ST anteriormente retido
	VBCSTRet   string `xml:"vBCSTRet"`
	PSTRet     string `xml:"pSTRet"`
	VICMSSTRet string `xml:"vICMSSTRet"`
}

type ICMSSN900 struct {
	Orig  string `xml:"orig"`
	CSOSN string `xml:"CSOSN"` // 900=outros SN
	ModBC string `xml:"modBC"`
	VBC   string `xml:"vBC"`
	PICMS string `xml:"pICMS"`
	VICMS string `xml:"vICMS"`
}

// IPI
type IPI struct {
	CEnq    string   `xml:"cEnq"`              // código de enquadramento legal
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
	CST       string `xml:"CST"`
	VBC       string `xml:"vBC,omitempty"`
	PPIS      string `xml:"pPIS,omitempty"`
	QBCPROD   string `xml:"qBCProd,omitempty"`
	VAliqProd string `xml:"vAliqProd,omitempty"`
	VPIS      string `xml:"vPIS"`
}

// COFINS — mesma estrutura do PIS
type COFINS struct {
	COFINSAliq *COFINSAliq `xml:"COFINSAliq,omitempty"`
	COFINSNt   *COFINSNt   `xml:"COFINSNT,omitempty"`
	COFINSOutr *COFINSOutr `xml:"COFINSOutr,omitempty"`
}

type COFINSAliq struct {
	CST     string `xml:"CST"`
	VBC     string `xml:"vBC"`
	PCOFINS string `xml:"pCOFINS"`
	VCOFINS string `xml:"vCOFINS"`
}

type COFINSNt struct {
	CST string `xml:"CST"`
}

type COFINSOutr struct {
	CST       string `xml:"CST"`
	VBC       string `xml:"vBC,omitempty"`
	PCOFINS   string `xml:"pCOFINS,omitempty"`
	QBCPROD   string `xml:"qBCProd,omitempty"`
	VAliqProd string `xml:"vAliqProd,omitempty"`
	VCOFINS   string `xml:"vCOFINS"`
}

// ── Total ─────────────────────────────────────────────────────────────────────

type Total struct {
	ICMSTot   ICMSTot    `xml:"ICMSTot"`
	IBSCBSTot *IBSCBSTot `xml:"IBSCBSTot,omitempty"`
}

// IBSCBSTot — grupo W03/W34 da NT 2025.002-RTC: somatório dos itens com
// IBSCBS. O elemento <IBSCBSTot> do leiaute usa o tipo TIBSCBSMonoTot
// (leiauteNFe_v4.00.xsd), não TIBSCBSTot -- este cobre também o mono/gMono e
// exige vDif/vDevTrib/vCredPres/vCredPresCondSus mesmo sem os grupos
// correspondentes implementados nos itens, por isso sempre "0.00" aqui,
// nunca omitidos. gMono e gEstornoCred são opcionais (minOccurs=0) e ficam
// de fora -- não implementamos monofasia nem estorno de crédito.
type IBSCBSTot struct {
	VBCIBSCBS string       `xml:"vBCIBSCBS"`
	GIBS      GIBSTotal    `xml:"gIBS"`
	GCBS      GCBSTotalIBS `xml:"gCBS"`
}

type GIBSTotal struct {
	GIBSUF           GIBSUFTotal  `xml:"gIBSUF"`
	GIBSMun          GIBSMunTotal `xml:"gIBSMun"`
	VIBS             string       `xml:"vIBS"`
	VCredPres        string       `xml:"vCredPres"`
	VCredPresCondSus string       `xml:"vCredPresCondSus"`
}

type GIBSUFTotal struct {
	VDif     string `xml:"vDif"`
	VDevTrib string `xml:"vDevTrib"`
	VIBSUF   string `xml:"vIBSUF"`
}

type GIBSMunTotal struct {
	VDif     string `xml:"vDif"`
	VDevTrib string `xml:"vDevTrib"`
	VIBSMun  string `xml:"vIBSMun"`
}

type GCBSTotalIBS struct {
	VDif             string `xml:"vDif"`
	VDevTrib         string `xml:"vDevTrib"`
	VCBS             string `xml:"vCBS"`
	VCredPres        string `xml:"vCredPres"`
	VCredPresCondSus string `xml:"vCredPresCondSus"`
}

type ICMSTot struct {
	VBC          string `xml:"vBC"`
	VICMS        string `xml:"vICMS"`
	VICMSDeson   string `xml:"vICMSDeson"`
	VFCPUFDest   string `xml:"vFCPUFDest,omitempty"`
	VICMSUFDest  string `xml:"vICMSUFDest,omitempty"`
	VICMSUFRemet string `xml:"vICMSUFRemet,omitempty"`
	VFCP         string `xml:"vFCP"`
	VBCST        string `xml:"vBCST"`
	VST          string `xml:"vST"`
	VFCPST       string `xml:"vFCPST"`
	VFCPSTRet    string `xml:"vFCPSTRet"`
	VProd        string `xml:"vProd"`
	VFrete       string `xml:"vFrete"`
	VSeg         string `xml:"vSeg"`
	VDesc        string `xml:"vDesc"`
	VII          string `xml:"vII"`
	VIPI         string `xml:"vIPI"`
	VIPIDevol    string `xml:"vIPIDevol"`
	VPIS         string `xml:"vPIS"`
	VCOFINS      string `xml:"vCOFINS"`
	VOutro       string `xml:"vOutro"`
	VNF          string `xml:"vNF"`
	VTotTrib     string `xml:"vTotTrib"`
}

// ── Transporte ────────────────────────────────────────────────────────────────

type Transporte struct {
	ModFrete string          `xml:"modFrete"` // 0=CIF, 1=FOB, 2=terceiros, 9=sem frete
	Transp   *Transportadora `xml:"transporta,omitempty"`
	Vol      []Volume        `xml:"vol,omitempty"`
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
	QVol  string `xml:"qVol,omitempty"`
	Esp   string `xml:"esp,omitempty"`
	Marca string `xml:"marca,omitempty"`
	NVol  string `xml:"nVol,omitempty"`
	PesoL string `xml:"pesoL,omitempty"`
	PesoB string `xml:"pesoB,omitempty"`
}

// ── Cobrança ─────────────────────────────────────────────────────────────────

type Cobranca struct {
	Fat *Fatura     `xml:"fat,omitempty"`
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
	Card *Card  `xml:"card,omitempty"` // grupo do cartão -- presente quando tPag=03 ou 04
}

// Card é o grupo de pagamento por cartão (detPag/card) -- tpIntegra é o único
// filho obrigatório do grupo em si (1=integrado com o emissor via TEF/POS,
// 2=não integrado), o resto é opcional no schema.
type Card struct {
	TpIntegra string `xml:"tpIntegra"`
	CNPJ      string `xml:"CNPJ,omitempty"` // credenciadora da bandeira
	TBand     string `xml:"tBand,omitempty"`
	CAut      string `xml:"cAut,omitempty"`
	CNPJReceb string `xml:"CNPJReceb,omitempty"`
	IdTermPag string `xml:"idTermPag,omitempty"`
}

// ── Informações Adicionais ────────────────────────────────────────────────────

type InfAdic struct {
	InfAdFisco string `xml:"infAdFisco,omitempty"` // informações para o fisco
	InfCpl     string `xml:"infCpl,omitempty"`     // informações complementares
}
