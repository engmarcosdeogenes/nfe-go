package cte

import "time"

// Entrada é o que o chamador preenche. Não expõe structs do schema.
// Valores monetários em reais (float64); o builder arredonda em 2 casas.
type Entrada struct {
	TpAmb  string // "1" produção, "2" homologação
	Serie  string // 0-999
	NCT    string // 1-999999999
	DhEmi  time.Time
	CFOP   string // TCfop
	NatOp  string // 1-60
	TpImp  string // vazio vira "1" (retrato); "2" paisagem
	TpCTe  string // "0" normal, "1" complemento, "3" substituição (2 extinto)
	TpServ string // "0" normal, "1" subcontratação, "2" redespacho, "3" redespacho intermediário, "4" vinculado a multimodal
	Modal  string // vazio vira "01"; só "01" é montado

	// TpEmis: "1" normal (vazio vira "1"), "2" contingência offline, "3" regime
	// especial NFF (usa InfSolicNFF), "4" EPEC (em desativação), "7" SVC-RS,
	// "8" SVC-SP. Diferente de "1": DhCont e XJust (15-256) obrigatórios.
	// A NT 2026.004 (fim de EPEC/FS-DA, emissão offline) não mudou o enum do
	// XSD; qual valor é a "emissão offline" nova: não confirmado.
	TpEmis string
	DhCont *time.Time
	XJust  string
	// QRSign: assinatura RSA-SHA1 da chave (base64), calculada pelo chamador.
	// Obrigatória com TpEmis "2" (parâmetro &sign= do QR Code).
	QRSign string
	// URLQRCode: base do QR Code do autorizador da UF (ex.: SVRS
	// "https://dfe-portal.svrs.rs.gov.br/cte/qrCode"). Vazio usa SVRS.
	URLQRCode string
	VerProc   string // vazio vira "nfe-go"

	IndGlobalizado bool // ide/indGlobalizado=1; exige InfGlobal
	Envio          Local
	Inicio, Fim    Local
	Retira         bool
	XDetRetira     string
	// IndIEToma: "1" contribuinte com IE, "2" isento, "9" não contribuinte.
	IndIEToma string
	// Tomador: Toma3 "0" remetente, "1" expedidor, "2" recebedor, "3"
	// destinatário (o participante correspondente deve existir) OU Toma4
	// (outro tomador, dados completos). Exatamente um dos dois.
	Toma3 string
	Toma4 *Terceiro

	CompraGov     *CompraGov // ide/gCompraGov (NT RTC)
	TpPagAnt      string     // "1" ou "3"
	PagAntecipado []string   // chDFePagAnt, 1..99

	Compl *Compl

	Emitente Emitente
	Rem      *Parte
	Exped    *Parte
	Receb    *Parte
	Dest     *Parte

	VTPrest    float64
	VTPrestLiq *float64 // NT 2026.004 (valor líquido)
	VRec       float64
	Comp       []Comp

	ICMS       ICMS
	VTotTrib   *float64
	InfAdFisco string
	ICMSUFFim  *ICMSUFFim
	IBSCBS     *IBSCBS
	VTotDFe    *float64
	PgtoVinc   []PagVinc // NT 2026.001, 0..99

	// infCTeNorm (tpCTe 0 e 3); InfCteComp (tpCTe 1, 1..10 chaves).
	Carga       *Carga
	Docs        Docs
	DocAnt      []EmiDocAnt
	Rodo        *Rodoviario
	Aereo       *Aereo
	Aquaviario  *Aquaviario
	Ferroviario *Ferroviario
	Dutoviario  *Dutoviario
	Multimodal  *Multimodal
	VeicNovos   []VeicNovo
	Cobr        *Cobr
	InfCteSub   *InfCteSub
	InfGlobal   string   // infGlobalizado/xObs (15-256)
	InfServVinc []string // chCTeMultimodal (tpServ 4)
	InfCteComp  []string

	AutXML      []string
	RespTec     *RespTec
	InfSolicNFF string
}

type Local struct{ CodigoMun, Nome, UF string }

type Endereco struct {
	Logradouro, Numero, Complemento, Bairro string
	CodigoMun, Municipio, CEP, UF           string
	CodPais, NomePais                       string // exterior (UF "EX")
	Fone                                    string // só enderEmit tem fone
}

type Emitente struct {
	CNPJ, CPF, IE, IEST, Nome, Fantasia string
	End                                 Endereco
	CRT                                 string // 1 SN, 2 SN excesso, 3 normal, 4 MEI
	ISUF                                string
}

// Parte é remetente, expedidor, recebedor ou destinatário. UF "EX" sem
// CNPJ/CPF: o XSD aceita CNPJ vazio para exterior (não existe idEstrangeiro
// no CT-e 4.00).
type Parte struct {
	CNPJ, CPF, IE, Nome string
	Fantasia            string // só rem
	Fone, Email         string
	ISUF                string // só dest
	End                 Endereco
}

// Terceiro é o tomador toma4 (tomador "outros").
type Terceiro struct {
	CNPJ, CPF, IE, Nome, Fantasia, Fone, Email string
	End                                        Endereco
}

type Compl struct {
	XCaracAd, XCaracSer, XEmi string
	XOrig, XDest, XRota       string
	Pass                      []string
	Entrega                   *Entrega
	OrigCalc, DestCalc, XObs  string
	ObsCont, ObsFisco         []Obs
}

type Obs struct{ Campo, Texto string }

// Entrega: TpPer "0" sem data, "1" na data, "2" até a data, "3" a partir da
// data, "4" no período; TpHor idem para hora.
type Entrega struct {
	TpPer      string
	DProg      string // AAAA-MM-DD
	DIni, DFim string
	TpHor      string
	HProg      string // HH:MM:SS
	HIni, HFim string
}

type CompraGov struct {
	TpEnteGov, TpOperGov string
	PRedutor             float64
	RefDFeAnt            []string
}

type Comp struct {
	Nome  string
	Valor float64
}

// ICMS escolhe o grupo pelo CST: 00, 20, 40/41/51 (ICMS45), 60, 90; OutraUF
// com CST 90 usa ICMSOutraUF; SN usa ICMSSN (CST 90 + indSN=1). Previsto
// (sem CST) emite gICMSPrevistoPagtoAntecip (NT 2026.004).
type ICMS struct {
	CST        string
	OutraUF    bool
	SN         bool
	PRedBC     float64
	VBC        float64
	PICMS      float64
	VICMS      float64
	VCred      *float64
	VBCSTRet   float64
	VICMSSTRet float64
	PICMSSTRet float64
	VICMSDeson float64 // sempre com CBenef (8 ou 10 caracteres)
	CBenef     string
	Previsto   *float64
}

type ICMSUFFim struct {
	VBCUFFim, PFCPUFFim, PICMSUFFim, PICMSInter float64
	VFCPUFFim, VICMSUFFim, VICMSUFIni           float64
}

// IBSCBS é o grupo imp/IBSCBS (TTribCTe, NT 2025.001 RTC).
type IBSCBS struct {
	CST        string // 3 dígitos
	CClassTrib string // 6 dígitos
	IndDoacao  bool
	G          *GIBSCBS
	EstornoIBS *float64 // gEstornoCred
	EstornoCBS *float64
}

type GIBSCBS struct {
	VBC     float64
	IBSUF   AliqRTC
	IBSMun  AliqRTC
	VIBS    float64
	CBS     AliqRTC
	ALCZFM  *ALCZFMCBS
	Regular *TribRegular
	CompraG *TribCompraGov
}

// AliqRTC: P alíquota nominal, V valor; Dif, DevTrib e Red opcionais.
type AliqRTC struct {
	P, V                float64
	PDif                *float64 // gDif (com VDif)
	VDif                float64
	PDev                *float64 // gDevTrib (com VDev)
	VDev                float64
	PRedAliq, PAliqEfet *float64 // gRed (os dois juntos)
}

type ALCZFMCBS struct{ PAliqEfetRegCBS, VTribRegCBS float64 }

type TribRegular struct {
	CST, CClassTrib                              string
	PIBSUF, VIBSUF, PIBSMun, VIBSMun, PCBS, VCBS float64
}

type TribCompraGov struct {
	PIBSUF, VIBSUF, PIBSMun, VIBSMun, PCBS, VCBS float64
}

type PagVinc struct {
	NPag, IDTransacao, TpMeioPgto, CNPJReceb, CNPJBasePSP string
}

type Carga struct {
	VCarga      *float64
	ProPred     string
	XOutCat     string
	Quantidades []QCarga
	VCargaAverb *float64
}

type QCarga struct {
	CUnid string // 00 M3, 01 KG, 02 TON, 03 UNIDADE, 04 LITROS, 05 MMBTU
	TpMed string
	Qtd   float64
}

// Docs: apenas UM dos quatro grupos pode vir preenchido (choice do XSD).
type Docs struct {
	NFe    []DocNFe
	NF     []DocNF
	Outros []DocOutros
	DCe    []string
}

type DocNFe struct {
	Chave, PIN, DPrev string
	UnidCarga         []UnidCarga
	UnidTransp        []UnidTransp
}

// DocNF é nota de papel (modelos 01 e 04).
type DocNF struct {
	NRoma, NPed, Mod, Serie, NDoc, DEmi string
	VBC, VICMS, VBCST, VST, VProd, VNF  float64
	NCFOP                               string
	NPeso                               float64
	PIN, DPrev                          string
	UnidCarga                           []UnidCarga
	UnidTransp                          []UnidTransp
}

type DocOutros struct {
	TpDoc, Desc, NDoc, DEmi, DPrev string // TpDoc 00, 10, 59, 65, 99
	VDocFisc                       float64
	UnidCarga                      []UnidCarga
	UnidTransp                     []UnidTransp
}

// UnidCarga/UnidTransp: em cada documento só um dos dois (choice do XSD).
type UnidCarga struct {
	Tp     string // 1 container, 2 ULD, 3 pallet, 4 outros
	ID     string
	Lacres []string
	QtdRat float64
}

type UnidTransp struct {
	Tp     string // 1..7
	ID     string
	Lacres []string
	Carga  []UnidCarga
	QtdRat float64
}

// EmiDocAnt: documentos de transporte anteriores (tpServ 1, 2, 3).
type EmiDocAnt struct {
	CNPJ, CPF, IE, UF, Nome string
	Papel                   []DocAntPap
	Eletronico              []string // chCTe
}

type DocAntPap struct{ TpDoc, Serie, Subser, NDoc, DEmi string }

type Rodoviario struct {
	RNTRC string // 8 dígitos ou "ISENTO"
	OCC   []OCC
}

type OCC struct {
	Serie, NOcc, DEmi        string
	CNPJ, CInt, IE, UF, Fone string
}

// Modelo de dados dos demais modais (Onda 4): Build recusa.
type Aereo struct{ NMinu, NOCA, DPrevAereo string }
type Aquaviario struct {
	VPrest, VAFRMM float64
	XNavio         string
}
type Ferroviario struct{ TpTraf, Fluxo string }
type Dutoviario struct {
	VTar       float64
	DIni, DFim string
}
type Multimodal struct{ COTM, IndNegociavel string }

type VeicNovo struct {
	Chassi, CCor, XCor, CMod string
	VUnit, VFrete            float64
}

type Cobr struct {
	NFat               string
	VOrig, VDesc, VLiq *float64
	Dup                []Dup
}

type Dup struct {
	NDup, DVenc string
	VDup        *float64
}

type InfCteSub struct {
	ChCte         string
	IndAlteraToma bool
}

type RespTec struct{ CNPJ, Contato, Email, Fone string }
