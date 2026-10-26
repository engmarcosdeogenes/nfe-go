package cte

import (
	"strings"
	"testing"
	"time"
)

// Regras de BuildOS: cada caso deve ser recusado, com a mensagem esperada.
// Fontes: enums e documentação do cteTiposBasico_v4.00.xsd (TCTeOS) e do
// cteModalRodoviarioOS_v4.00.xsd; números de rejeição do MOC: não confirmados,
// exceto a 310 (IBS/CBS CRT 3 em homologação, NT 2026.002).
func TestBuildOS_Rejeicoes(t *testing.T) {
	viagem := time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)
	casos := []struct {
		nome string
		mut  func(*EntradaOS)
		erro string
	}{
		{"tpServ do CT-e 57", func(e *EntradaOS) { e.TpServ = "0" }, "tpServ"},
		{"tpServ 5", func(e *EntradaOS) { e.TpServ = "5" }, "tpServ"},
		{"tpCTe 2 extinto", func(e *EntradaOS) { e.TpCTe = "2" }, "tpCTe"},
		{"tpEmis 3 não existe no OS", func(e *EntradaOS) { e.TpEmis = "3" }, "tpEmis"},
		{"tpEmis 7 sem justificativa", func(e *EntradaOS) { e.TpEmis = "7" }, "dhCont"},
		{"modal aéreo ainda não", func(e *EntradaOS) { e.Modal = "02" }, "não implementado"},
		{"infTribFed negativo", func(e *EntradaOS) { e.TribFed = &TribFed{VPIS: fp(-1)} }, "vPIS"},
		{"modal 05 não existe no OS", func(e *EntradaOS) { e.Modal = "05" }, "inválido"},
		{"CFOP de entrada", func(e *EntradaOS) { e.CFOP = "1353" }, "CFOP"},
		{"emitente CPF", func(e *EntradaOS) { e.Emitente.CNPJ, e.Emitente.CPF = "", "12345678909" }, "só CNPJ"},
		{"emitente sem IE", func(e *EntradaOS) { e.Emitente.IE = "" }, "IE"},
		{"passageiros sem tomador", func(e *EntradaOS) { e.Toma = nil }, "toma"},
		{"valores sem tomador", func(e *EntradaOS) { valoresOS(e); e.Toma = nil }, "toma"},
		{"tomador incompleto", func(e *EntradaOS) { e.Toma.End.Bairro = "" }, "toma exige"},
		{"indIEToma 1 sem IE", func(e *EntradaOS) { e.Toma.IE = "" }, "indIEToma=1"},
		{"indIEToma 9 com IE", func(e *EntradaOS) { e.IndIEToma = "9" }, "indIEToma=9"},
		{"início sem fim", func(e *EntradaOS) { e.Fim = nil }, "juntos"},
		{"passageiros sem rodoOS", func(e *EntradaOS) { e.RodoOS = nil }, "infModal"},
		{"bagagem sem rodoOS", func(e *EntradaOS) { e.TpServ, e.RodoOS = "8", nil }, "infModal"},
		{"TAF e registro estadual juntos", func(e *EntradaOS) { e.RodoOS.NroRegEstadual = strings.Repeat("1", 25) }, "exatamente um"},
		{"nem TAF nem registro", func(e *EntradaOS) { e.RodoOS.TAF = "" }, "exatamente um"},
		{"TAF com 11 dígitos", func(e *EntradaOS) { e.RodoOS.TAF = "12345678901" }, "12 dígitos"},
		{"registro estadual curto", func(e *EntradaOS) { e.RodoOS.TAF, e.RodoOS.NroRegEstadual = "", "123" }, "25 dígitos"},
		{"placa inválida", func(e *EntradaOS) { e.RodoOS.Veic.Placa = "12" }, "placa"},
		{"fretamento eventual sem dhViagem", func(e *EntradaOS) { e.RodoOS.Fretamento.DhViagem = nil }, "dhViagem"},
		{"fretamento contínuo com dhViagem", func(e *EntradaOS) { e.RodoOS.Fretamento = &Fretamento{TpFretamento: "2", DhViagem: &viagem} }, "dhViagem"},
		{"tpFretamento 3", func(e *EntradaOS) { e.RodoOS.Fretamento.TpFretamento = "3" }, "tpFretamento"},
		{"fretamento em valores", func(e *EntradaOS) {
			valoresOS(e)
			e.RodoOS = &RodoviarioOS{TAF: "123456789012", Fretamento: &Fretamento{TpFretamento: "2"}}
		}, "só no transporte de pessoas"},
		{"proprietário sem tpProp", func(e *EntradaOS) {
			e.RodoOS.Veic.Prop = &PropVeicOS{CPF: "12345678909", TAF: "123456789012", Nome: "Fulano"}
		}, "tpProp"},
		{"refCTeCanc fora de valores", func(e *EntradaOS) { e.RefCTeCanc = ch("8") }, "refCTeCanc"},
		{"infServico ausente", func(e *EntradaOS) { e.Servico = nil }, "infServico"},
		{"xDescServ longo", func(e *EntradaOS) { e.Servico.XDescServ = strings.Repeat("a", 31) }, "xDescServ"},
		{"infDocRef com chBPe e nDoc", func(e *EntradaOS) { e.DocRef = []DocRefOS{{ChBPe: ch("6"), NDoc: "1"}} }, "chBPe ou nDoc"},
		{"seguro com responsável 1", func(e *EntradaOS) { e.Seg = []SegOS{{RespSeg: "1"}} }, "respSeg"},
		{"GTV-e sem componente", func(e *EntradaOS) { valoresOS(e); e.GTVe = []InfGTVe{{Chave: ch("6")}} }, "Comp"},
		{"substituição sem chave", func(e *EntradaOS) { e.TpCTe = "3" }, "InfCteSub"},
		{"InfCteSub sem substituição", func(e *EntradaOS) { e.InfCteSub = ch("7") }, "InfCteSub"},
		{"complemento sem chaves", func(e *EntradaOS) { e.TpCTe, e.Servico, e.RodoOS = "1", nil, nil }, "InfCteComp"},
		{"complemento com serviço", func(e *EntradaOS) { e.TpCTe, e.InfCteComp = "1", []string{ch("6")} }, "não leva infCTeNorm"},
		{"compl com fluxo (não existe no OS)", func(e *EntradaOS) { e.Compl = &Compl{XOrig: "Goiania"} }, "compl do CT-e OS"},
		{"ICMS60 não existe no OS", func(e *EntradaOS) {
			normalOS(e)
			e.ICMS = ICMS{CST: "60", VBCSTRet: 100, PICMSSTRet: 12, VICMSSTRet: 12}
		}, "CST 60"},
		{"Simples com ICMS00", func(e *EntradaOS) { e.ICMS = ICMS{CST: "00", VBC: 100, PICMS: 12, VICMS: 12} }, "ICMSSN"},
		{"CRT 3 homologação sem IBSCBS (rej 310)", func(e *EntradaOS) {
			normalOS(e)
			e.IBSCBS = nil
		}, "rej 310"},
		{"vRec maior que vTPrest", func(e *EntradaOS) { e.VRec = 3000 }, "vRec"},
		{"percurso com mais de 25 UF", func(e *EntradaOS) { e.Percurso = make([]string, 26) }, "infPercurso"},
		{"contingência 2 sem QRSign", func(e *EntradaOS) {
			j := time.Now()
			e.TpEmis, e.DhCont, e.XJust = "2", &j, "Emissao offline por falha de conexao"
		}, "QRSign"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := entradaOSBase()
			c.mut(&e)
			_, _, err := BuildOS(e)
			if err == nil {
				t.Fatal("deveria recusar")
			}
			if !strings.Contains(err.Error(), c.erro) {
				t.Errorf("erro %q não menciona %q", err, c.erro)
			}
		})
	}
}

func TestBuildOS_ChaveEModelo(t *testing.T) {
	e := entradaOSBase()
	e.CCT = "12345678"
	c1, err := ChaveOS(e)
	if err != nil {
		t.Fatal(err)
	}
	x, c2, err := BuildOS(e)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 || c2[20:22] != "67" || c2[6:20] != "11222333000181" || c2[2:4] != "26" {
		t.Errorf("chave %s / %s", c1, c2)
	}
	s := string(x)
	for _, w := range []string{"<mod>67</mod>", "<tpServ>6</tpServ>", "<qrCodCTe>", "chCTe=" + c2, "<dhViagem>2026-11-05T08:00:00-03:00</dhViagem>", "<TAF>123456789012</TAF>"} {
		if !strings.Contains(s, w) {
			t.Errorf("XML sem %s", w)
		}
	}
	e.CCT = ""
	if _, err := ChaveOS(e); err == nil {
		t.Error("ChaveOS sem CCT deveria falhar")
	}
}
