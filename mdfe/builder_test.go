package mdfe

import (
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// chaveNFe gera uma chave de NF-e válida (mod 55) para os testes.
func chaveNFe(n string) string {
	return builder.NovaChave("GO", "11222333000181", "1", n, "1", "55",
		time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)).String()
}

func entradaBase() Entrada {
	return Entrada{
		TpAmb: "2", TpEmit: "2", Serie: "1", NMDF: "42",
		DhEmi: time.Date(2026, 10, 26, 15, 30, 0, 0, time.UTC), TpEmis: "1",
		UFIni: "GO", UFFim: "GO",
		MunCarrega: []Municipio{{"5208707", "Goiânia"}},
		Emitente: Emitente{
			CNPJ: "11.222.333/0001-81", IE: "10.987.654-3", Nome: "Metalúrgica Exemplo Ltda",
			End: Endereco{Logradouro: "Rua A", Numero: "10", Bairro: "Centro", CodigoMun: "5208707",
				Municipio: "Goiânia", CEP: "74000-000", UF: "GO", Fone: "6232221111", Email: "a@b.com.br"},
		},
		Rodo: Rodoviario{
			RNTRC:      "12345678",
			Tracao:     Veiculo{Placa: "abc-1d23", TaraKG: 8000, CapKG: 20000, TpRod: "03", TpCar: "02", UF: "GO"},
			Condutores: []Condutor{{"João da Silva", "123.456.789-09"}},
		},
		Descargas: []Descarga{{CodigoMun: "5300108", Nome: "Brasília", NFes: []string{chaveNFe("1")}}},
		ProdPred: &ProdutoPredominante{TpCarga: "05", XProd: "Peças metálicas", NCM: "73089090",
			CEPCarrega: "74000-000", CEPDescarrega: "70040-010"},
		VCarga: 12345.6, CUnid: "01", QCarga: 1500.5,
	}
}

func TestBuild_RegrasFixas(t *testing.T) {
	x, ch, err := Build(entradaBase())
	if err != nil {
		t.Fatal(err)
	}
	s := string(x)
	// Chave: 44 dígitos, cUF GO=52, AAMM no fuso da UF, mod 58 nas posições 20-21, DV conferido.
	if len(ch) != 44 || ch[:2] != "52" || ch[2:6] != "2610" || ch[20:22] != "58" {
		t.Fatalf("chave fora do padrão: %s", ch)
	}
	if got := dvMod11(ch[:43]); got != ch[43:] {
		t.Fatalf("DV %s, esperado %s", ch[43:], got)
	}
	for _, want := range []string{
		`<MDFe xmlns="http://www.portalfiscal.inf.br/mdfe">`,
		`<infMDFe versao="3.00" Id="MDFe` + ch + `">`,
		`<cMDF>` + ch[35:43] + `</cMDF>`, `<cDV>` + ch[43:] + `</cDV>`,
		`<mod>58</mod>`, `<modal>1</modal>`, `<procEmi>0</procEmi>`, `<verProc>nfe-go</verProc>`,
		`<dhEmi>2026-10-26T12:30:00-03:00</dhEmi>`, // 15:30Z em GO
		`<placa>ABC1D23</placa>`,
		`<qNFe>1</qNFe>`, `<vCarga>12345.60</vCarga>`, `<qCarga>1500.5000</qCarga>`,
		`<infLotacao>`, // 1 documento = lotação
		`<qrCodMDFe>https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe=` + ch + `&amp;tpAmb=2</qrCodMDFe>`,
		`<xNome>Metalúrgica Exemplo Ltda</xNome>`, // homologação não troca xNome
	} {
		if !strings.Contains(s, want) {
			t.Errorf("faltou %s\n%s", want, s)
		}
	}
	for _, no := range []string{"<tpTransp>", "<qCTe>", "<?xml", "<Signature"} {
		if strings.Contains(s, no) {
			t.Errorf("não devia conter %s", no)
		}
	}
	ord := []string{"<ide>", "<emit>", "<infModal", "<infDoc>", "<prodPred>", "<tot>", "</infMDFe>", "<infMDFeSupl>"}
	last := -1
	for _, o := range ord {
		i := strings.Index(s, o)
		if i <= last {
			t.Errorf("%s fora de ordem", o)
		}
		last = i
	}
}

func TestBuild_DocsVariosSemLotacao(t *testing.T) {
	e := entradaBase()
	e.Descargas = []Descarga{
		{CodigoMun: "5300108", Nome: "Brasília", NFes: []string{chaveNFe("1"), chaveNFe("2")}},
		{CodigoMun: "5208707", Nome: "Goiânia", NFes: []string{chaveNFe("3")}},
	}
	x, _, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(x), "<qNFe>3</qNFe>") || strings.Contains(string(x), "infLotacao") {
		t.Fatalf("totalizador/lotação errados: %s", x)
	}
}

func TestBuild_Erros(t *testing.T) {
	casos := map[string]func(*Entrada){
		"sem condutor":     func(e *Entrada) { e.Rodo.Condutores = nil },
		"CPF condutor":     func(e *Entrada) { e.Rodo.Condutores[0].CPF = "123" },
		"placa inválida":   func(e *Entrada) { e.Rodo.Tracao.Placa = "AB-12" },
		"lotação sem CEP":  func(e *Entrada) { e.ProdPred.CEPDescarrega = "" },
		"percurso = UFIni": func(e *Entrada) { e.UFPercurso = []string{"GO"} },
		"percurso repete":  func(e *Entrada) { e.UFFim = "SP"; e.UFPercurso = []string{"MG", "MG"} },
		"sem NF-e":         func(e *Entrada) { e.Descargas[0].NFes = nil },
		"chave curta":      func(e *Entrada) { e.Descargas[0].NFes = []string{"123"} },
		"NF-e repetida": func(e *Entrada) {
			k := chaveNFe("1")
			e.Descargas[0].NFes = []string{k, k}
		},
		"sem município carga": func(e *Entrada) { e.MunCarrega = nil },
		"UF inválida":         func(e *Entrada) { e.UFFim = "XX" },
		"cUnid":               func(e *Entrada) { e.CUnid = "03" },
		"tpAmb":               func(e *Entrada) { e.TpAmb = "3" },
		"CNPJ e CPF":          func(e *Entrada) { e.Emitente.CPF = "12345678909" },
		"RNTRC curto":         func(e *Entrada) { e.Rodo.RNTRC = "123" },
		"4 reboques": func(e *Entrada) {
			rb := Veiculo{Placa: "XYZ9876", CapKG: 1, TpCar: "00"}
			e.Rodo.Reboques = []Veiculo{rb, rb, rb, rb}
		},
		"tpEmit 9":                  func(e *Entrada) { e.TpEmit = "9" },
		"tpEmit 1 sem seguro":       func(e *Entrada) { e.TpEmit = "1" },
		"tpEmit 3 sem seguro":       func(e *Entrada) { e.TpEmit = "3" },
		"contingência sem QRSign":   func(e *Entrada) { e.TpEmis = "2" },
		"tpEmis 3":                  func(e *Entrada) { e.TpEmis = "3"; e.QRSign = "x" },
		"tpTransp sem proprietário": func(e *Entrada) { e.TpTransp = "1" },
		"tpTransp inválido":         func(e *Entrada) { e.TpTransp = "7" },
		"prop CPF com ETC": func(e *Entrada) {
			e.TpTransp = "1"
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Auto", TpProp: "1"}
		},
		"prop CNPJ com TAC": func(e *Entrada) {
			e.TpTransp = "2"
			e.Rodo.Tracao.Prop = &Proprietario{CNPJ: "11222333000181", RNTRC: "11112222", Nome: "Beta", TpProp: "2"}
		},
		"CT-e com tpEmit 2": func(e *Entrada) {
			e.Descargas[0].CTes = []string{chaveNFe("7")}
		},
		"sem prodPred com tpTransp": func(e *Entrada) {
			e.TpTransp = "2"
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Auto", TpProp: "1"}
			e.ProdPred = nil
		},
		"lotação com tpTransp sem infPag": func(e *Entrada) {
			e.TpTransp = "2"
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Auto", TpProp: "1"}
		},
		"CIOT exigido por data": func(e *Entrada) {
			e.TpTransp = "2"
			e.DhEmi = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
			e.Rodo.Tracao.Prop = &Proprietario{CPF: "12345678909", RNTRC: "11112222", Nome: "Auto", TpProp: "1"}
			e.Rodo.Pagamentos = []Pagamento{pagPIX()}
		},
		"infBanc ambíguo": func(e *Entrada) {
			p := pagPIX()
			p.CNPJIPEF = "11222333000181"
			e.Rodo.Pagamentos = []Pagamento{p}
		},
		"indPag inválido": func(e *Entrada) {
			p := pagPIX()
			p.IndPag = "5"
			e.Rodo.Pagamentos = []Pagamento{p}
		},
		"SegCodBarra curto": func(e *Entrada) {
			e.Descargas[0].Opc = map[string]DocOpc{e.Descargas[0].NFes[0]: {SegCodBarra: "123"}}
		},
		"texto fora do Latin-1": func(e *Entrada) { e.Emitente.Nome = "Empresa 日本" },
	}
	for nome, mut := range casos {
		t.Run(nome, func(t *testing.T) {
			e := entradaBase()
			mut(&e)
			if x, _, err := Build(e); err == nil {
				t.Fatalf("esperava erro, veio XML: %s", x)
			}
		})
	}
}

func pagPIX() Pagamento {
	return Pagamento{Nome: "Transportador", CPF: "12345678909", Comp: []CompPag{{Tipo: "01", Valor: 1000}},
		VContrato: 1000, IndPag: "0", PIX: "chave@pix.com"}
}

func TestBuild_ContingenciaQRSign(t *testing.T) {
	e := entradaBase()
	e.TpEmis, e.QRSign = "2", "QUJD+/=="
	x, ch, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if ch[34] != '2' || !strings.Contains(string(x), "&amp;sign=QUJD+/==</qrCodMDFe>") || !strings.Contains(string(x), "<tpEmis>2</tpEmis>") {
		t.Fatalf("contingência: %s", x)
	}
}

func TestBuild_TotalizadoresETpTransp(t *testing.T) {
	e := entradaBase()
	e.TpEmit = "1"
	e.Seguros = []Seguro{{RespSeg: "1", RespCNPJ: "11222333000181"}}
	e.Descargas = []Descarga{{CodigoMun: "5300108", Nome: "Brasília",
		CTes: []string{chaveNFe("1"), chaveNFe("2")}, NFes: []string{chaveNFe("3")}, MDFes: []string{chaveNFe("4")}}}
	x, _, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"<qCTe>2</qCTe><qNFe>1</qNFe><qMDFe>1</qMDFe>", "<tpEmit>1</tpEmit>"} {
		if !strings.Contains(string(x), w) {
			t.Errorf("faltou %s", w)
		}
	}
	if strings.Contains(string(x), "tpTransp") || strings.Contains(string(x), "infLotacao") {
		t.Error("tpTransp/infLotacao indevidos")
	}
}

func TestURLQRCode(t *testing.T) {
	if got := URLQRCode("123", "1"); got != "https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe=123&tpAmb=1" {
		t.Fatal(got)
	}
}

// dvMod11 é uma implementação independente do módulo 11 da chave.
func dvMod11(b string) string {
	soma, p := 0, 2
	for i := len(b) - 1; i >= 0; i-- {
		soma += int(b[i]-'0') * p
		if p++; p > 9 {
			p = 2
		}
	}
	if r := soma % 11; r >= 2 {
		return string(rune('0' + 11 - r))
	}
	return "0"
}

// dvRef: módulo 11 (pesos 2-9 da direita) só sobre dígitos, independente do código.
func dvRef(base43 string) string {
	soma := 0
	for i := len(base43) - 1; i >= 0; i-- {
		soma += int(base43[i]-'0') * (2 + (len(base43)-1-i)%8)
	}
	if r := soma % 11; r >= 2 {
		return string(rune('0' + 11 - r))
	}
	return "0"
}

func TestChave_AntesDoBuildContingencia(t *testing.T) {
	e := entradaBase()
	e.TpEmis, e.CMDF = "2", "12345678"
	if _, err := Chave(entradaBase()); err == nil {
		t.Error("Chave sem CMDF deveria falhar")
	}
	ch, err := Chave(e)
	if err != nil {
		t.Fatal(err)
	}
	e.QRSign = "SIGN" + ch[:6]
	x, chBuild, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if ch != chBuild || ch[35:43] != "12345678" || ch[43:] != dvRef(ch[:43]) {
		t.Fatalf("chave %s x %s", ch, chBuild)
	}
	if !strings.Contains(string(x), "chMDFe="+ch+"&amp;tpAmb=2&amp;sign=SIGN"+ch[:6]) || !strings.Contains(string(x), "<cMDF>12345678</cMDF>") {
		t.Errorf("QR/cMDF: %s", x)
	}
	e.CMDF = "1234"
	if _, err := Chave(e); err == nil {
		t.Error("CMDF curto deveria falhar")
	}
}
