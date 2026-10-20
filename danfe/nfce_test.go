package danfe_test

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/danfe"
)

// entradaNFCeCupom: NFC-e de balcão com 2 itens, desconto, 2 pagamentos e troco.
func entradaNFCeCupom() builder.EntradaNFe {
	return builder.EntradaNFe{
		Serie: "1", NNF: "12345",
		DhEmi: time.Date(2026, 10, 20, 14, 35, 12, 0, time.FixedZone("BRT", -3*3600)),
		NatOp: "VENDA A CONSUMIDOR", TpAmb: "1", FinNFe: "1", IndFinal: "1", IndPres: "1", Mod: "65",
		QRVersao: "3", UrlConsultaNFCe: "https://nfe.sefaz.go.gov.br/nfeweb/sites/nfce/danfeNFCe",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "MERCADINHO BOA VISTA LTDA", IE: "104567890", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Palmeiras", Numero: "150", Complemento: "Loja 2", Bairro: "Setor Central",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO", CEP: "74000000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Dest: builder.EntradaDest{CPF: "12345678909", Nome: "MARIA DA SILVA"},
		Itens: []builder.EntradaItem{
			{CProd: "7891", CEAN: "SEM GTIN", Nome: "LEITE INTEGRAL UHT", NCM: "04012010", CFOP: "5102", Unidade: "UN", Quantidade: 6, VUnitario: 5.49, ICMS: builder.EntradaICMS{CSOSN: "102"}},
			{CProd: "A042", CEAN: "SEM GTIN", Nome: "QUEIJO MUSSARELA", NCM: "04061010", CFOP: "5102", Unidade: "KG", Quantidade: 0.482, VUnitario: 42.90, VDesconto: 3.00, ICMS: builder.EntradaICMS{CSOSN: "102"}},
		},
		Frete:     builder.EntradaFrete{Modalidade: "9"},
		Pagamento: []builder.EntradaPagamento{{Forma: "01", Valor: 50.00}, {Forma: "17", Valor: 5.00}},
		InfCpl:    "Obrigado pela preferencia",
	}
}

// xmlNFCeProc gera o XML assinado e o embrulha num nfeProc com protocolo.
func xmlNFCeProc(t *testing.T, e builder.EntradaNFe) (xml string, chave string) {
	t.Helper()
	assinado := string(assinarEntrada(t, e))
	// a chave sai do próprio XML assinado (o cNF é sorteado a cada Build)
	m := regexp.MustCompile(`Id="NFe(\d{44})"`).FindStringSubmatch(assinado)
	if m == nil {
		t.Fatal("chave não encontrada no XML assinado")
	}
	ch := m[1]
	assinado = strings.Replace(assinado, `<?xml version="1.0" encoding="UTF-8"?>`, "", 1)
	if e.TpEmis == "9" {
		return assinado, ch
	}
	prot := `<protNFe versao="4.00"><infProt><tpAmb>` + e.TpAmb + `</tpAmb><chNFe>` + ch +
		`</chNFe><dhRecbto>2026-10-20T14:35:20-03:00</dhRecbto><nProt>152260000123456</nProt><cStat>100</cStat></infProt></protNFe>`
	return `<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">` + assinado + prot + `</nfeProc>`, ch
}

// cp1252 converte o texto esperado para os bytes que o fpdf grava no PDF.
func cp1252(s string) []byte {
	s = strings.ReplaceAll(s, "–", "\x96")
	var out []byte
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

// TestGerarDANFENFCe_ExigenciasDoManual confere, divisão por divisão, o texto
// que o Manual DANFE NFC-e v6.0 manda imprimir (e o que não pode aparecer).
func TestGerarDANFENFCe_ExigenciasDoManual(t *testing.T) {
	tests := []struct {
		nome      string
		ajusta    func(*builder.EntradaNFe)
		opcoes    danfe.OpcoesNFCe
		contem    []string // texto fixo / dado esperado
		naoContem []string
		vezes     map[string]int
	}{
		{
			nome: "online com CPF, desconto, 2 pagamentos e troco",
			contem: []string{
				"Documento Auxiliar da Nota Fiscal de Consumidor", "Eletrônica", // I: título (2 linhas)
				"CNPJ: 11.222.333/0001-81",                                    // I: máscara
				"MERCADINHO BOA VISTA LTDA",                                   // I: razão social
				"Rua das Palmeiras, 150, Loja 2, Setor Central, Goiania - GO", // I: endereço sem país
				"7891  LEITE INTEGRAL UHT",                                    // II: código + descrição
				"6 UN", "5,49", "32,94", "0,482 KG", "42,90", "20,68",         // II: qtde, un, unit, total
				"Qtde. total de itens", "Valor total R$", "Descontos R$", "Valor a Pagar R$", // III
				"FORMA DE PAGAMENTO", "Dinheiro", "Pix", "Troco R$", // III
				"Consulte pela Chave de Acesso em",                                // IV
				"CONSUMIDOR CPF: 123.456.789-09",                                  // VI
				"NFC-e nº 12345  Série 1  20/10/2026 14:35:12",                    // VII
				"Protocolo de autoriza", "152260000123456", "20/10/2026 14:35:20", // VII
				"Obrigado pela preferencia", // IX
			},
			naoContem: []string{"EMITIDA EM CONTING", "HOMOLOGA", "NÃO IDENTIFICADO", "Via do Estabelecimento"},
		},
		{
			nome: "sem consumidor, sem desconto e sem troco",
			ajusta: func(e *builder.EntradaNFe) {
				e.Dest = builder.EntradaDest{}
				e.Itens[1].VDesconto = 0
				e.Pagamento = e.Pagamento[:1]
				e.Pagamento[0].Valor = 300.00
			},
			contem: []string{"CONSUMIDOR NÃO IDENTIFICADO", "Valor total R$", "Dinheiro", "Troco R$"},
			// sem acréscimo/desconto não imprime "Valor a Pagar" (3.1.3); troco existe porque pagou 300.
			naoContem: []string{"Valor a Pagar", "Descontos R$", "CONSUMIDOR CPF"},
		},
		{
			nome:   "pagamento exato: sem linha de troco",
			ajusta: func(e *builder.EntradaNFe) { e.Pagamento = []builder.EntradaPagamento{{Forma: "01", Valor: 50.62}} },
			contem: []string{"Dinheiro"}, naoContem: []string{"Troco R$"},
		},
		{
			nome:      "homologação imprime o texto fixo",
			ajusta:    func(e *builder.EntradaNFe) { e.TpAmb = "2" },
			contem:    []string{"EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO", "SEM VALOR FISCAL"},
			naoContem: []string{"EMITIDA EM CONTING"},
		},
		{
			nome: "contingência: texto nos 2 lugares, sem protocolo, via consumidor",
			ajusta: func(e *builder.EntradaNFe) {
				e.TpEmis, e.DhCont, e.XJust = "9", "2026-10-20T14:30:00-03:00", "queda de internet na loja"
				e.CSC, e.CSCId, e.QRVersao = "SEU-CSC-123456", "1", "2"
			},
			contem:    []string{"Pendente de autoriza", "Via consumidor"},
			naoContem: []string{"Protocolo de autoriza", "Via do Estabelecimento"},
			vezes:     map[string]int{"EMITIDA EM CONTING": 2, "Pendente de autoriza": 2},
		},
		{
			nome: "contingência 2ª via do estabelecimento",
			ajusta: func(e *builder.EntradaNFe) {
				e.TpEmis, e.DhCont, e.XJust = "9", "2026-10-20T14:30:00-03:00", "queda de internet na loja"
				e.CSC, e.CSCId, e.QRVersao = "SEU-CSC-123456", "1", "2"
			},
			opcoes:    danfe.OpcoesNFCe{ViaEstabelecimento: true},
			contem:    []string{"Via do Estabelecimento"},
			naoContem: []string{"Via consumidor"},
		},
		{
			nome:      "2ª via fora de contingência é ignorada",
			opcoes:    danfe.OpcoesNFCe{ViaEstabelecimento: true},
			naoContem: []string{"Via do Estabelecimento"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			e := entradaNFCeCupom()
			if tc.ajusta != nil {
				tc.ajusta(&e)
			}
			xml, chave := xmlNFCeProc(t, e)
			pdf, err := danfe.GerarDANFENFCeComOpcoes([]byte(xml), tc.opcoes)
			if err != nil {
				t.Fatalf("GerarDANFENFCe: %v", err)
			}
			conteudo := decodificarStreamsPDF(t, pdf)
			for _, s := range tc.contem {
				if !bytes.Contains(conteudo, cp1252(s)) {
					t.Errorf("PDF sem %q", s)
				}
			}
			for _, s := range tc.naoContem {
				if bytes.Contains(conteudo, cp1252(s)) {
					t.Errorf("PDF não devia conter %q", s)
				}
			}
			for s, n := range tc.vezes {
				if got := bytes.Count(conteudo, cp1252(s)); got != n {
					t.Errorf("%q aparece %d vez(es), esperava %d", s, got, n)
				}
			}
			// IV: chave em 11 blocos de 4 dígitos separados por espaço
			blocos := make([]string, 0, 11)
			for i := 0; i < 44; i += 4 {
				blocos = append(blocos, chave[i:i+4])
			}
			if !bytes.Contains(conteudo, []byte(strings.Join(blocos, " "))) {
				t.Errorf("chave não impressa em 11 blocos de 4: %s", strings.Join(blocos, " "))
			}
		})
	}
}

func TestGerarDANFENFCe_EmitenteCPFeConsumidorEstrangeiro(t *testing.T) {
	xml, _ := xmlNFCeProc(t, entradaNFCeCupom())
	// emitente pessoa física (produtor rural, manual 3.1.1: C02a) e consumidor estrangeiro (E03a)
	xml = strings.Replace(xml, "<emit><CNPJ>11222333000181</CNPJ>", "<emit><CPF>98765432100</CPF>", 1)
	xml = strings.Replace(xml, "<CPF>12345678909</CPF>", "<idEstrangeiro>AB1234567</idEstrangeiro>", 1)
	pdf, err := danfe.GerarDANFENFCe([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	c := decodificarStreamsPDF(t, pdf)
	for _, s := range []string{"CPF: 987.654.321-00", "CONSUMIDOR Id. Estrangeiro: AB1234567"} {
		if !bytes.Contains(c, cp1252(s)) {
			t.Errorf("PDF sem %q", s)
		}
	}
	if bytes.Contains(c, cp1252("CNPJ: ")) {
		t.Error("emitente CPF não devia imprimir rótulo CNPJ")
	}
}

func TestParseNFeXML_CamposDoCupom(t *testing.T) {
	xml, chave := xmlNFCeProc(t, entradaNFCeCupom())
	d, err := danfe.ParseNFeXML([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if d.ChaveAcesso != chave || d.DhEmiCompleta != "20/10/2026 14:35:12" || d.DhRecbtoCompleta != "20/10/2026 14:35:20" {
		t.Errorf("chave/datas: %s %q %q", d.ChaveAcesso, d.DhEmiCompleta, d.DhRecbtoCompleta)
	}
	if d.VTroco != 4.38 || d.DestCPF != "123.456.789-09" {
		t.Errorf("vTroco=%v destCPF=%q", d.VTroco, d.DestCPF)
	}
	if !strings.HasSuffix(d.QrCode, chave+"|3|1") {
		t.Errorf("QR 3.00 online do XML = %s", d.QrCode)
	}
}

// O cupom sai com a altura do conteúdo, não numa folha fixa de 500 mm.
func TestGerarDANFENFCe_AlturaDoConteudo(t *testing.T) {
	xml, _ := xmlNFCeProc(t, entradaNFCeCupom())
	pdf, err := danfe.GerarDANFENFCe([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`/MediaBox \[0(?:\.00)? 0(?:\.00)? ([0-9.]+) ([0-9.]+)\]`).FindSubmatch(pdf)
	if m == nil {
		t.Fatal("MediaBox não encontrado")
	}
	larg, _ := strconv.ParseFloat(string(m[1]), 64)
	alt, _ := strconv.ParseFloat(string(m[2]), 64)
	mm := func(pt float64) float64 { return pt / 72 * 25.4 }
	if l := mm(larg); l < 79.9 || l > 80.1 {
		t.Errorf("largura = %.1f mm, esperava 80", l)
	}
	if a := mm(alt); a < 120 || a > 300 {
		t.Errorf("altura = %.1f mm, fora do esperado para 2 itens", a)
	}
}
