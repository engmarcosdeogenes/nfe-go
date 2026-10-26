package cte

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CT-e OS contra cteOS_v4.00.xsd; infModal é xs:any, então <rodoOS> é validado
// à parte contra cteModalRodoviarioOS_v4.00.xsd. Falha, não pula, sem python+lxml.
const scriptXSDOS = `
import sys
from lxml import etree
d = sys.argv[1]
sch = etree.XMLSchema(etree.parse(d + '/cteOS_v4.00.xsd'))
doc = etree.parse(sys.argv[2])
sch.validate(doc)
for e in sch.error_log:
    print('cteOS', e.line, e.message)
ns = {'m': 'http://www.portalfiscal.inf.br/cte'}
rodo = doc.find('.//m:rodoOS', ns)
if rodo is not None:
    rs = etree.XMLSchema(etree.parse(d + '/cteModalRodoviarioOS_v4.00.xsd'))
    rs.validate(etree.ElementTree(rodo))
    for e in rs.error_log:
        print('rodoOS', e.line, e.message)
`

func validarXSDOS(t *testing.T, x []byte) {
	t.Helper()
	py := pythonLxml(t)
	com := string(x)
	if !strings.Contains(com, "<Signature") {
		com = strings.Replace(com, "</CTeOS>", sigDummy+"</CTeOS>", 1)
	}
	arq := filepath.Join(t.TempDir(), "cteos.xml")
	if err := os.WriteFile(arq, []byte(com), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-c", scriptXSDOS, filepath.Join("..", "testdata", "schemas", "cte"), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("XSD:\n%s\n%s", s, x)
	}
}

func endTomaOS() Endereco {
	return Endereco{Logradouro: "Rua D", Numero: "40", Bairro: "Centro", CodigoMun: "5208707", Municipio: "Goiania", CEP: "74000000", UF: "GO"}
}

// entradaOSBase: passageiros, fretamento eventual, Simples Nacional.
func entradaOSBase() EntradaOS {
	viagem := time.Date(2026, 11, 5, 8, 0, 0, 0, time.FixedZone("x", -3*3600))
	return EntradaOS{
		TpAmb: "2", Serie: "1", NCT: "77", DhEmi: time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC),
		CFOP: "5353", NatOp: "Transporte de passageiros", TpCTe: "0", TpServ: "6", Modal: "01",
		Envio:     Local{"5208707", "Goiania", "GO"},
		Inicio:    &Local{"5208707", "Goiania", "GO"},
		Fim:       &Local{"5300108", "Brasilia", "DF"},
		IndIEToma: "1",
		Toma: &Terceiro{CNPJ: "44555666000177", IE: "110220330", Nome: "Empresa Tomadora", Fone: "6233334444",
			Email: "t@x.com.br", End: endTomaOS()},
		Emitente: Emitente{CNPJ: "11222333000181", IE: "102030405", Nome: "Viacao Alfa", CRT: "1",
			End: Endereco{"Rua A", "10", "", "Centro", "5208707", "Goiania", "74000000", "GO", "", "", "62999990000"}},
		VTPrest: 2500, VRec: 2500, Comp: []Comp{{"Fretamento", 2500}},
		ICMS:    ICMS{SN: true, CST: "90"},
		Servico: &InfServicoOS{XDescServ: "Fretamento eventual", QCarga: fp(40)},
		RodoOS: &RodoviarioOS{TAF: "123456789012",
			Veic:       &VeicOS{Placa: "ABC1D23", RENAVAM: "12345678901", UF: "GO"},
			Fretamento: &Fretamento{TpFretamento: "1", DhViagem: &viagem}},
	}
}

func normalOS(e *EntradaOS) {
	e.Emitente.CRT = "3"
	e.ICMS = ICMS{CST: "00", VBC: 2500, PICMS: 12, VICMS: 300}
	e.IBSCBS = &IBSCBS{CST: "000", CClassTrib: "000001", G: &GIBSCBS{
		VBC: 2500, IBSUF: AliqRTC{P: 0.1, V: 2.5}, IBSMun: AliqRTC{}, VIBS: 2.5, CBS: AliqRTC{P: 0.9, V: 22.5}}}
}

func valoresOS(e *EntradaOS) {
	e.TpServ, e.CFOP = "7", "5353"
	e.NatOp = "Transporte de valores"
	e.Servico = &InfServicoOS{XDescServ: "Transporte de valores", QCarga: fp(3)}
	e.RodoOS = nil
}

func TestXSD_OS_Variantes(t *testing.T) {
	sub := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	variantes := map[string]func(*EntradaOS){
		"passageiros fretamento eventual (Simples)": func(e *EntradaOS) {},
		"passageiros fretamento contínuo com registro estadual": func(e *EntradaOS) {
			e.RodoOS.TAF, e.RodoOS.NroRegEstadual = "", "1234567890123456789012345"
			e.RodoOS.Fretamento = &Fretamento{TpFretamento: "2"}
		},
		"passageiros regular com bilhetes e seguro": func(e *EntradaOS) {
			e.RodoOS.Fretamento = nil
			e.Servico.XDescServ = "Linha regular"
			e.DocRef = []DocRefOS{{ChBPe: ch("6")}, {NDoc: "123", Serie: "1", Subserie: "A", DEmi: "2026-10-20", VDoc: fp(80)}}
			e.Seg = []SegOS{{RespSeg: "4", XSeg: "Seguradora X", NApol: "AP-1"}, {RespSeg: "5"}}
		},
		"passageiros veículo de terceiro": func(e *EntradaOS) {
			e.RodoOS.Veic.Prop = &PropVeicOS{CPF: "12345678909", NroRegEstadual: "1234567890123456789012345",
				Nome: "Proprietario Autonomo", IE: "ISENTO", UF: "GO", TpProp: "1"}
		},
		"valores com GTV-e e cobrança": func(e *EntradaOS) {
			valoresOS(e)
			e.GTVe = []InfGTVe{{Chave: ch("6"), Comp: []CompGTVe{{"1", 10, ""}, {"6", 5, "Outros"}}}}
			e.Cobr = &Cobr{NFat: "F-1", VOrig: fp(2500), Dup: []Dup{{"001", "2026-11-26", fp(2500)}}}
		},
		"valores com rodoOS": func(e *EntradaOS) {
			valoresOS(e)
			e.RodoOS = &RodoviarioOS{NroRegEstadual: "1234567890123456789012345"}
		},
		"excesso de bagagem sem tomador": func(e *EntradaOS) {
			e.TpServ, e.Toma, e.IndIEToma = "8", nil, "9"
			e.Servico = &InfServicoOS{XDescServ: "Excesso de bagagem", QCarga: fp(2)}
			e.RodoOS.Fretamento = nil
		},
		"excesso de bagagem com tomador CPF": func(e *EntradaOS) {
			e.TpServ, e.IndIEToma = "8", "9"
			e.Toma = &Terceiro{CPF: "12345678909", Nome: "Passageiro", End: endTomaOS()}
			e.RodoOS.Fretamento = nil
		},
		"regime normal ICMS00 com IBS/CBS": normalOS,
		"ICMS20": func(e *EntradaOS) {
			normalOS(e)
			e.ICMS = ICMS{CST: "20", PRedBC: 60, VBC: 1000, PICMS: 12, VICMS: 120, VICMSDeson: 180, CBenef: "GO000001"}
		},
		"ICMS45": func(e *EntradaOS) { normalOS(e); e.ICMS = ICMS{CST: "40"} },
		"ICMS90 e OutraUF": func(e *EntradaOS) {
			normalOS(e)
			e.ICMS = ICMS{CST: "90", OutraUF: true, PRedBC: 50, VBC: 500, PICMS: 12, VICMS: 60}
		},
		"partilha, tributos e pagamento vinculado": func(e *EntradaOS) {
			normalOS(e)
			e.ICMSUFFim = &ICMSUFFim{2500, 2, 18, 12, 50, 150, 0}
			e.VTotTrib, e.InfAdFisco = fp(100), "Partilha EC 87/2015"
			e.PgtoVinc = []PagVinc{{"1", "ID-TRANS-1", "17", "11222333000181", "11222333"}}
		},
		"tributos federais retidos": func(e *EntradaOS) {
			normalOS(e)
			e.TribFed = &TribFed{VPIS: fp(16.25), VCOFINS: fp(75), VIR: fp(25), VINSS: fp(0), VCSLL: fp(5)}
		},
		"complemento": func(e *EntradaOS) {
			e.TpCTe = "1"
			e.Servico, e.RodoOS = nil, nil
			e.InfCteComp = []string{ch("6"), ch("7")}
			e.VTPrest, e.VRec, e.Comp = 100, 100, []Comp{{"Complemento", 100}}
		},
		"substituição": func(e *EntradaOS) {
			e.TpCTe, e.InfCteSub = "3", ch("7")
		},
		"substituição de valores com refCTeCanc": func(e *EntradaOS) {
			valoresOS(e)
			e.TpCTe, e.InfCteSub, e.RefCTeCanc = "3", ch("7"), ch("8")
		},
		"percurso, compl, autXML e respTec": func(e *EntradaOS) {
			e.Percurso = []string{"TO", "MG"}
			e.Compl = &Compl{XCaracAd: "Urgente", XCaracSer: "Executivo", XEmi: "Joao", XObs: "Obs & <teste>",
				ObsCont: []Obs{{"campo&1", "texto 1"}}, ObsFisco: []Obs{{"campo2", "texto 2"}}}
			e.AutXML = []string{"11222333000181", "12345678909"}
			e.RespTec = &RespTec{CNPJ: "11222333000181", Contato: "Maria", Email: "m@x.com.br", Fone: "62999990000"}
			e.VTPrestLiq = fp(2400)
		},
		"sem início e fim da prestação": func(e *EntradaOS) { e.Inicio, e.Fim = nil, nil },
		"toma exterior sem documento": func(e *EntradaOS) {
			e.IndIEToma = "9"
			e.Toma = &Terceiro{Nome: "Foreign Co", End: Endereco{Logradouro: "Main St", Numero: "1", Bairro: "Downtown",
				CodigoMun: "9999999", Municipio: "EXTERIOR", UF: "EX", CodPais: "1058", NomePais: "Estados Unidos"}}
		},
		"compra governamental": func(e *EntradaOS) {
			e.CompraGov = &CompraGov{TpEnteGov: "1", TpOperGov: "1", PRedutor: 5, RefDFeAnt: []string{ch("9")}}
			e.TpPagAnt, e.PagAntecipado = "1", []string{ch("1")}
		},
		"contingência SVC-RS (7)": func(e *EntradaOS) {
			e.TpEmis, e.DhCont, e.XJust = "7", &sub, "SEFAZ de origem fora do ar por mais de uma hora"
		},
		"contingência offline (2) com sign": func(e *EntradaOS) {
			e.TpEmis, e.DhCont, e.XJust = "2", &sub, "Emissao offline por falha de conexao"
			e.QRSign = "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo="
		},
	}
	for nome, mut := range variantes {
		t.Run(nome, func(t *testing.T) {
			e := entradaOSBase()
			mut(&e)
			x, c, err := BuildOS(e)
			if err != nil {
				t.Fatal(err)
			}
			if len(c) != 44 || c[20:22] != "67" || !strings.Contains(string(x), `Id="CTe`+c+`"`) || !strings.HasPrefix(string(x), "<CTeOS ") {
				t.Fatalf("chave/Id/raiz inconsistentes: %s", c)
			}
			validarXSDOS(t, x)
		})
	}
}
