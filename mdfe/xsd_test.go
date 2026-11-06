package mdfe

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Valida o XML de cada variante contra o XSD oficial (leiaute 3.00). O XSD
// mantém infModal como xs:any, então o <rodo> é validado à parte contra
// mdfeModalRodoviario. Signature (obrigatória em TMDFe) é injetada dummy,
// estruturalmente válida. Falha, não pula, se não houver python+lxml.
const scriptXSD = `
import sys
from lxml import etree
d = sys.argv[1]
sch = etree.XMLSchema(etree.parse(d + '/mdfe_v3.00.xsd'))
doc = etree.parse(sys.argv[2])
sch.validate(doc)
for e in sch.error_log:
    print('mdfe', e.line, e.message)
ns = {'m': 'http://www.portalfiscal.inf.br/mdfe'}
rodo = doc.find('.//m:rodo', ns)
rs = etree.XMLSchema(etree.parse(d + '/mdfeModalRodoviario_v3.00.xsd'))
rs.validate(etree.ElementTree(rodo))
for e in rs.error_log:
    print('rodo', e.line, e.message)
`

const sigDummy = `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignedInfo>` +
	`<CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>` +
	`<SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/>` +
	`<Reference URI="#MDFe0"><Transforms><Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/>` +
	`<Transform Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/></Transforms>` +
	`<DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/><DigestValue>AAAA</DigestValue></Reference></SignedInfo>` +
	`<SignatureValue>AAAA</SignatureValue><KeyInfo><X509Data><X509Certificate>AAAA</X509Certificate></X509Data></KeyInfo></Signature>`

func validarXSD(t *testing.T, x []byte) {
	// PYTHON força o interpretador (útil quando o PATH tem aspas soltas e o
	// LookPath do Go não enxerga o python).
	py := ""
	for _, c := range []string{os.Getenv("PYTHON"), "python3", "python"} {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil && exec.Command(p, "-c", "import lxml").Run() == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Fatal("python com lxml é necessário para validar o XSD (pip install lxml)")
	}
	com := strings.Replace(string(x), "</MDFe>", sigDummy+"</MDFe>", 1)
	arq := filepath.Join(t.TempDir(), "mdfe.xml")
	if err := os.WriteFile(arq, []byte(com), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-c", scriptXSD, filepath.Join("..", "testdata", "schemas", "mdfe"), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("XSD:\n%s\n%s", s, x)
	}
}

func TestXSD_VariantesDoBuilder(t *testing.T) {
	prop := &Proprietario{CNPJ: "11222333000181", RNTRC: "87654321", Nome: "Transportes Beta", IE: "109876543", UF: "GO", TpProp: "2"}
	ini := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	variantes := map[string]func(*Entrada){
		"1 NF-e com lotação": func(e *Entrada) {},
		"várias NF-e em 2 municípios": func(e *Entrada) {
			e.Descargas = []Descarga{
				{CodigoMun: "5300108", Nome: "Brasília", NFes: []string{chaveNFe("1"), chaveNFe("2")}},
				{CodigoMun: "5201108", Nome: "Anápolis", NFes: []string{chaveNFe("3")}},
			}
			e.Lacres = []string{"L-001", "L-002"}
			e.AutXML = []string{"11222333000181", "12345678909"}
			e.InfCpl = "Transporte de carga própria"
			e.InfAdFisco = "Nada a declarar"
			e.RespTec = &RespTec{CNPJ: "11222333000181", Contato: "Maria", Email: "m@x.com.br", Fone: "62999990000"}
		},
		"reboque com proprietário": func(e *Entrada) {
			e.Rodo.Reboques = []Veiculo{{Placa: "XYZ-9876", Renavam: "123456789", TaraKG: 5000, CapKG: 25000, CapM3: 60, TpCar: "00", UF: "GO", Prop: prop}}
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Autônomo", TpProp: "1"}
			e.Rodo.Tracao.CInt = "T1"
			e.Descargas[0].NFes = append(e.Descargas[0].NFes, chaveNFe("9"))
		},
		"percurso GO-MG-SP": func(e *Entrada) {
			e.UFFim = "SP"
			e.UFPercurso = []string{"MG"}
			e.DhIniViagem = &ini
			e.Descargas = []Descarga{{CodigoMun: "3550308", Nome: "São Paulo", NFes: []string{chaveNFe("4"), chaveNFe("5")}}}
			e.TpAmb = "1"
		},
		"tpEmit 1 completo": func(e *Entrada) {
			e.TpEmit = "1"
			e.TpTransp = "1"
			e.ExigirCIOT = true
			e.Rodo.Tracao.Prop = prop
			e.Rodo.CIOTs = []CIOT{{CIOT: "123456789012", CPF: "12345678909"}, {CNPJ: "11222333000181"}}
			e.Rodo.ValePed = &ValePed{Disp: []ValeDisp{{CNPJForn: "11222333000181", CNPJPg: "11222333000181", NCompra: "99", VValePed: 150.5, TpValePed: "01"}}, CategCombVeic: "02"}
			e.Rodo.Contratantes = []Contratante{
				{Nome: "Contratante SA", CNPJ: "11222333000181", NroContrato: "CT-01", VContratoGlobal: 5000},
				{IDEstrangeiro: "AB123456"},
			}
			e.Rodo.Pagamentos = []Pagamento{
				{Nome: "Autônomo", CPF: "12345678909", Comp: []CompPag{{Tipo: "01", Valor: 900}, {Tipo: "99", Valor: 100, XComp: "Outros"}},
					VContrato: 1000, IndAltoDesemp: true, IndPag: "1", VAdiant: 300, IndAntecipaAdiant: true,
					Prazos:    []Prazo{{1, "2026-12-01", 350}, {2, "2027-01-01", 350}},
					TpAntecip: "1", PIX: "chave@pix.com"},
				{CNPJ: "11222333000181", Comp: []CompPag{{Tipo: "02", Valor: 10}}, VContrato: 10, IndPag: "0", CodBanco: "001", CodAgencia: "1234"},
				{IDEstrangeiro: "PASS12345", Comp: []CompPag{{Tipo: "03", Valor: 10}}, VContrato: 10, IndPag: "0", CNPJIPEF: "11222333000181"},
			}
			e.Seguros = []Seguro{
				{RespSeg: "1", RespCNPJ: "11222333000181", Seguradora: "Seguradora X", CNPJSeg: "11222333000181", NApol: "AP1", NAver: []string{"AV1", "AV2"}},
				{RespSeg: "2", RespCPF: "12345678909"},
			}
			e.Descargas = []Descarga{
				{CodigoMun: "5300108", Nome: "Brasília", CTes: []string{chaveNFe("1")}, NFes: []string{chaveNFe("2")}, MDFes: []string{chaveNFe("3")},
					Opc: map[string]DocOpc{chaveNFe("x"): {}}},
			}
			e.Descargas[0].Opc = map[string]DocOpc{e.Descargas[0].CTes[0]: {SegCodBarra: "123456789012345678901234567890123456", IndReentrega: true}, e.Descargas[0].MDFes[0]: {IndReentrega: true}}
		},
		"tpEmit 3 com NF-e": func(e *Entrada) {
			e.TpEmit = "3"
			e.Seguros = []Seguro{{RespSeg: "1", RespCNPJ: "11222333000181"}}
			e.Descargas[0].NFes = append(e.Descargas[0].NFes, chaveNFe("6"))
		},
		"tpEmis 2 contingência": func(e *Entrada) {
			e.TpEmis, e.QRSign = "2", "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo="
		},
		"tpEmit 2 com tpTransp TAC": func(e *Entrada) {
			e.TpTransp = "2"
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Autônomo", TpProp: "1"}
			e.Rodo.Pagamentos = []Pagamento{pagPIX()}
			e.Rodo.CIOTs = []CIOT{{CIOT: "123456789012", CPF: "12345678909"}}
		},
		"emitente CPF sem RNTRC": func(e *Entrada) {
			e.Emitente.CNPJ, e.Emitente.CPF = "", "12345678909"
			e.Rodo.RNTRC = ""
			e.Descargas[0].NFes = append(e.Descargas[0].NFes, chaveNFe("8"))
		},
	}
	for nome, mut := range variantes {
		t.Run(nome, func(t *testing.T) {
			e := entradaBase()
			mut(&e)
			x, _, err := Build(e)
			if err != nil {
				t.Fatal(err)
			}
			validarXSD(t, x)
		})
	}
}
