package builder

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// qrVersao é a versão do QR Code NFC-e implementada — 2.00 (NT 2015.002 e
// revisões). A versão 1.00 (campo signAC, incluía cDest/vICMS/dIEDest) está
// obsoleta desde 2017 e é rejeitada pela SEFAZ.
const qrVersao = "2"

// ── Input ─────────────────────────────────────────────────────────────────────
// EntradaNFe é a struct de alto nível que o caller preenche.
// O builder converte ela para NFe (schema SEFAZ).

type EntradaNFe struct {
	// Identificação
	Serie    string    // ex: "1"
	NNF      string    // número da nota ex: "42"
	DhEmi    time.Time // data/hora de emissão
	NatOp    string    // "VENDA DE MERCADORIA"
	TpAmb    string    // "1"=produção "2"=homologação
	FinNFe   string    // "1"=normal
	IndFinal string    // "1"=consumidor final
	IndPres  string    // "1"=presencial

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
	AutXML    []EntradaAutXML // terceiros autorizados a baixar o XML da nota
	Itens     []EntradaItem
	Frete     EntradaFrete
	Pagamento []EntradaPagamento
	InfCpl    string // informações complementares

	// RespTec — responsável técnico pelo sistema emissor (NT 2015/002).
	// Opcional no schema, mas algumas SEFAZ tratam como obrigatório na
	// prática. Preencher todos os 4 campos ou nenhum.
	RespTecCNPJ    string
	RespTecContato string
	RespTecEmail   string
	RespTecFone    string
}

type EntradaEmitente struct {
	CNPJ     string
	Nome     string
	Fantasia string
	IE       string
	CRT      string // "1", "2" ou "3"
	End      EntradaEndereco
}

// EntradaAutXML identifica 1 terceiro autorizado a baixar o XML — CNPJ ou CPF.
type EntradaAutXML struct {
	CNPJ string
	CPF  string
}

type EntradaDest struct {
	CNPJ      string // preencher CNPJ ou CPF
	CPF       string
	Nome      string
	IE        string
	IndIEDest string // "1", "2" ou "9"
	Email     string
	End       EntradaEndereco
}

type EntradaEndereco struct {
	Logradouro  string
	Numero      string
	Complemento string
	Bairro      string
	CodigoMun   string // código IBGE 7 dígitos
	Municipio   string
	UF          string
	CEP         string
	Pais        string // "1058" = Brasil
	NomePais    string // "Brasil"
	Fone        string
}

type EntradaItem struct {
	CProd      string // código interno do produto
	CEAN       string // código de barras EAN-13 (ou "SEM GTIN")
	Nome       string
	NCM        string // ex: "73089090" para estruturas metálicas
	CEST       string // Código Especificador da Substituição Tributária — vazio se não aplicável
	CFOP       string // ex: "5102"
	CBenef     string // código de benefício fiscal (Convênio ICMS 190/17) -- vazio se o item não tem benefício
	Unidade    string // "UN", "KG", "M2", etc.
	Quantidade float64
	VUnitario  float64
	VDesconto  float64
	ICMS       EntradaICMS
	IPI        *EntradaIPI    // nil = sem IPI
	IBSCBS     *EntradaIBSCBS // nil = sem grupo IBS/CBS (obrigatório em homologação desde 01/07/2026 pra CRT=3)
	PISCofins  EntradaPISCofins
	// ICMSUFDest = DIFAL (EC 87/2015) -- só quando o destinatário é consumidor
	// final não contribuinte em outro Estado. nil = sem DIFAL (venda interna,
	// venda interestadual pra revenda, ou Simples Nacional -- ver comentário
	// em ICMSUFDest no tipos.go).
	ICMSUFDest *EntradaICMSUFDest
}

// EntradaICMSUFDest informa as alíquotas do DIFAL -- alíquota interna do UF
// de destino e interestadual variam por produto/UF/regime, sem default
// seguro pra chutar (mesma filosofia de EntradaIBSCBS): quem chama informa,
// tipicamente a partir de um cadastro de "ICMS por UF de destino" mantido
// pela empresa.
type EntradaICMSUFDest struct {
	AliqInterna       float64 // pICMSUFDest -- alíquota interna do UF de destino pro produto
	AliqInterestadual float64 // pICMSInter -- alíquota interestadual (4/7/12%, tabela CONFAZ)
	AliqFCP           float64 // pFCPUFDest -- Fundo de Combate à Pobreza do UF de destino, 0 se não houver
}

// EntradaPISCofins sobrepõe o CST/alíquota de PIS/COFINS do item -- CST vazio
// mantém o comportamento antigo (07=isento pro Simples, 01@0.65/3.00 pro
// Regime Normal). AliqPIS/AliqCOFINS só valem quando o CST exige alíquota
// (01, 02, 99); CSTs de não-tributação (04-09, 49...) ignoram os dois.
type EntradaPISCofins struct {
	CST        string
	AliqPIS    float64
	AliqCOFINS float64
}

// EntradaIBSCBS informa a tributação IBS/CBS do item (NT 2025.002-RTC).
// CST e ClassTrib vêm da tabela oficial publicada em nfe.fazenda.gov.br >
// Documentos > Diversos (Anexo III cClassTrib) -- não tem default seguro
// aqui, quem chama informa. Alíquotas idem: variam por UF/Município/período
// de transição, sem valor universal pra chutar.
type EntradaIBSCBS struct {
	CST        string
	ClassTrib  string
	AliqIBSUF  float64 // % de competência da UF
	AliqIBSMun float64 // % de competência do Município
	AliqCBS    float64 // %
	// PercentualReducao — grupo gRed (redução de alíquota, LC 214/25 art.
	// 125 e Anexos II-X: saúde/educação/insumo agropecuário/cultura em
	// 60%, cesta básica/produtos in natura em 100%). 0 = sem redução
	// (comportamento de sempre). Quem chama informa -- não tem tabela
	// NCM/serviço->percentual embutida na lib, mesma filosofia do resto
	// deste struct.
	PercentualReducao float64
}

type EntradaICMS struct {
	// Regime Normal
	CST    string  // "00", "20", "40", "60", "90"
	ModBC  string  // "3" = valor da operação
	PRedBC float64 // % redução de BC (ICMS20)
	Aliq   float64 // alíquota %
	// Simples Nacional
	CSOSN string // "102", "400", "500", etc.
	// ST prospectivo (CST 10)
	ModBCST string
	PMVAST  float64
	AliqST  float64
	// ST retido anteriormente (CST 60 no Regime Normal, CSOSN 500 no Simples)
	VBCSTRet   float64
	PST        float64 // alíquota suportada pelo consumidor final (schema: pST)
	VICMSSTRet float64
	// Desoneração
	VICMSDeson float64
	MotDesICMS string
}

type EntradaIPI struct {
	CEnq string // código de enquadramento
	CST  string // "50"=tributado por alíq, "99"=outros
	Aliq float64
}

type EntradaFrete struct {
	Modalidade string // "0"=CIF "1"=FOB "9"=sem frete
	VFrete     float64
}

type EntradaPagamento struct {
	Forma  string // "01"=dinheiro "15"=boleto "99"=outros
	XPag   string // obrigatório quando Forma="99" (descrição do meio de pagamento)
	Valor  float64
	APrazo bool

	// Cartão (Forma="03" crédito ou "04" débito) -- TBand preenchido dispara o
	// grupo card. TpIntegra vazio assume "2" (não integrado): a maioria dos
	// emitentes não roda o pagamento pelo próprio sistema, só registra a
	// bandeira usada na maquininha de terceiro.
	TBand             string // 01=Visa, 02=Mastercard, 03=Amex, 06=Elo, 07=Hipercard... tabela oficial por código
	TpIntegra         string // "1"=integrado (TEF/POS do emissor) "2"=não integrado (padrão quando TBand preenchido)
	CNPJCredenciadora string
	CAut              string // código de autorização da transação, se disponível
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
	// tpEmis entra na própria chave de acesso (posição fixa, antes do cNF) —
	// tem que bater com o <tpEmis> do XML (montado abaixo com o mesmo
	// default). Antes disso era sempre "1" aqui, então qualquer emissão em
	// contingência (FS-DA, EPEC) saía com chave e elemento divergentes; a
	// SEFAZ valida essa consistência.
	tpEmisChave := e.TpEmis
	if tpEmisChave == "" {
		tpEmisChave = "1"
	}
	chave := NovaChave(e.Emitente.End.UF, FormatarCNPJ(e.Emitente.CNPJ),
		e.Serie, e.NNF, tpEmisChave, mod, e.DhEmi)

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

// TotaisEPEC contém os 3 valores que o evento prévio EPEC exige (vNF, vICMS,
// vST) já formatados no padrão decimal do leiaute — mesmo cálculo usado no
// total da NF-e (montarDetalhes), exposto aqui pra quem monta o evento EPEC
// não duplicar a soma de impostos por item.
type TotaisEPEC struct {
	VNF   string
	VICMS string
	VST   string
}

// CalcularTotaisEPEC soma os itens de uma EntradaNFe pros 3 campos que o
// evento prévio EPEC exige. Não builda nem valida o resto da entrada — só a
// conta, reaproveitando a mesma lógica de imposto por item do Build normal.
func CalcularTotaisEPEC(e EntradaNFe) (TotaisEPEC, error) {
	_, totais, _, err := montarDetalhes(e)
	if err != nil {
		return TotaisEPEC{}, fmt.Errorf("builder: calcular totais EPEC: %w", err)
	}
	return TotaisEPEC{VNF: totais.VNF, VICMS: totais.VICMS, VST: totais.VST}, nil
}

// ── Montagem ─────────────────────────────────────────────────────────────────

func montarNFe(e EntradaNFe, chave ChaveAcesso) (NFe, error) {
	detalhes, totais, ibscbsTot, err := montarDetalhes(e)
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
	// Em homologação o xNome do destinatário tem que ser exatamente este texto
	// (regra nacional, todo SEFAZ). Sem isso a SEFAZ rejeita com cStat=999
	// "Erro não catalogado", que não dá nenhuma pista da causa.
	if tpAmb == "2" && dest != nil {
		dest.XNome = XNomeDestHomologacao
	}
	finNFe := e.FinNFe
	if finNFe == "" {
		finNFe = "1"
	}

	tpEmis := e.TpEmis
	if tpEmis == "" {
		tpEmis = "1"
	}

	// modFrete não tem valor vazio válido no enum — sem default aqui, quem
	// esquecer de preencher só descobre com cStat=225 "Falha no Schema XML"
	// vindo da SEFAZ, que não diz qual campo.
	modFrete := e.Frete.Modalidade
	if modFrete == "" {
		modFrete = "9" // sem ocorrência de transporte
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
			Emit:   montarEmitente(e.Emitente),
			Dest:   dest,
			AutXML: montarAutXML(e.AutXML),
			Det:    detalhes,
			Total:  Total{ICMSTot: totais, IBSCBSTot: ibscbsTot},
			Transp: Transporte{
				ModFrete: modFrete,
			},
			Pag: montarPagamento(e.Pagamento),
			InfAdic: func() *InfAdic {
				if e.InfCpl == "" {
					return nil
				}
				return &InfAdic{InfCpl: e.InfCpl}
			}(),
			InfRespTec: func() *InfRespTec {
				if e.RespTecCNPJ == "" {
					return nil
				}
				return &InfRespTec{
					CNPJ:     e.RespTecCNPJ,
					XContato: e.RespTecContato,
					Email:    e.RespTecEmail,
					Fone:     e.RespTecFone,
				}
			}(),
		},
	}

	// tpEmis=9 (contingência offline): o QR Code precisa do DigestValue da
	// assinatura, que só existe depois de assinar — montado por
	// MontarQRCodeContingenciaNFCe e injetado pós-assinatura.
	if mod == ModeloNFCe && tpEmis != "9" {
		nfe.InfNFeSupl = montarQRCode(e, chave, tpAmb)
	}

	return nfe, nil
}

// qrHashCode calcula o cHashQRCode: SHA-1 da string de parâmetros (já unida
// por "|", sem o hash) concatenada diretamente com o CSC. Resultado em hex
// maiúsculo (40 chars). Vale igual pro QR online e pro de contingência.
func qrHashCode(paramsUnidos, csc string) string {
	h := sha1.Sum([]byte(paramsUnidos + csc))
	return strings.ToUpper(hex.EncodeToString(h[:]))
}

// montarQRCode gera o infNFeSupl com QR Code versão 2.00 (NT 2015.002 e
// revisões) para NFC-e emitida ONLINE (tpEmis != 9):
//
//	URL?p=chNFe|2|tpAmb|cIdToken|cHashQRCode
//
// O QR de contingência offline (tpEmis=9) é outro formato — precisa do
// DigestValue da assinatura, montado só depois de assinar
// (ver MontarQRCodeContingenciaNFCe).
func montarQRCode(e EntradaNFe, chave ChaveAcesso, tpAmb string) *InfNFeSupl {
	chNFe := chave.String()
	cIdToken := semZerosEsquerda(e.CSCId)
	params := strings.Join([]string{chNFe, qrVersao, tpAmb, cIdToken}, "|")
	qrCode := e.UrlConsultaNFCe + "?p=" + params + "|" + qrHashCode(params, e.CSC)
	// urlChave é só a URL base da consulta por chave (o schema limita a 85
	// chars — a URL de GO + "?chNFe=" + 44 dígitos estoura). O consumidor
	// digita a chave nesse site; quem já tem o QR não usa esse campo.
	return &InfNFeSupl{QrCode: qrCode, UrlChave: e.UrlConsultaNFCe}
}

// MontarQRCodeContingenciaNFCe monta o infNFeSupl da NFC-e em contingência
// offline (tpEmis=9). Precisa do DigestValue da assinatura (base64, como sai
// no <DigestValue> do XML assinado) e do valor total da nota (vNF, 2 casas,
// ponto decimal — o mesmo que sai em <ICMSTot><vNF>):
//
//	URL?p=chNFe|2|tpAmb|dd|vNF|digValHex|cIdToken|cHashQRCode
//
// dd = dia (2 dígitos) de e.DhEmi; digValHex = hex maiúsculo dos bytes do
// DigestValue (base64 decodificado).
func MontarQRCodeContingenciaNFCe(e EntradaNFe, chave ChaveAcesso, vNF, digestValueB64 string) (*InfNFeSupl, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(digestValueB64))
	if err != nil {
		return nil, fmt.Errorf("builder: DigestValue base64 inválido: %w", err)
	}
	tpAmb := e.TpAmb
	if tpAmb == "" {
		tpAmb = "2"
	}
	chNFe := chave.String()
	params := strings.Join([]string{
		chNFe, qrVersao, tpAmb,
		e.DhEmi.Format("02"),
		vNF,
		strings.ToUpper(hex.EncodeToString(raw)),
		semZerosEsquerda(e.CSCId),
	}, "|")
	qrCode := e.UrlConsultaNFCe + "?p=" + params + "|" + qrHashCode(params, e.CSC)
	return &InfNFeSupl{QrCode: qrCode, UrlChave: e.UrlConsultaNFCe}, nil
}

// XMLFragment serializa o infNFeSupl com o nome de tag do schema (minúsculo).
// Usado pra injetar o bloco depois da assinatura (contingência) — encoding/xml
// sozinho usaria o nome do tipo Go ("InfNFeSupl").
func (s *InfNFeSupl) XMLFragment() string {
	esc := func(v string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(v))
		return b.String()
	}
	return "<infNFeSupl><qrCode>" + esc(s.QrCode) + "</qrCode><urlChave>" + esc(s.UrlChave) + "</urlChave></infNFeSupl>"
}

func montarEmitente(e EntradaEmitente) Emitente {
	return Emitente{
		CNPJ:  FormatarCNPJ(e.CNPJ),
		XNome: e.Nome,
		XFant: e.Fantasia,
		IE:    FormatarIE(e.IE),
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
	// indIEDest não tem valor vazio válido no enum. "9" (não contribuinte) é o
	// caso comum de destinatário sem inscrição estadual informada.
	indIEDest := d.IndIEDest
	if indIEDest == "" {
		indIEDest = "9"
	}

	dest := Destinatario{
		CNPJ:      FormatarCNPJ(d.CNPJ),
		CPF:       FormatarCPF(d.CPF),
		XNome:     d.Nome,
		IndIEDest: indIEDest,
		IE:        FormatarIE(d.IE),
		Email:     d.Email,
	}

	// Sem UF não dá pra montar um enderDest válido — melhor omitir o grupo
	// inteiro (é minOccurs=0) do que emitir com filhos obrigatórios vazios.
	if d.End.UF != "" {
		dest.EnderDest = &EnderecoDest{
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
		}
	}
	return dest
}

func montarAutXML(entradas []EntradaAutXML) []AutXML {
	if len(entradas) == 0 {
		return nil
	}
	autXML := make([]AutXML, len(entradas))
	for i, a := range entradas {
		autXML[i] = AutXML{CNPJ: FormatarCNPJ(a.CNPJ), CPF: FormatarCPF(a.CPF)}
	}
	return autXML
}

func montarDetalhes(e EntradaNFe) ([]Detalhe, ICMSTot, *IBSCBSTot, error) {
	var detalhes []Detalhe
	tot := ICMSTot{}
	vProdTotal := 0.0
	vDescTotal := 0.0
	vBCTotal := 0.0
	vICMSTotal := 0.0
	vBCSTTotal := 0.0
	vICMSSTTotal := 0.0
	vPISTotal := 0.0
	vCOFINSTotal := 0.0
	vBCIBSCBSTotal := 0.0
	vIBSUFTotal := 0.0
	vIBSMunTotal := 0.0
	vIBSTotal := 0.0
	vCBSTotal := 0.0
	vIPITotal := 0.0
	vICMSUFDestTotal := 0.0
	vICMSUFRemetTotal := 0.0
	vFCPUFDestTotal := 0.0
	temIBSCBS := false

	for i, item := range e.Itens {
		vProd := item.Quantidade * item.VUnitario
		vProdTotal += vProd
		vDescTotal += item.VDesconto

		imp, totItem := montarImposto(item, e.Emitente.CRT)
		vBCTotal += totItem.vBC
		vICMSTotal += totItem.vICMS
		vBCSTTotal += totItem.vBCST
		vICMSSTTotal += totItem.vICMSST
		vPISTotal += totItem.vPIS
		vCOFINSTotal += totItem.vCOFINS
		vIPITotal += totItem.vIPI
		vICMSUFDestTotal += totItem.vICMSUFDest
		vICMSUFRemetTotal += totItem.vICMSUFRemet
		vFCPUFDestTotal += totItem.vFCPUFDest
		if item.IBSCBS != nil {
			temIBSCBS = true
			vBCIBSCBSTotal += totItem.vBCIBSCBS
			vIBSUFTotal += totItem.vIBSUF
			vIBSMunTotal += totItem.vIBSMun
			vIBSTotal += totItem.vIBS
			vCBSTotal += totItem.vCBS
		}

		// Em homologação a descrição do PRIMEIRO item tem que ser exatamente
		// este texto (regra nacional, MOC) — senão cStat=373. Igual ao
		// tratamento do xNome do destinatário (ver montarNFe).
		xProd := item.Nome
		if i == 0 && e.TpAmb != "1" {
			xProd = XProdPrimeiroItemHomologacao
		}

		det := Detalhe{
			NItem: fmt.Sprintf("%d", i+1),
			Prod: Produto{
				CProd:    item.CProd,
				CEAN:     ceanOuSemGTIN(item.CEAN),
				XProd:    xProd,
				NCM:      item.NCM,
				CEST:     item.CEST,
				CBenef:   item.CBenef,
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
			Imposto: imp,
		}

		if e.Frete.VFrete > 0 && i == 0 {
			det.Prod.VFrete = fmtVal(e.Frete.VFrete)
		}

		detalhes = append(detalhes, det)
	}

	if vDescTotal > 0 {
		tot.VDesc = fmtVal(vDescTotal)
	}
	tot.VProd = fmtVal(vProdTotal)
	tot.VFrete = fmtVal(e.Frete.VFrete)
	tot.VNF = fmtVal(vProdTotal + e.Frete.VFrete - vDescTotal)
	tot.VBC = fmtVal(vBCTotal)
	tot.VICMS = fmtVal(vICMSTotal)
	tot.VICMSDeson = "0.00"
	tot.VFCPUFDest = fmtValOmitZero(vFCPUFDestTotal)
	tot.VICMSUFDest = fmtValOmitZero(vICMSUFDestTotal)
	tot.VICMSUFRemet = fmtValOmitZero(vICMSUFRemetTotal)
	tot.VFCP = "0.00"
	tot.VBCST = fmtVal(vBCSTTotal)
	tot.VST = fmtVal(vICMSSTTotal)
	tot.VFCPST = "0.00"
	tot.VFCPSTRet = "0.00"
	tot.VSeg = "0.00"
	tot.VII = "0.00"
	tot.VIPI = fmtVal(vIPITotal)
	tot.VIPIDevol = "0.00"
	tot.VPIS = fmtVal(vPISTotal)
	tot.VCOFINS = fmtVal(vCOFINSTotal)
	tot.VOutro = "0.00"
	tot.VTotTrib = "0.00"
	if tot.VDesc == "" {
		tot.VDesc = "0.00"
	}

	var ibscbsTot *IBSCBSTot
	if temIBSCBS {
		ibscbsTot = &IBSCBSTot{
			VBCIBSCBS: fmtVal(vBCIBSCBSTotal),
			GIBS: GIBSTotal{
				GIBSUF:           GIBSUFTotal{VDif: "0.00", VDevTrib: "0.00", VIBSUF: fmtVal(vIBSUFTotal)},
				GIBSMun:          GIBSMunTotal{VDif: "0.00", VDevTrib: "0.00", VIBSMun: fmtVal(vIBSMunTotal)},
				VIBS:             fmtVal(vIBSTotal),
				VCredPres:        "0.00",
				VCredPresCondSus: "0.00",
			},
			GCBS: GCBSTotalIBS{
				VDif: "0.00", VDevTrib: "0.00", VCBS: fmtVal(vCBSTotal),
				VCredPres: "0.00", VCredPresCondSus: "0.00",
			},
		}
	}

	return detalhes, tot, ibscbsTot, nil
}

// montarPISCOFINS resolve PIS/COFINS do item. CST vazio aplica defaultCST
// (comportamento de antes desse override existir: "07" isento pro Simples,
// "01"@0.65/3.00 pro Regime Normal). CSTs 01/02/99 levam vBC+alíquota; os
// demais (04-09, 49...) são não-tributação, sem base nem valor.
func montarPISCOFINS(p EntradaPISCofins, vProd float64, defaultCST string) (PIS, COFINS, float64, float64) {
	cst, aliqPIS, aliqCOFINS := p.CST, p.AliqPIS, p.AliqCOFINS
	if cst == "" {
		cst = defaultCST
		if cst == "01" {
			aliqPIS, aliqCOFINS = 0.65, 3.00
		}
	}

	switch cst {
	case "01", "02":
		vPIS := vProd * aliqPIS / 100
		vCOFINS := vProd * aliqCOFINS / 100
		return PIS{PISAliq: &PISAliq{CST: cst, VBC: fmtVal(vProd), PPIS: fmtVal(aliqPIS), VPIS: fmtVal(vPIS)}},
			COFINS{COFINSAliq: &COFINSAliq{CST: cst, VBC: fmtVal(vProd), PCOFINS: fmtVal(aliqCOFINS), VCOFINS: fmtVal(vCOFINS)}},
			vPIS, vCOFINS
	case "99":
		vPIS := vProd * aliqPIS / 100
		vCOFINS := vProd * aliqCOFINS / 100
		return PIS{PISOutr: &PISOutr{CST: cst, VBC: fmtVal(vProd), PPIS: fmtVal(aliqPIS), VPIS: fmtVal(vPIS)}},
			COFINS{COFINSOutr: &COFINSOutr{CST: cst, VBC: fmtVal(vProd), PCOFINS: fmtVal(aliqCOFINS), VCOFINS: fmtVal(vCOFINS)}},
			vPIS, vCOFINS
	default:
		return PIS{PISNt: &PISNt{CST: cst}}, COFINS{COFINSNt: &COFINSNt{CST: cst}}, 0, 0
	}
}

// totaisItem carrega as parcelas de ICMS e IBS/CBS de um item que precisam
// ser somadas no total da NF-e -- SEFAZ rejeita (regras 531/etc.) se o total
// declarado não bater com o somatório dos itens.
type totaisItem struct {
	vBC, vICMS, vBCST, vICMSST             float64
	vPIS, vCOFINS                          float64
	vIPI                                   float64
	vBCIBSCBS, vIBSUF, vIBSMun, vIBS, vCBS float64
	vICMSUFDest, vICMSUFRemet, vFCPUFDest  float64
}

func montarImposto(item EntradaItem, crt string) (Imposto, totaisItem) {
	imp := Imposto{}

	icms := &ICMS{}
	// Simples Nacional (CRT 1 ou 2)
	if crt == "1" || crt == "2" {
		csosn := item.ICMS.CSOSN
		if csosn == "" {
			csosn = "400" // isento/sem destaque (mais comum no SN)
		}
		// CSOSN 500: ICMS-ST retido na operação anterior (revenda de mercadoria
		// já tributada -- bebida, autopeça, etc). Grupo próprio ICMSSN500 com os
		// valores da retenção, que vêm da nota de entrada do fornecedor. Os
		// demais CSOSN de Simples (102/103/300/400) não carregam valor.
		if csosn == "500" {
			icms.ICMSSN500 = &ICMSSN500{
				Orig: "0", CSOSN: "500",
				VBCSTRet:   fmtVal(item.ICMS.VBCSTRet),
				PST:        fmtVal(item.ICMS.PST),
				VICMSSTRet: fmtVal(item.ICMS.VICMSSTRet),
			}
		} else {
			icms.ICMSSN102 = &ICMSSN102{Orig: "0", CSOSN: csosn}
		}
		imp.ICMS = icms
		vProd := item.Quantidade * item.VUnitario
		pis, cofins, vPIS, vCOFINS := montarPISCOFINS(item.PISCofins, vProd, "07")
		imp.PIS, imp.COFINS = pis, cofins
		return imp, totaisItem{vPIS: vPIS, vCOFINS: vCOFINS}
	}

	// Regime Normal (CRT 3)
	cst := item.ICMS.CST
	if cst == "" {
		cst = "00"
	}
	vProd := item.Quantidade * item.VUnitario
	tot := totaisItem{}

	switch cst {
	case "00":
		vBC := vProd
		vICMS := vBC * item.ICMS.Aliq / 100
		icms.ICMS00 = &ICMS00{
			Orig: "0", CST: "00", ModBC: "3",
			VBC: fmtVal(vBC), PICMS: fmtVal(item.ICMS.Aliq), VICMS: fmtVal(vICMS),
		}
		tot.vBC, tot.vICMS = vBC, vICMS
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
		tot.vBC, tot.vICMS, tot.vBCST, tot.vICMSST = vBC, vICMS, vBCST, vICMSST
	case "40", "41", "50":
		icms.ICMS40 = &ICMS40{Orig: "0", CST: cst}
	case "60":
		icms.ICMS60 = &ICMS60{
			Orig:       "0",
			CST:        "60",
			VBCSTRet:   fmtVal(item.ICMS.VBCSTRet),
			PST:        fmtVal(item.ICMS.PST),
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
		tot.vBC, tot.vICMS = vBC, vICMS
	default:
		icms.ICMS40 = &ICMS40{Orig: "0", CST: "40"}
	}

	imp.ICMS = icms
	pis, cofins, vPIS, vCOFINS := montarPISCOFINS(item.PISCofins, vProd, "01")
	imp.PIS, imp.COFINS = pis, cofins
	tot.vPIS, tot.vCOFINS = vPIS, vCOFINS

	if item.IPI != nil {
		vIPI := vProd * item.IPI.Aliq / 100
		imp.IPI = &IPI{
			CEnq:    item.IPI.CEnq,
			IPITrib: &IPITrib{CST: item.IPI.CST, VBC: fmtVal(vProd), PIPI: fmtVal(item.IPI.Aliq), VIPI: fmtVal(vIPI)},
		}
		tot.vIPI = vIPI
	}

	if item.ICMSUFDest != nil {
		ud := item.ICMSUFDest
		vBCUFDest := vProd
		vICMSUFRemet := vBCUFDest * ud.AliqInterestadual / 100
		vICMSUFDest := vBCUFDest * (ud.AliqInterna - ud.AliqInterestadual) / 100 // pICMSInterPart=100% desde 2019
		if vICMSUFDest < 0 {
			vICMSUFDest = 0
		}
		vFCPUFDest := vBCUFDest * ud.AliqFCP / 100
		grupo := &ICMSUFDest{
			VBCUFDest:      fmtVal(vBCUFDest),
			PICMSUFDest:    fmtVal(ud.AliqInterna),
			PICMSInter:     fmtVal(ud.AliqInterestadual),
			PICMSInterPart: "100",
			VICMSUFDest:    fmtVal(vICMSUFDest),
			VICMSUFRemet:   fmtVal(vICMSUFRemet),
		}
		if ud.AliqFCP > 0 {
			grupo.VBCFCPUFDest = fmtVal(vBCUFDest)
			grupo.PFCPUFDest = fmtVal(ud.AliqFCP)
			grupo.VFCPUFDest = fmtVal(vFCPUFDest)
		}
		imp.ICMSUFDest = grupo
		tot.vICMSUFDest, tot.vICMSUFRemet, tot.vFCPUFDest = vICMSUFDest, vICMSUFRemet, vFCPUFDest
	}

	if item.IBSCBS != nil {
		ib := item.IBSCBS
		vBCIBSCBS := vProd

		// gRed: com redução, a alíquota efetiva (e o valor devido) sai
		// menor que a nominal -- pIBSUF/pIBSMun/pCBS continuam mostrando a
		// alíquota padrão, só o grupo gRed carrega o percentual reduzido e
		// a alíquota efetiva (NT 2025.002-RTC, TRed).
		fatorRed := 1 - ib.PercentualReducao/100
		aliqEfetUF := ib.AliqIBSUF * fatorRed
		aliqEfetMun := ib.AliqIBSMun * fatorRed
		aliqEfetCBS := ib.AliqCBS * fatorRed

		vIBSUF := vBCIBSCBS * aliqEfetUF / 100
		vIBSMun := vBCIBSCBS * aliqEfetMun / 100
		vIBS := vIBSUF + vIBSMun
		vCBS := vBCIBSCBS * aliqEfetCBS / 100

		gIBSUF := GIBSUF{PIBSUF: fmtVal(ib.AliqIBSUF), VIBSUF: fmtVal(vIBSUF)}
		gIBSMun := GIBSMun{PIBSMun: fmtVal(ib.AliqIBSMun), VIBSMun: fmtVal(vIBSMun)}
		gCBS := GCBS{PCBS: fmtVal(ib.AliqCBS), VCBS: fmtVal(vCBS)}
		if ib.PercentualReducao > 0 {
			gIBSUF.GRed = &GRed{PRedAliq: fmtVal(ib.PercentualReducao), PAliqEfet: fmtVal(aliqEfetUF)}
			gIBSMun.GRed = &GRed{PRedAliq: fmtVal(ib.PercentualReducao), PAliqEfet: fmtVal(aliqEfetMun)}
			gCBS.GRed = &GRed{PRedAliq: fmtVal(ib.PercentualReducao), PAliqEfet: fmtVal(aliqEfetCBS)}
		}

		imp.IBSCBS = &IBSCBS{
			CST: ib.CST, ClassTrib: ib.ClassTrib,
			GIBSCBS: &GIBSCBS{
				VBC:     fmtVal(vBCIBSCBS),
				GIBSUF:  gIBSUF,
				GIBSMun: gIBSMun,
				VIBS:    fmtVal(vIBS),
				GCBS:    gCBS,
			},
		}
		tot.vBCIBSCBS, tot.vIBSUF, tot.vIBSMun, tot.vIBS, tot.vCBS = vBCIBSCBS, vIBSUF, vIBSMun, vIBS, vCBS
	}

	return imp, tot
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
		var card *Card
		if p.TBand != "" {
			tpIntegra := p.TpIntegra
			if tpIntegra == "" {
				tpIntegra = "2"
			}
			card = &Card{
				TpIntegra: tpIntegra,
				CNPJ:      FormatarCNPJ(p.CNPJCredenciadora),
				TBand:     p.TBand,
				CAut:      p.CAut,
			}
		}
		pag.DetPag = append(pag.DetPag, DetalhePag{
			IndPag: indPag,
			TPag:   p.Forma,
			XPag:   xPag,
			VPag:   fmtVal(p.Valor),
			Card:   card,
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
	// UF tem que estar em EstadoCodigo (a lista canônica das 27 siglas), não só
	// ser não-vazia. UF inválida ("go", "XX") não dava erro nenhum antes: a
	// chave nascia com cUF=99 (fallback de NovaChave) e a transmissão caía no
	// SVRS em vez da SEFAZ do estado — documento fiscal endereçado errado, em
	// silêncio, por dois fallbacks que apontam pra lugares diferentes.
	if _, ok := EstadoCodigo[e.Emitente.End.UF]; !ok {
		return fmt.Errorf("UF do emitente inválida: %q (esperado a sigla de 2 letras maiúsculas, ex: \"GO\")", e.Emitente.End.UF)
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
	if e.TpEmis == "4" || e.TpEmis == "5" || e.TpEmis == "9" {
		if e.DhCont == "" {
			return fmt.Errorf("TpEmis=%s exige DhCont preenchida (data/hora entrada em contingência)", e.TpEmis)
		}
		if len(e.XJust) < 15 {
			return fmt.Errorf("TpEmis=%s exige XJust com pelo menos 15 caracteres (atual: %d)", e.TpEmis, len(e.XJust))
		}
	}
	if e.TpEmis == "9" && e.Mod != ModeloNFCe {
		return fmt.Errorf("TpEmis=9 (contingência offline) só vale para NFC-e (mod=65)")
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
