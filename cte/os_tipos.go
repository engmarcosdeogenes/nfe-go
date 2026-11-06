package cte

import "time"

// EntradaOS é o que o chamador preenche para o CT-e OS (modelo 67, leiaute
// 4.00, TCTeOS de cteOS_v4.00.xsd): transporte de pessoas (TpServ "6"), de
// valores ("7") e excesso de bagagem ("8"). Reaproveita Emitente, Terceiro,
// Local, Compl, Comp, ICMS, IBSCBS, Cobr etc. do CT-e 57.
type EntradaOS struct {
	TpAmb  string // "1" produção, "2" homologação
	Serie  string // 0-999 ("0" = série única)
	NCT    string // 1-999999999
	DhEmi  time.Time
	CFOP   string
	NatOp  string // 1-60
	TpImp  string // vazio vira "1"
	TpCTe  string // "0" normal, "1" complemento, "3" substituição
	TpServ string // "6" pessoas, "7" valores, "8" excesso de bagagem
	Modal  string // vazio vira "01"; só "01" é montado (02 aéreo, 03 aquaviário, 04 ferroviário: só no enum do XSD)

	// TpEmis: "1" normal (vazio vira "1"), "2" contingência offline, "7" SVC-RS,
	// "8" SVC-SP (enum do TCTeOS; 3 e 4 não existem no CT-e OS). Diferente de
	// "1": DhCont e XJust (15-256).
	TpEmis    string
	DhCont    *time.Time
	XJust     string
	QRSign    string // obrigatório com TpEmis "2"
	CCT       string // cCT (8 dígitos) para obter a chave antes do Build
	URLQRCode string // vazio usa SVRS
	VerProc   string // vazio vira "nfe-go"

	Envio Local
	// Início e fim da prestação são opcionais no TCTeOS; vêm os dois ou nenhum.
	Inicio, Fim *Local
	Percurso    []string // infPercurso/UFPer, até 25 (sem repetir UF de início e fim)
	IndIEToma   string   // "1", "2" ou "9"

	// Toma é o tomador/usuário do serviço (grupo toma). Obrigatório em
	// passageiros e valores; opcional no excesso de bagagem (documentação do XSD).
	Toma *Terceiro

	CompraGov     *CompraGov
	TpPagAnt      string
	PagAntecipado []string

	Compl *Compl // só xCaracAd, xCaracSer, xEmi, xObs, ObsCont e ObsFisco existem no TCTeOS

	Emitente Emitente // CNPJ obrigatório (TCnpj) e IE numérica

	VTPrest    float64
	VTPrestLiq *float64
	VRec       float64
	Comp       []Comp

	ICMS       ICMS // sem CST 60: TImpOS não tem ICMS60
	VTotTrib   *float64
	InfAdFisco string
	TribFed    *TribFed // infTribFed: tributos federais retidos
	ICMSUFFim  *ICMSUFFim
	IBSCBS     *IBSCBS
	VTotDFe    *float64
	PgtoVinc   []PagVinc

	// infCTeNorm (tpCTe 0 e 3) ou InfCteComp (tpCTe 1, 1..10 chaves).
	Servico    *InfServicoOS
	DocRef     []DocRefOS
	Seg        []SegOS
	RodoOS     *RodoviarioOS
	InfCteSub  string // chCte do CT-e OS substituído (tpCTe 3)
	RefCTeCanc string // CT-e cancelado; só em transporte de valores (TpServ 7)
	Cobr       *Cobr
	GTVe       []InfGTVe
	InfCteComp []string

	AutXML  []string
	RespTec *RespTec
}

// TribFed é infTribFed (TCTeOS/imp): PIS, COFINS, IR, INSS e CSLL, todos
// opcionais (TDec_1302, sem negativo).
type TribFed struct{ VPIS, VCOFINS, VIR, VINSS, VCSLL *float64 }

// InfServicoOS é infServico: descrição (1-30) e quantidade (passageiros, ou
// volumes/malotes em excesso de bagagem e valores).
type InfServicoOS struct {
	XDescServ string
	QCarga    *float64
}

// DocRefOS é infDocRef: ou ChBPe (bilhete eletrônico) ou NDoc+DEmi (papel).
type DocRefOS struct {
	ChBPe                       string
	NDoc, Serie, Subserie, DEmi string // DEmi AAAA-MM-DD
	VDoc                        *float64
}

// SegOS é seg: RespSeg "4" emitente, "5" tomador.
type SegOS struct{ RespSeg, XSeg, NApol string }

// InfGTVe liga uma GTV-e (chave) ao CT-e OS com seus componentes de valor.
type InfGTVe struct {
	Chave string
	Comp  []CompGTVe
}

// CompGTVe: TpComp 1 custódia, 2 embarque, 3 tempo de espera, 4 malote,
// 5 ad valorem, 6 outros (XComp, até 15 caracteres).
type CompGTVe struct {
	TpComp string
	Valor  float64
	XComp  string
}

// RodoviarioOS é rodoOS (cteModalRodoviarioOS_v4.00.xsd): exatamente um entre
// TAF (12 dígitos) e NroRegEstadual (25 dígitos).
type RodoviarioOS struct {
	TAF, NroRegEstadual string
	Veic                *VeicOS
	Fretamento          *Fretamento
}

// VeicOS é o veículo; Prop só quando não pertence ao emitente.
type VeicOS struct {
	Placa, RENAVAM, UF string
	Prop               *PropVeicOS
}

// PropVeicOS: CNPJ ou CPF; TAF ou NroRegEstadual; TpProp "0" TAC agregado,
// "1" TAC independente, "2" outros. IE e UF vêm juntos.
type PropVeicOS struct {
	CNPJ, CPF, TAF, NroRegEstadual, Nome, IE, UF, TpProp string
}

// Fretamento (infFretamento, só transporte de pessoas): TpFretamento "1"
// eventual (DhViagem obrigatório), "2" contínuo (sem DhViagem).
type Fretamento struct {
	TpFretamento string
	DhViagem     *time.Time
}
