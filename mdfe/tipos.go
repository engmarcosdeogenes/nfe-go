package mdfe

import "time"

// Entrada é o que o chamador preenche. Não expõe structs do schema.
type Entrada struct {
	TpAmb  string // "1" produção, "2" homologação
	TpEmit string // "1" prestador (CT-e), "2" carga própria, "3" prestador com CT-e globalizado (NF-e)
	// TpTransp: "1" ETC, "2" TAC, "3" CTC. Opcional; se informado precisa
	// bater com o proprietário da tração (CPF -> 2; CNPJ -> 1 ou 3).
	TpTransp string
	Serie    string // 0-999
	NMDF     string // 1-999999999
	DhEmi    time.Time
	TpEmis   string // "1" normal (vazio vira "1"); "2" contingência offline (exige QRSign)
	// QRSign: assinatura RSA-SHA1 da chave, base64, calculada pelo chamador
	// (o builder não tem o certificado). Obrigatória com TpEmis "2".
	QRSign string
	// CMDF: cMDF da chave, 8 dígitos (opcional; vazio = aleatório). Preencha para
	// obter a chave com Chave(e) antes do Build (ex.: assinar o QR de contingência).
	CMDF string
	// ExigirCIOT força a exigência de CIOT; também é exigido por data
	// (DhEmi >= 2026-11-23, NT 2026.001).
	ExigirCIOT bool
	Seguros    []Seguro

	UFIni, UFFim string
	MunCarrega   []Municipio // 1..50
	UFPercurso   []string    // UFs intermediárias, sem UFIni/UFFim, 0..25
	DhIniViagem  *time.Time

	Emitente  Emitente
	Rodo      Rodoviario
	Descargas []Descarga // 1..1000 municípios, cada um com 1+ NF-e
	ProdPred  *ProdutoPredominante

	VCarga float64
	CUnid  string // "01" KG, "02" TON
	QCarga float64

	Lacres []string
	AutXML []string // CNPJ ou CPF, até 10

	InfAdFisco, InfCpl string
	RespTec            *RespTec
}

type Municipio struct{ CodigoMun, Nome string }

type Emitente struct {
	CNPJ, CPF, IE, Nome, Fantasia string
	End                           Endereco
}

type Endereco struct {
	Logradouro, Numero, Complemento, Bairro string
	CodigoMun, Municipio, CEP, UF           string
	Fone, Email                             string
}

type Rodoviario struct {
	RNTRC        string // opcional para tpEmit=2; 8 dígitos
	CIOTs        []CIOT
	ValePed      *ValePed
	Contratantes []Contratante
	Pagamentos   []Pagamento
	Tracao       Veiculo
	Reboques     []Veiculo // 0..3
	Condutores   []Condutor
}

type Veiculo struct {
	CInt, Placa, Renavam string
	TaraKG, CapKG, CapM3 int
	TpRod                string // só tração: "01".."06"
	TpCar                string // "00".."05"
	UF                   string
	Prop                 *Proprietario
}

type Proprietario struct {
	CPF, CNPJ, RNTRC, Nome, IE, UF string
	TpProp                         string // "0" TAC agregado, "1" TAC independente, "2" outros
}

type Condutor struct{ Nome, CPF string }

type Descarga struct {
	CodigoMun, Nome string
	NFes            []string // chaves de NF-e (44 posições)
	CTes            []string // chaves de CT-e (proibido com tpEmit=2)
	MDFes           []string // chaves de MDF-e transportado (infMDFeTransp)
	// Opc traz os opcionais por chave: SegCodBarra e indReentrega.
	Opc map[string]DocOpc
}

type DocOpc struct {
	SegCodBarra  string // 36 dígitos (CT-e/NF-e)
	IndReentrega bool
}

type CIOT struct{ CIOT, CPF, CNPJ string }

type ValePed struct {
	Disp          []ValeDisp
	CategCombVeic string // opcional
}

type ValeDisp struct {
	CNPJForn, CNPJPg, CPFPg, NCompra, TpValePed string
	VValePed                                    float64
}

type Contratante struct {
	Nome, CPF, CNPJ, IDEstrangeiro string
	NroContrato                    string
	VContratoGlobal                float64
}

type Pagamento struct {
	Nome, CPF, CNPJ, IDEstrangeiro string
	Comp                           []CompPag
	VContrato                      float64
	IndAltoDesemp                  bool
	IndPag                         string // "0" à vista, "1" a prazo
	VAdiant                        float64
	IndAntecipaAdiant              bool
	Prazos                         []Prazo
	TpAntecip                      string // opcional "0".."2"
	CodBanco, CodAgencia           string // banco+agência...
	CNPJIPEF                       string // ...ou IPEF...
	PIX                            string // ...ou PIX (exatamente um)
}

type CompPag struct {
	Tipo  string // 01..04, 99
	Valor float64
	XComp string
}

type Prazo struct {
	Parcela int
	Venc    string // AAAA-MM-DD
	Valor   float64
}

type Seguro struct {
	RespSeg             string // "1" emitente, "2" contratante
	RespCNPJ, RespCPF   string
	Seguradora, CNPJSeg string
	NApol               string
	NAver               []string
}

type ProdutoPredominante struct {
	TpCarga                   string // "01".."12"
	XProd, CEAN, NCM          string
	CEPCarrega, CEPDescarrega string
}

type RespTec struct{ CNPJ, Contato, Email, Fone string }
