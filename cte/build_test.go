package cte

import (
	"strings"
	"testing"
	"time"
)

// Cada linha quebra uma regra com uma entrada válida (entradaBase) e confere
// a mensagem. Os números de rejeição são os do pedido; os não confirmados no
// MOC estão marcados nos comentários do código.
func TestBuild_Regras(t *testing.T) {
	sub := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	casos := []struct {
		nome string
		mut  func(*Entrada)
		quer string
	}{
		{"vRec maior que vTPrest", func(e *Entrada) { e.VRec = 1000.01 }, "vRec"},
		{"soma de Comp diferente", func(e *Entrada) { e.Comp = []Comp{{"A", 400}, {"B", 500}} }, "soma dos Comp"},
		{"vTPrestLiq maior", func(e *Entrada) { e.VTPrestLiq = fp(1001) }, "vTPrestLiq"},
		{"sem tomador", func(e *Entrada) { e.Toma3 = "" }, "Toma3"},
		{"toma3 e toma4", func(e *Entrada) { e.Toma4 = &Terceiro{} }, "exatamente um"},
		{"toma3 sem participante", func(e *Entrada) { e.Toma3 = "2" }, "participante"},
		{"toma4 incompleto", func(e *Entrada) {
			e.Toma3, e.Toma4 = "", &Terceiro{CNPJ: "44555666000177", Nome: "X Y"}
		}, "dados completos"},
		{"indIEToma 1 sem IE", func(e *Entrada) { e.Rem.IE = "" }, "indIEToma=1"},
		{"indIEToma 2 com IE numérica", func(e *Entrada) { e.IndIEToma = "2" }, "indIEToma=2"},
		{"indIEToma 9 com IE", func(e *Entrada) { e.IndIEToma = "9" }, "indIEToma=9"},
		{"indIEToma inválido", func(e *Entrada) { e.IndIEToma = "3" }, "indIEToma"},
		{"CST 00 sem base", func(e *Entrada) { normal(e); e.ICMS.VBC = 0 }, "vICMS"},
		{"vICMS errado", func(e *Entrada) { normal(e); e.ICMS.VICMS = 121 }, "vICMS"},
		{"vICMS dentro da tolerância de 1 centavo", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "00", VBC: 333.33, PICMS: 12, VICMS: 40.01} }, ""},
		{"CST 20 sem pRedBC", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "20", VBC: 400, PICMS: 12, VICMS: 48} }, "pRedBC"},
		{"CST inexistente", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "10"} }, "sem grupo"},
		{"deson sem cBenef", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "40", VICMSDeson: 5} }, "cBenef"},
		{"cBenef curto", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "40", CBenef: "ABC"} }, "cBenef"},
		{"cBenef com espaço", func(e *Entrada) { normal(e); e.ICMS = ICMS{CST: "40", CBenef: "SEM CBENEF"} }, "cBenef"},
		{"ICMS60 vICMSSTRet errado", func(e *Entrada) {
			normal(e)
			e.ICMS = ICMS{CST: "60", VBCSTRet: 1000, PICMSSTRet: 12, VICMSSTRet: 100}
		}, "vICMSSTRet"},
		{"Simples sem ICMSSN", func(e *Entrada) { e.ICMS = ICMS{CST: "00", VBC: 1000, PICMS: 12, VICMS: 120} }, "deve usar ICMSSN"},
		{"MEI sem ICMSSN", func(e *Entrada) { e.Emitente.CRT = "4"; e.ICMS = ICMS{CST: "40"} }, "deve usar ICMSSN"},
		{"regime normal com ICMSSN", func(e *Entrada) { e.Emitente.CRT = "3"; e.IBSCBS = ibsPadrao() }, "ICMSSN só para CRT 1 ou 4"},
		{"CRT 3 homologação sem IBSCBS", func(e *Entrada) { normal(e); e.IBSCBS = nil }, "rej 310"},
		{"CRT 3 produção sem IBSCBS passa", func(e *Entrada) { normal(e); e.IBSCBS = nil; e.TpAmb = "1" }, ""},
		{"vIBSUF errado", func(e *Entrada) { normal(e); e.IBSCBS.G.IBSUF.V = 5; e.IBSCBS.G.VIBS = 5 }, "vIBSUF"},
		{"vIBS diferente da soma", func(e *Entrada) { normal(e); e.IBSCBS.G.VIBS = 3 }, "vIBS"},
		{"vCBS errado", func(e *Entrada) { normal(e); e.IBSCBS.G.CBS.V = 1 }, "vCBS"},
		{"gRed incompleto", func(e *Entrada) { normal(e); e.IBSCBS.G.CBS.PRedAliq = fp(10) }, "gRed"},
		{"CST IBSCBS com 2 dígitos", func(e *Entrada) { normal(e); e.IBSCBS.CST = "00" }, "CST (3 dígitos)"},
		{"FCP errado", func(e *Entrada) { e.ICMSUFFim = &ICMSUFFim{1000, 2, 18, 12, 99, 60, 0} }, "vFCPUFFim"},
		{"complemento sem chave", func(e *Entrada) { e.TpCTe = "1"; e.Carga, e.Rodo, e.Docs = nil, nil, Docs{} }, "InfCteComp"},
		{"complemento com carga", func(e *Entrada) { e.TpCTe = "1"; e.InfCteComp = []string{ch("6")} }, "não leva infCTeNorm"},
		{"substituição sem chave", func(e *Entrada) { e.TpCTe = "3" }, "InfCteSub.ChCte"},
		{"InfCteSub no CT-e normal", func(e *Entrada) { e.InfCteSub = &InfCteSub{ChCte: ch("7")} }, "só na substituição"},
		{"anulação extinta", func(e *Entrada) { e.TpCTe = "2" }, "anulação"},
		{"tpServ 1 sem docAnt", func(e *Entrada) { e.TpServ = "1" }, "docAnt"},
		{"tpServ 2 sem docAnt", func(e *Entrada) { e.TpServ = "2" }, "docAnt"},
		{"tpServ 3 sem docAnt", func(e *Entrada) { e.TpServ = "3" }, "docAnt"},
		{"tpServ 4 sem infServVinc", func(e *Entrada) { e.TpServ = "4" }, "infServVinc"},
		{"RNTRC inválido", func(e *Entrada) { e.Rodo.RNTRC = "123" }, "RNTRC"},
		{"sem rodo", func(e *Entrada) { e.Rodo = nil }, "Rodo"},
		{"chave NF-e curta", func(e *Entrada) { e.Docs.NFe[0].Chave = "123" }, "chave"},
		{"chave NF-e duplicada", func(e *Entrada) { e.Docs.NFe = append(e.Docs.NFe, e.Docs.NFe[0]) }, "repetida"},
		{"infDoc misturado", func(e *Entrada) { e.Docs.DCe = []string{ch("8")} }, "só um tipo"},
		{"unidade carga e transp juntas", func(e *Entrada) {
			e.Docs.NFe[0].UnidCarga = []UnidCarga{{Tp: "1", ID: "A1"}}
			e.Docs.NFe[0].UnidTransp = []UnidTransp{{Tp: "1", ID: "B1"}}
		}, "excludentes"},
		{"globalizado sem texto", func(e *Entrada) { e.IndGlobalizado = true }, "indGlobalizado"},
		{"contingência sem dhCont", func(e *Entrada) { e.TpEmis = "7" }, "dhCont"},
		{"dhCont na emissão normal", func(e *Entrada) { e.DhCont, e.XJust = &sub, "justificativa qualquer longa" }, "só existem"},
		{"offline sem sign", func(e *Entrada) { e.TpEmis, e.DhCont, e.XJust = "2", &sub, "justificativa qualquer longa" }, "QRSign"},
		{"tpEmis 5 (FS-DA extinto)", func(e *Entrada) { e.TpEmis = "5" }, "tpEmis"},
		{"CFOP inválido", func(e *Entrada) { e.CFOP = "6000" }, "CFOP"},
		{"cobrança com valor zero", func(e *Entrada) { e.Cobr = &Cobr{Dup: []Dup{{"1", "2026-11-01", fp(0)}}} }, "maior que zero"},
		{"autXML demais", func(e *Entrada) { e.AutXML = make([]string, 11) }, "autXML"},
		{"emitente sem documento", func(e *Entrada) { e.Emitente.CNPJ = "" }, "CNPJ ou CPF"},
		{"CRT inválido", func(e *Entrada) { e.Emitente.CRT = "5" }, "CRT"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := entradaBase()
			c.mut(&e)
			_, _, err := Build(e)
			switch {
			case c.quer == "" && err != nil:
				t.Fatalf("esperava sucesso: %v", err)
			case c.quer != "" && (err == nil || !strings.Contains(err.Error(), c.quer)):
				t.Fatalf("esperava erro com %q, veio %v", c.quer, err)
			}
		})
	}
}

func TestQRCode(t *testing.T) {
	c := ch("1")
	if got, want := URLQRCode(c, "2"), "https://dfe-portal.svrs.rs.gov.br/cte/qrCode?chCTe="+c+"&tpAmb=2"; got != want {
		t.Errorf("got %s", got)
	}
	if got := URLQRCodeContingencia(c, "1", "QUJD"); !strings.HasSuffix(got, "&tpAmb=1&sign=QUJD") {
		t.Errorf("got %s", got)
	}
	e := entradaBase()
	e.URLQRCode = "https://cte.fazenda.mg.gov.br/qr?"
	x, chave, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(x), "<qrCodCTe>https://cte.fazenda.mg.gov.br/qr?chCTe="+chave+"&amp;tpAmb=2</qrCodCTe>") {
		t.Errorf("QR da UF não usado: %s", x)
	}
}

func TestBuild_ChaveCPFeEstrutura(t *testing.T) {
	e := entradaBase()
	e.Emitente.CNPJ, e.Emitente.CPF = "", "12345678909"
	x, chave, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	if chave[6:20] != "00012345678909" || chave[20:22] != "57" {
		t.Errorf("chave %s", chave)
	}
	s := string(x)
	if !strings.HasPrefix(s, `<CTe xmlns="http://www.portalfiscal.inf.br/cte"><infCte versao="4.00"`) || strings.Contains(s, "<?xml") || strings.Contains(s, "<Signature") {
		t.Errorf("raiz inesperada: %.120s", s)
	}
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
	sub := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	e := entradaBase()
	e.TpEmis, e.DhCont, e.XJust, e.CCT = "2", &sub, "SEFAZ de origem fora do ar por mais de uma hora", "12345678"
	if _, err := Chave(entradaBase()); err == nil {
		t.Error("Chave sem CCT deveria falhar")
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
	if !strings.Contains(string(x), "chCTe="+ch+"&amp;tpAmb=2&amp;sign=SIGN"+ch[:6]) || !strings.Contains(string(x), "<cCT>12345678</cCT>") {
		t.Errorf("QR/cCT: %s", x)
	}
	e.CCT = "1234"
	if _, err := Chave(e); err == nil {
		t.Error("CCT curto deveria falhar")
	}
}
