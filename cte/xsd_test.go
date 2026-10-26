package cte

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Valida o XML de cada variante contra o XSD oficial (leiaute 4.00, pacote
// NT 2026.004). infModal é xs:any, então o <rodo> é validado à parte contra
// cteModalRodoviario. Signature (obrigatória em TCTe) entra dummy,
// estruturalmente válida. Falha, não pula, se não houver python+lxml.
const scriptXSD = `
import sys
from lxml import etree
d = sys.argv[1]
sch = etree.XMLSchema(etree.parse(d + '/cte_v4.00.xsd'))
doc = etree.parse(sys.argv[2])
sch.validate(doc)
for e in sch.error_log:
    print('cte', e.line, e.message)
ns = {'m': 'http://www.portalfiscal.inf.br/cte'}
rodo = doc.find('.//m:rodo', ns)
if rodo is not None:
    rs = etree.XMLSchema(etree.parse(d + '/cteModalRodoviario_v4.00.xsd'))
    rs.validate(etree.ElementTree(rodo))
    for e in rs.error_log:
        print('rodo', e.line, e.message)
`

const sigDummy = `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignedInfo>` +
	`<CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>` +
	`<SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/>` +
	`<Reference URI="#CTe0"><Transforms><Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/>` +
	`<Transform Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/></Transforms>` +
	`<DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/><DigestValue>AAAA</DigestValue></Reference></SignedInfo>` +
	`<SignatureValue>AAAA</SignatureValue><KeyInfo><X509Data><X509Certificate>AAAA</X509Certificate></X509Data></KeyInfo></Signature>`

func validarXSD(t *testing.T, x []byte) {
	t.Helper()
	py := pythonLxml(t)
	com := string(x)
	if !strings.Contains(com, "<Signature") { // já assinado (ponta a ponta) valida como está
		com = strings.Replace(com, "</CTe>", sigDummy+"</CTe>", 1)
	}
	arq := filepath.Join(t.TempDir(), "cte.xml")
	if err := os.WriteFile(arq, []byte(com), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-c", scriptXSD, filepath.Join("..", "testdata", "schemas", "cte"), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("XSD:\n%s\n%s", s, x)
	}
}

func fp(v float64) *float64 { return &v }

func ch(d string) string { return strings.Repeat(d, 44) }

func entradaBase() Entrada {
	return Entrada{
		TpAmb: "2", Serie: "1", NCT: "1234", DhEmi: time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC),
		CFOP: "6353", NatOp: "Prestacao de servico de transporte", TpCTe: "0", TpServ: "0", Modal: "01",
		Envio:     Local{"5208707", "Goiania", "GO"},
		Inicio:    Local{"5208707", "Goiania", "GO"},
		Fim:       Local{"3550308", "Sao Paulo", "SP"},
		IndIEToma: "1", Toma3: "0",
		Emitente: Emitente{CNPJ: "11222333000181", IE: "102030405", Nome: "Transportadora Alfa", CRT: "1",
			End: Endereco{"Rua A", "10", "", "Centro", "5208707", "Goiania", "74000000", "GO", "", "", "62999990000"}},
		Rem: &Parte{CNPJ: "12345678000195", IE: "109876543", Nome: "Remetente SA",
			End: Endereco{Logradouro: "Rua B", Numero: "20", Bairro: "Setor", CodigoMun: "5208707", Municipio: "Goiania", CEP: "74000000", UF: "GO"}},
		Dest: &Parte{CNPJ: "98765432000110", IE: "111222333", Nome: "Destinatario SA",
			End: Endereco{Logradouro: "Av C", Numero: "30", Bairro: "Bela Vista", CodigoMun: "3550308", Municipio: "Sao Paulo", CEP: "01310100", UF: "SP"}},
		VTPrest: 1000, VRec: 1000, Comp: []Comp{{"Frete valor", 1000}},
		ICMS:  ICMS{SN: true, CST: "90"},
		Carga: &Carga{VCarga: fp(50000), ProPred: "Pecas metalicas", Quantidades: []QCarga{{"01", "PESO BRUTO", 1500}}},
		Docs:  Docs{NFe: []DocNFe{{Chave: ch("1")}}},
		Rodo:  &Rodoviario{RNTRC: "12345678"},
	}
}

func normal(e *Entrada) { // regime normal, ICMS00 12%
	e.Emitente.CRT = "3"
	e.ICMS = ICMS{CST: "00", VBC: 1000, PICMS: 12, VICMS: 120}
	e.IBSCBS = ibsPadrao()
}

func ibsPadrao() *IBSCBS {
	return &IBSCBS{CST: "000", CClassTrib: "000001", G: &GIBSCBS{
		VBC: 1000, IBSUF: AliqRTC{P: 0.1, V: 1}, IBSMun: AliqRTC{P: 0, V: 0}, VIBS: 1, CBS: AliqRTC{P: 0.9, V: 9}}}
}

func TestXSD_VariantesDoBuilder(t *testing.T) {
	sub := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	docAntEle := []EmiDocAnt{{CNPJ: "55666777000188", IE: "123456", UF: "MG", Nome: "Transp Anterior", Eletronico: []string{ch("2")}}}
	variantes := map[string]func(*Entrada){
		"Simples CST90 indSN":              func(e *Entrada) {},
		"MEI (CRT 4)":                      func(e *Entrada) { e.Emitente.CRT = "4" },
		"regime normal ICMS00 com IBS/CBS": normal,
		"IBS/CBS com redução e diferimento": func(e *Entrada) {
			normal(e)
			e.IBSCBS.G.IBSUF = AliqRTC{P: 0.1, V: 0.4, PRedAliq: fp(60), PAliqEfet: fp(0.04), PDif: fp(100), VDif: 0}
			e.IBSCBS.G.IBSUF.V = 0.4
			e.IBSCBS.G.IBSUF.PDif, e.IBSCBS.G.IBSUF.VDif = nil, 0
			e.IBSCBS.G.VIBS = 0.4
			e.IBSCBS.G.CBS = AliqRTC{P: 0.9, V: 9, PDif: fp(10), VDif: 0.9}
			e.IBSCBS.G.CBS.V = 8.1
			e.IBSCBS.IndDoacao = true
			e.IBSCBS.G.Regular = &TribRegular{CST: "000", CClassTrib: "000001", PIBSUF: 0.1, VIBSUF: 1, PIBSMun: 0, VIBSMun: 0, PCBS: 0.9, VCBS: 9}
			e.IBSCBS.EstornoIBS, e.IBSCBS.EstornoCBS = fp(0), fp(0)
			e.VTotDFe = fp(1000)
		},
		"IBS/CBS sem gIBSCBS (imune)": func(e *Entrada) {
			normal(e)
			e.IBSCBS = &IBSCBS{CST: "410", CClassTrib: "410001"}
		},
		"ICMS20": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "20", PRedBC: 60, VBC: 400, PICMS: 12, VICMS: 48, VICMSDeson: 72, CBenef: "GO000001"}
		},
		"ICMS45 CST40": func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "40"} },
		"ICMS45 CST51 com benefício": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "51", VICMSDeson: 10, CBenef: "SEMCBENEF1"}
		},
		"ICMS60 ST retido": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "60", VBCSTRet: 1000, PICMSSTRet: 12, VICMSSTRet: 120, VCred: fp(10)}
		},
		"ICMS90": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "90", PRedBC: 50, VBC: 500, PICMS: 12, VICMS: 60, VCred: fp(5)}
		},
		"ICMSOutraUF": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "90", OutraUF: true, PRedBC: 50, VBC: 500, PICMS: 12, VICMS: 60}
		},
		"ICMS previsto (pagamento antecipado)": func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{Previsto: fp(120)}
		},
		"partilha ICMSUFFim": func(e *Entrada) {
			normal(e)
			e.IndIEToma, e.Toma3 = "9", "3"
			e.Dest.IE = ""
			e.ICMSUFFim = &ICMSUFFim{1000, 2, 18, 12, 20, 60, 0}
			e.VTotTrib = fp(150.5)
			e.InfAdFisco = "Partilha conforme EC 87/2015"
		},
		"tomador toma4 com endereço": func(e *Entrada) {
			e.Toma3 = ""
			e.IndIEToma = "2"
			e.Toma4 = &Terceiro{CNPJ: "44555666000177", IE: "ISENTO", Nome: "Tomador Outros", Fantasia: "Outros", Fone: "1133334444", Email: "t@x.com.br",
				End: Endereco{Logradouro: "Rua D", Numero: "40", Complemento: "Sala 1", Bairro: "Centro", CodigoMun: "3550308", Municipio: "Sao Paulo", CEP: "01001000", UF: "SP"}}
		},
		"toma4 CPF não contribuinte": func(e *Entrada) {
			e.Toma3, e.IndIEToma = "", "9"
			e.Toma4 = &Terceiro{CPF: "12345678909", Nome: "Pessoa Fisica",
				End: Endereco{Logradouro: "Rua E", Numero: "S/N", Bairro: "Centro", CodigoMun: "3550308", Municipio: "Sao Paulo", UF: "SP"}}
		},
		"expedidor e recebedor": func(e *Entrada) {
			e.Toma3 = "1"
			e.Exped = &Parte{CNPJ: "12345678000195", IE: "109876543", Nome: "Expedidor", Fone: "6233334444", Email: "e@x.com.br",
				End: Endereco{Logradouro: "Rua F", Numero: "5", Bairro: "Centro", CodigoMun: "5208707", Municipio: "Goiania", UF: "GO"}}
			e.Receb = &Parte{CPF: "12345678909", Nome: "Recebedor",
				End: Endereco{Logradouro: "Rua G", Numero: "6", Bairro: "Centro", CodigoMun: "3550308", Municipio: "Sao Paulo", UF: "SP"}}
		},
		"destinatário no exterior": func(e *Entrada) {
			e.Toma3, e.IndIEToma = "3", "9"
			e.CFOP = "7358"
			e.Fim = Local{"9999999", "EXTERIOR", "EX"}
			e.Dest = &Parte{Nome: "Foreign Buyer", ISUF: "123456789",
				End: Endereco{Logradouro: "Main St", Numero: "1", Bairro: "Downtown", CodigoMun: "9999999", Municipio: "EXTERIOR", UF: "EX", CodPais: "1058", NomePais: "Estados Unidos"}}
		},
		"subcontratação com docAnt": func(e *Entrada) {
			e.TpServ = "1"
			e.DocAnt = docAntEle
			e.Docs = Docs{}
		},
		"redespacho com docAnt papel e eletrônico": func(e *Entrada) {
			e.TpServ = "2"
			e.DocAnt = []EmiDocAnt{{CPF: "12345678909", Nome: "Autonomo", Eletronico: []string{ch("2"), ch("3")},
				Papel: []DocAntPap{{"09", "1", "A", "123", "2026-10-01"}, {"10", "2", "", "456", "2026-10-02"}}}}
		},
		"redespacho intermediário": func(e *Entrada) {
			e.TpServ = "3"
			e.DocAnt = docAntEle
		},
		"multimodal vinculado": func(e *Entrada) {
			e.TpServ = "4"
			e.InfServVinc = []string{ch("4"), ch("5")}
		},
		"complemento": func(e *Entrada) {
			e.TpCTe = "1"
			e.Carga, e.Rodo, e.Docs = nil, nil, Docs{}
			e.InfCteComp = []string{ch("6")}
			e.VTPrest, e.VRec, e.Comp = 100, 100, []Comp{{"Complemento", 100}}
		},
		"substituição": func(e *Entrada) {
			e.TpCTe = "3"
			e.InfCteSub = &InfCteSub{ChCte: ch("7"), IndAlteraToma: true}
		},
		"com cobrança fat/dup": func(e *Entrada) {
			e.Cobr = &Cobr{NFat: "F-1", VOrig: fp(1000), VDesc: fp(50), VLiq: fp(950),
				Dup: []Dup{{"001", "2026-11-26", fp(475)}, {"002", "2026-12-26", fp(475)}}}
		},
		"várias NF-e com PIN e unidades": func(e *Entrada) {
			e.Docs = Docs{NFe: []DocNFe{
				{Chave: ch("1"), PIN: "123456", DPrev: "2026-11-01", UnidCarga: []UnidCarga{{"1", "ABCD1234567", []string{"L1", "L2"}, 50}}},
				{Chave: ch("2"), UnidTransp: []UnidTransp{{Tp: "1", ID: "TRUCK1", Lacres: []string{"X"}, Carga: []UnidCarga{{Tp: "3", ID: "PAL1"}}, QtdRat: 25.5}}},
				{Chave: ch("3")},
			}}
		},
		"infOutros": func(e *Entrada) {
			e.Docs = Docs{Outros: []DocOutros{
				{TpDoc: "99", Desc: "Declaracao de conteudo", NDoc: "77", DEmi: "2026-10-20", VDocFisc: 1500, DPrev: "2026-11-01"},
				{TpDoc: "00"},
			}}
		},
		"infNF papel": func(e *Entrada) {
			e.Docs = Docs{NF: []DocNF{{NRoma: "R1", NPed: "P1", Mod: "01", Serie: "1", NDoc: "999", DEmi: "2026-10-20",
				VBC: 100, VICMS: 12, VProd: 100, VNF: 100, NCFOP: "6102", NPeso: 12.5, PIN: "98765", DPrev: "2026-11-02"}}}
		},
		"infDCe": func(e *Entrada) { e.Docs = Docs{DCe: []string{ch("8")}} },
		"carga completa e veículos novos": func(e *Entrada) {
			e.Carga = &Carga{VCarga: fp(90000), ProPred: "Veiculos", XOutCat: "Outras", VCargaAverb: fp(90000),
				Quantidades: []QCarga{{"01", "PESO", 12000}, {"03", "UNIDADES", 3}}}
			e.VeicNovos = []VeicNovo{{"9BWZZZ377VT004251", "01", "Preto", "A1", 80000, 500}}
		},
		"rodo com OCC": func(e *Entrada) {
			e.Rodo = &Rodoviario{RNTRC: "ISENTO", OCC: []OCC{{Serie: "1", NOcc: "55", DEmi: "2026-10-20", CNPJ: "11222333000181", CInt: "X1", IE: "102030405", UF: "GO", Fone: "62999990000"}}}
		},
		"compl completo": func(e *Entrada) {
			e.Compl = &Compl{XCaracAd: "Urgente", XCaracSer: "Expresso", XEmi: "Joao", XOrig: "Goiania", Pass: []string{"Anapolis", "Uberlandia"}, XDest: "Sao Paulo", XRota: "BR153",
				Entrega:  &Entrega{TpPer: "4", DIni: "2026-11-01", DFim: "2026-11-05", TpHor: "4", HIni: "08:00:00", HFim: "17:00:00"},
				OrigCalc: "Goiania", DestCalc: "Sao Paulo", XObs: "Observacao & <teste>",
				ObsCont: []Obs{{"campo&1", "texto 1"}}, ObsFisco: []Obs{{"campo2", "texto 2"}}}
		},
		"compl entrega na data": func(e *Entrada) {
			e.Compl = &Compl{Entrega: &Entrega{TpPer: "1", DProg: "2026-11-01", TpHor: "1", HProg: "10:00:00"}}
		},
		"compl entrega sem data/hora": func(e *Entrada) {
			e.Compl = &Compl{Entrega: &Entrega{TpPer: "0", TpHor: "0"}}
		},
		"retira com detalhe": func(e *Entrada) { e.Retira, e.XDetRetira = true, "Retirar no balcao" },
		"globalizado": func(e *Entrada) {
			e.IndGlobalizado, e.InfGlobal = true, "Operacao globalizada conforme regime"
		},
		"autXML, respTec e valores líquidos": func(e *Entrada) {
			e.AutXML = []string{"11222333000181", "12345678909"}
			e.RespTec = &RespTec{CNPJ: "11222333000181", Contato: "Maria", Email: "m@x.com.br", Fone: "62999990000"}
			e.VTPrestLiq = fp(900)
			e.ICMS = ICMS{SN: true}
		},
		"compra governamental e pagamento antecipado": func(e *Entrada) {
			e.CompraGov = &CompraGov{TpEnteGov: "1", TpOperGov: "1", PRedutor: 5, RefDFeAnt: []string{ch("9")}}
			e.TpPagAnt = "1"
			e.PagAntecipado = []string{ch("1"), ch("2")}
			e.PgtoVinc = []PagVinc{{"1", "ID-TRANS-1", "17", "11222333000181", "11222333"}}
		},
		"URL de QR Code de outra UF": func(e *Entrada) { e.URLQRCode = "https://cte.fazenda.mg.gov.br/portalcte/sistema/qrcode.xhtml" },
		"contingência SVC-RS (7)": func(e *Entrada) {
			e.TpEmis, e.DhCont, e.XJust = "7", &sub, "SEFAZ de origem fora do ar por mais de uma hora"
		},
		"contingência SVC-SP (8)": func(e *Entrada) {
			e.TpEmis, e.DhCont, e.XJust = "8", &sub, "SEFAZ de origem fora do ar por mais de uma hora"
		},
		"contingência offline (2) com sign": func(e *Entrada) {
			e.TpEmis, e.DhCont, e.XJust = "2", &sub, "Emissao offline por falha de conexao"
			e.QRSign = "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo="
		},
		"EPEC (4)": func(e *Entrada) {
			e.TpEmis, e.DhCont, e.XJust = "4", &sub, "EPEC em contingencia de autorizacao"
		},
		"regime especial NFF (3)": func(e *Entrada) {
			e.TpEmis, e.DhCont, e.XJust = "3", &sub, "Regime especial NFF autorizado"
			e.InfSolicNFF = "Solicitacao de emissao em regime especial NFF"
		},
		"emitente CPF e produção sem IBSCBS": func(e *Entrada) {
			e.TpAmb = "1"
			e.Emitente.CNPJ, e.Emitente.CPF = "", "12345678909"
			e.Emitente.CRT = "3"
			e.ICMS = ICMS{CST: "00", VBC: 1000, PICMS: 12, VICMS: 120}
		},
	}
	for nome, mut := range variantes {
		t.Run(nome, func(t *testing.T) {
			e := entradaBase()
			mut(&e)
			x, c, err := Build(e)
			if err != nil {
				t.Fatal(err)
			}
			if len(c) != 44 || !strings.Contains(string(x), `Id="CTe`+c+`"`) {
				t.Fatalf("chave/Id inconsistentes: %s", c)
			}
			validarXSD(t, x)
		})
	}
}

// CNPJ alfanumérico (NT 2025.001): o XSD (TCnpj/TChDFe) já aceita [A-Z0-9] nas
// 12 primeiras posições; emitente, chave de NF-e vinculada e autor de evento passam.
func TestXSD_CNPJAlfanumerico(t *testing.T) {
	const cnpj = "AB12CD34EF5678"
	e := entradaBase()
	e.Emitente.CNPJ = cnpj
	e.Docs.NFe = []DocNFe{{Chave: builder.NovaChave("GO", cnpj, "1", "7", "1", "55", e.DhEmi).String()}}
	x, chave, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if chave[6:20] != cnpj {
		t.Fatalf("chave sem o CNPJ alfanumérico: %s", chave)
	}
	validarXSD(t, x)
	ev, err := (&Cliente{tpAmb: "2", agora: time.Now}).montarEvento(certSF(t), chave, "110111", 1,
		`<evCancCTe><descEvento>Cancelamento</descEvento><nProt>`+nProtSF+`</nProt><xJust>Cancelamento por erro de digitacao</xJust></evCancCTe>`, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ev), "<CNPJ>"+cnpj+"</CNPJ>") {
		t.Errorf("autor do evento deveria ser %s:\n%s", cnpj, ev)
	}
}
