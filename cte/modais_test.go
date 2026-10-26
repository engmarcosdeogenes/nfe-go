package cte

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Valida o CT-e inteiro e o grupo do modal à parte contra o XSD do próprio
// modal (infModal é xs:any no cte_v4.00.xsd).
const scriptXSDModal = `
import sys
from lxml import etree
d = sys.argv[1]
doc = etree.parse(sys.argv[2])
sch = etree.XMLSchema(etree.parse(d + '/cte_v4.00.xsd'))
sch.validate(doc)
for e in sch.error_log:
    print('cte', e.line, e.message)
ns = {'m': 'http://www.portalfiscal.inf.br/cte'}
for tag, xsd in [('aereo','cteModalAereo'),('aquav','cteModalAquaviario'),('ferrov','cteModalFerroviario'),('duto','cteModalDutoviario'),('multimodal','cteMultiModal')]:
    el = doc.find('.//m:' + tag, ns)
    if el is not None:
        rs = etree.XMLSchema(etree.parse(d + '/' + xsd + '_v4.00.xsd'))
        rs.validate(etree.ElementTree(el))
        for e in rs.error_log:
            print(tag, e.line, e.message)
`

func validarModal(t *testing.T, x []byte, tag string) {
	t.Helper()
	if !strings.Contains(string(x), "<"+tag+">") {
		t.Fatalf("XML sem <%s>", tag)
	}
	com := strings.Replace(string(x), "</CTe>", sigDummy+"</CTe>", 1)
	arq := filepath.Join(t.TempDir(), "cte.xml")
	if err := os.WriteFile(arq, []byte(com), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(pythonLxml(t), "-c", scriptXSDModal, filepath.Join("..", "testdata", "schemas", "cte"), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("XSD:\n%s\n%s", s, x)
	}
}

func modalBase(m string) Entrada {
	e := entradaBase()
	e.Modal, e.Rodo = m, nil
	switch m {
	case "02":
		e.Aereo = &Aereo{DPrevAereo: "2026-11-01", Tarifa: &TarifaAerea{CL: "G", VTar: 12.5}}
	case "03":
		e.Aquaviario = &Aquaviario{VPrest: 100, VAFRMM: 10, XNavio: "MV Alfa", Direc: "N", Irin: "ABC123"}
	case "04":
		e.Ferroviario = &Ferroviario{TpTraf: "0", Fluxo: "FLX001"}
	case "05":
		e.Dutoviario = &Dutoviario{DIni: "2026-10-01", DFim: "2026-10-31"}
	case "06":
		e.Multimodal = &Multimodal{COTM: "COTM123", IndNegociavel: "0"}
	}
	return e
}

func TestModal_XSD(t *testing.T) {
	vf := fp(0.5)
	u := fp(12.5)
	v := map[string]struct {
		tag string
		mut func(*Entrada)
	}{
		"aéreo mínimo": {"aereo", func(e *Entrada) { *e = modalBase("02") }},
		"aéreo completo com perigosa": {"aereo", func(e *Entrada) {
			*e = modalBase("02")
			*e.Aereo = Aereo{NMinu: "123456789", NOCA: "12345678901", DPrevAereo: "2026-11-01", XDime: "10X20X30", CInfManu: []string{"02", "99"},
				Tarifa: &TarifaAerea{CL: "E", CTar: "AB12", VTar: 99.99},
				Peri:   []PeriAereo{{NONU: "1234", QTotEmb: "2 caixas", QTotProd: 10.5, UniAP: "2"}, {NONU: "ND", QTotEmb: "1", QTotProd: 1, UniAP: "5"}}}
		}},
		"aquaviário mínimo": {"aquav", func(e *Entrada) { *e = modalBase("03") }},
		"aquaviário completo com contêiner NF-e": {"aquav", func(e *Entrada) {
			*e = modalBase("03")
			*e.Aquaviario = Aquaviario{VPrest: 100, VAFRMM: 0, XNavio: "MV Alfa", Balsa: []string{"B1", "B2"}, NViag: "123", Direc: "L", Irin: "ABC123", TpNav: "1",
				DetCont: []DetCont{{NCont: "ABCU1234567", Lacres: []string{"L1", "L2"}, NFe: []ContNFe{{Chave: ch("1"), UnidRat: u}}}}}
		}},
		"aquaviário contêiner NF papel": {"aquav", func(e *Entrada) {
			*e = modalBase("03")
			e.Aquaviario.DetCont = []DetCont{{NCont: "TEMU7654321", NF: []ContNF{{Serie: "1", NDoc: "99", UnidRat: u}}}}
		}},
		"ferroviário próprio": {"ferrov", func(e *Entrada) { *e = modalBase("04") }},
		"ferroviário tráfego mútuo": {"ferrov", func(e *Entrada) {
			*e = modalBase("04")
			e.Ferroviario.TpTraf = "1"
			e.Ferroviario.TrafMut = &TrafMut{RespFat: "1", FerrEmi: "2", VFrete: 250.75, ChCTeFerroOrigem: ch("3"),
				FerroEnv: []FerroEnv{{CNPJ: "11222333000181", CInt: "F1", IE: "123456", Nome: "Ferrovia Beta",
					End: EnderFerro{"Av Trilhos", "1", "Pátio", "Centro", "3550308", "Sao Paulo", "01310100", "SP"}}}}
		}},
		"dutoviário mínimo": {"duto", func(e *Entrada) { *e = modalBase("05") }},
		"dutoviário completo": {"duto", func(e *Entrada) {
			*e = modalBase("05")
			*e.Dutoviario = Dutoviario{VTar: vf, DIni: "2026-10-01", DFim: "2026-10-31", ClassDuto: "2", TpContratacao: "1",
				CodPontoEntrada: "PE01", CodPontoSaida: "PS01", NContrato: "CT-2026"}
		}},
		"multimodal mínimo": {"multimodal", func(e *Entrada) { *e = modalBase("06") }},
		"multimodal com seguro": {"multimodal", func(e *Entrada) {
			*e = modalBase("06")
			e.Multimodal = &Multimodal{COTM: "COTM 99", IndNegociavel: "1", Seg: &SegMulti{XSeg: "Seguradora Z", CNPJ: "11222333000181", NApol: "AP1", NAver: "AV1"}}
		}},
		"multimodal seguro sem CNPJ": {"multimodal", func(e *Entrada) {
			*e = modalBase("06")
			e.Multimodal.Seg = &SegMulti{XSeg: "Seguradora Z", NApol: "AP1", NAver: "AV1"}
		}},
	}
	for nome, c := range v {
		t.Run(nome, func(t *testing.T) {
			var e Entrada
			c.mut(&e)
			x, _, err := Build(e)
			if err != nil {
				t.Fatal(err)
			}
			validarModal(t, x, c.tag)
		})
	}
}

func TestModal_Erros(t *testing.T) {
	casos := []struct {
		nome, modal, quer string
		mut               func(*Entrada)
	}{
		{"02 sem grupo", "02", "exige o grupo Aereo", func(e *Entrada) { e.Aereo = nil }},
		{"03 sem grupo", "03", "exige o grupo Aquaviario", func(e *Entrada) { e.Aquaviario = nil }},
		{"04 sem grupo", "04", "exige o grupo Ferroviario", func(e *Entrada) { e.Ferroviario = nil }},
		{"05 sem grupo", "05", "exige o grupo Dutoviario", func(e *Entrada) { e.Dutoviario = nil }},
		{"06 sem grupo", "06", "exige o grupo Multimodal", func(e *Entrada) { e.Multimodal = nil }},
		{"grupo de outro modal", "02", "só o grupo do modal", func(e *Entrada) { e.Rodo = &Rodoviario{RNTRC: "12345678"} }},
		{"aéreo nMinu curto", "02", "nMinu", func(e *Entrada) { e.Aereo.NMinu = "123" }},
		{"aéreo nOCA curto", "02", "nOCA", func(e *Entrada) { e.Aereo.NOCA = "123" }},
		{"aéreo sem dPrevAereo", "02", "dPrevAereo", func(e *Entrada) { e.Aereo.DPrevAereo = "" }},
		{"aéreo sem tarifa", "02", "tarifa", func(e *Entrada) { e.Aereo.Tarifa = nil }},
		{"aéreo CL inválido", "02", "CL", func(e *Entrada) { e.Aereo.Tarifa.CL = "X" }},
		{"aéreo cTar longo", "02", "cTar", func(e *Entrada) { e.Aereo.Tarifa.CTar = "12345" }},
		{"aéreo xDime curto", "02", "xDime", func(e *Entrada) { e.Aereo.XDime = "1x2" }},
		{"aéreo cInfManu inválido", "02", "cInfManu", func(e *Entrada) { e.Aereo.CInfManu = []string{"16"} }},
		{"aéreo nONU inválido", "02", "nONU", func(e *Entrada) { e.Aereo.Peri = []PeriAereo{{"12", "1", 1, "1"}} }},
		{"aéreo uniAP inválido", "02", "uniAP", func(e *Entrada) { e.Aereo.Peri = []PeriAereo{{"1234", "1", 1, "9"}} }},
		{"aquav sem xNavio", "03", "xNavio", func(e *Entrada) { e.Aquaviario.XNavio = "" }},
		{"aquav direc inválida", "03", "direc", func(e *Entrada) { e.Aquaviario.Direc = "X" }},
		{"aquav sem irin", "03", "irin", func(e *Entrada) { e.Aquaviario.Irin = "" }},
		{"aquav 4 balsas", "03", "balsa", func(e *Entrada) { e.Aquaviario.Balsa = []string{"a", "b", "c", "d"} }},
		{"aquav nViag zero", "03", "nViag", func(e *Entrada) { e.Aquaviario.NViag = "0123" }},
		{"aquav tpNav inválido", "03", "tpNav", func(e *Entrada) { e.Aquaviario.TpNav = "2" }},
		{"aquav vAFRMM negativo", "03", "vAFRMM", func(e *Entrada) { e.Aquaviario.VAFRMM = -1 }},
		{"aquav contêiner minúsculo", "03", "nCont", func(e *Entrada) { e.Aquaviario.DetCont = []DetCont{{NCont: "abc"}} }},
		{"aquav 4 lacres", "03", "lacre", func(e *Entrada) {
			e.Aquaviario.DetCont = []DetCont{{NCont: "ABC1", Lacres: []string{"1", "2", "3", "4"}}}
		}},
		{"aquav NF e NFe juntas", "03", "infNF ou infNFe", func(e *Entrada) {
			e.Aquaviario.DetCont = []DetCont{{NCont: "ABC1", NF: []ContNF{{"1", "1", nil}}, NFe: []ContNFe{{ch("1"), nil}}}}
		}},
		{"ferrov tpTraf inválido", "04", "tpTraf", func(e *Entrada) { e.Ferroviario.TpTraf = "9" }},
		{"ferrov sem fluxo", "04", "fluxo", func(e *Entrada) { e.Ferroviario.Fluxo = "" }},
		{"ferrov respFat inválido", "04", "respFat", func(e *Entrada) { e.Ferroviario.TrafMut = &TrafMut{RespFat: "3", FerrEmi: "1"} }},
		{"ferrov ferrEmi inválido", "04", "ferrEmi", func(e *Entrada) { e.Ferroviario.TrafMut = &TrafMut{RespFat: "1", FerrEmi: "0"} }},
		{"ferrov chave origem curta", "04", "chCTeFerroOrigem", func(e *Entrada) {
			e.Ferroviario.TrafMut = &TrafMut{RespFat: "1", FerrEmi: "1", ChCTeFerroOrigem: "12"}
		}},
		{"ferrov ferroEnv sem CNPJ", "04", "ferroEnv", func(e *Entrada) {
			e.Ferroviario.TrafMut = &TrafMut{RespFat: "1", FerrEmi: "1", FerroEnv: []FerroEnv{{Nome: "X"}}}
		}},
		{"duto sem dIni", "05", "dIni", func(e *Entrada) { e.Dutoviario.DIni = "" }},
		{"duto dFim antes de dIni", "05", "dFim", func(e *Entrada) { e.Dutoviario.DFim = "2026-09-01" }},
		{"duto vTar zero", "05", "vTar", func(e *Entrada) { e.Dutoviario.VTar = fp(0) }},
		{"duto classDuto inválida", "05", "classDuto", func(e *Entrada) { e.Dutoviario.ClassDuto = "4" }},
		{"duto tpContratacao inválido", "05", "tpContratacao", func(e *Entrada) { e.Dutoviario.TpContratacao = "3" }},
		{"duto codPontoEntrada curto", "05", "codPontoEntrada", func(e *Entrada) { e.Dutoviario.CodPontoEntrada = "A" }},
		{"duto nContrato longo", "05", "nContrato", func(e *Entrada) { e.Dutoviario.NContrato = strings.Repeat("A", 21) }},
		{"multimodal sem COTM", "06", "COTM", func(e *Entrada) { e.Multimodal.COTM = "" }},
		{"multimodal COTM longo", "06", "COTM", func(e *Entrada) { e.Multimodal.COTM = strings.Repeat("A", 21) }},
		{"multimodal indNegociavel", "06", "indNegociavel", func(e *Entrada) { e.Multimodal.IndNegociavel = "2" }},
		{"multimodal seguro sem apólice", "06", "nApol", func(e *Entrada) { e.Multimodal.Seg = &SegMulti{XSeg: "S", NAver: "1"} }},
		{"multimodal seguradora CNPJ ruim", "06", "seguradora", func(e *Entrada) { e.Multimodal.Seg = &SegMulti{XSeg: "S", CNPJ: "123", NApol: "1", NAver: "1"} }},
		{"multimodal com tpServ 4", "06", "tpServ 4", func(e *Entrada) {
			e.TpServ = "4"
			e.InfServVinc = []string{ch("4")}
		}},
		{"complemento com grupo aéreo", "02", "não leva infCTeNorm", func(e *Entrada) {
			e.TpCTe, e.Carga, e.Docs = "1", nil, Docs{}
			e.InfCteComp = []string{ch("6")}
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := modalBase(c.modal)
			c.mut(&e)
			_, _, err := Build(e)
			if err == nil || !strings.Contains(err.Error(), c.quer) {
				t.Fatalf("esperava erro com %q, veio %v", c.quer, err)
			}
		})
	}
}
