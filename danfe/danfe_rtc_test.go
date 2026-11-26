package danfe_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/danfe"
)

// Testes do DANFE da NT 2026.010 (IBS/CBS/IS no PDF). As notas com IBS/CBS
// saem do builder de verdade; IS e gMono o builder ainda não monta, então
// esses grupos são enxertados no XML com os nomes de tag da NT 2025.002-RTC.

// entradaRTC monta uma nota CRT=3 com n itens; com ibscbs, todo item leva o
// grupo IBSCBS e os ímpares levam gRed de 60%.
func entradaRTC(n int, ibscbs bool) builder.EntradaNFe {
	e := builder.EntradaNFe{
		Serie: "1", NNF: "77",
		DhEmi:    time.Date(2026, 12, 1, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA DE MERCADORIA",
		TpAmb:    "2",
		FinNFe:   "1",
		IndFinal: "0",
		IndPres:  "1",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "METALURGICA TESTE LTDA", Fantasia: "METALTESTE",
			IE: "123456789", CRT: "3",
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Chapas", Numero: "100", Bairro: "Industrial",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil", Fone: "6299999999",
			},
		},
		Dest: builder.EntradaDest{
			CNPJ: "99888777000155", Nome: "CLIENTE INDUSTRIA SA", IndIEDest: "1", IE: "987654321",
			End: builder.EntradaEndereco{
				Logradouro: "Av. do Aco", Numero: "500", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74100000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Frete: builder.EntradaFrete{Modalidade: "9"},
	}
	for i := range n {
		item := builder.EntradaItem{
			CProd: fmt.Sprintf("ITEM-%03d", i), CEAN: "SEM GTIN",
			Nome: fmt.Sprintf("PERFIL ESTRUTURAL DE ACO GALVANIZADO LOTE %03d", i),
			NCM:  "72162100", CFOP: "5102", Unidade: "UN",
			Quantidade: 10, VUnitario: 100.00,
			ICMS: builder.EntradaICMS{CST: "00", Aliq: 12.0},
		}
		if ibscbs {
			item.IBSCBS = &builder.EntradaIBSCBS{
				CST: "000", ClassTrib: "000001",
				AliqIBSUF: 0.1, AliqIBSMun: 0.05, AliqCBS: 0.9,
			}
			if i%2 == 1 {
				item.IBSCBS.CST, item.IBSCBS.ClassTrib = "200", "200009"
				item.IBSCBS.PercentualReducao = 60
			}
		}
		e.Itens = append(e.Itens, item)
	}
	return e
}

// comISeMono enxerta no XML o grupo IS do primeiro item, o ISTot e o gMono
// do IBSCBSTot. Ordem do schema: IS antes de IBSCBS no imposto; ISTot antes
// de IBSCBSTot no total; gMono depois de gCBS.
func comISeMono(t *testing.T, xml []byte) []byte {
	t.Helper()
	s := string(xml)
	troca := func(antigo, novo string) {
		if !strings.Contains(s, antigo) {
			t.Fatalf("XML de teste sem %q", antigo)
		}
		s = strings.Replace(s, antigo, novo, 1)
	}
	troca("<IBSCBS>", "<IS><CSTIS>000</CSTIS><cClassTribIS>000001</cClassTribIS><vBCIS>1000.00</vBCIS><pIS>2.0000</pIS><vIS>20.00</vIS></IS><IBSCBS>")
	troca("<IBSCBSTot>", "<ISTot><vIS>20.00</vIS></ISTot><IBSCBSTot>")
	troca("</gCBS></IBSCBSTot>", "</gCBS><gMono><vIBSMono>3.00</vIBSMono><vCBSMono>4.00</vCBSMono><vIBSMonoReten>1.50</vIBSMonoReten><vCBSMonoReten>2.50</vCBSMonoReten><vIBSMonoRet>0.00</vIBSMonoRet><vCBSMonoRet>0.00</vCBSMonoRet></gMono></IBSCBSTot>")
	return []byte(s)
}

func TestParseNFeXML_IBSCBS(t *testing.T) {
	d, err := danfe.ParseNFeXML(assinarEntrada(t, entradaRTC(2, true)))
	if err != nil {
		t.Fatal(err)
	}
	if d.EmitCRT != "3" {
		t.Errorf("CRT = %q", d.EmitCRT)
	}
	// vBC = 1000 - ICMS 120 - PIS 6,50 - COFINS 30 = 843,50 (LC 214/2025 art. 12 §2º V); CBS 7,59 + 3,04.
	if d.VCBSTot != "10.63" || d.VIBSUFTot == "" || d.VIBSMunTot == "" {
		t.Errorf("totais IBS/CBS: CBS=%q UF=%q Mun=%q", d.VCBSTot, d.VIBSUFTot, d.VIBSMunTot)
	}
	if d.VISTot != "" || d.TemMono {
		t.Errorf("nota sem IS/gMono não pode ter VISTot=%q TemMono=%v", d.VISTot, d.TemMono)
	}

	// Item 1: sem gRed, alíquota vigente (pIBSUF/pIBSMun/pCBS).
	it := d.Itens[0]
	if !it.TemIBSCBS || it.ClassTrib != "000001" || it.VBCIBSCBS != 843.5 {
		t.Errorf("item 1: %+v", it)
	}
	if it.AliqIBSUF != 0.1 || it.AliqIBSMun != 0.05 || it.AliqCBS != 0.9 || it.VCBS != 7.59 {
		t.Errorf("item 1 alíquotas: UF=%v Mun=%v CBS=%v vCBS=%v", it.AliqIBSUF, it.AliqIBSMun, it.AliqCBS, it.VCBS)
	}

	// Item 2: gRed de 60% — NT 2026.010 §4.3 manda a alíquota efetiva.
	it = d.Itens[1]
	if it.ClassTrib != "200009" || it.AliqCBS != 0.36 || it.AliqIBSUF != 0.04 || it.AliqIBSMun != 0.02 {
		t.Errorf("item 2 (gRed): cClassTrib=%q UF=%v Mun=%v CBS=%v, esperava 200009/0.04/0.02/0.36",
			it.ClassTrib, it.AliqIBSUF, it.AliqIBSMun, it.AliqCBS)
	}
}

func TestParseNFeXML_SemIBSCBS(t *testing.T) {
	d, err := danfe.ParseNFeXML(assinarEntrada(t, entradaRTC(1, false)))
	if err != nil {
		t.Fatal(err)
	}
	if d.VCBSTot != "" || d.VIBSUFTot != "" || d.VIBSMunTot != "" || d.VISTot != "" {
		t.Errorf("nota sem IBSCBS com total preenchido: %q %q %q %q", d.VCBSTot, d.VIBSUFTot, d.VIBSMunTot, d.VISTot)
	}
	if d.Itens[0].TemIBSCBS || d.Itens[0].TemIS {
		t.Error("item sem grupos marcado com IBSCBS/IS")
	}
}

func TestParseNFeXML_ISeMono(t *testing.T) {
	d, err := danfe.ParseNFeXML(comISeMono(t, assinarEntrada(t, entradaRTC(1, true))))
	if err != nil {
		t.Fatal(err)
	}
	it := d.Itens[0]
	if !it.TemIS || it.VBCIS != 1000 || it.AliqIS != 2 || it.VIS != 20 {
		t.Errorf("IS do item: %+v", it)
	}
	if d.VISTot != "20.00" || !d.TemMono || d.VIBSMonoTot != "3.00" || d.VCBSMonoRetenTot != "2.50" {
		t.Errorf("totais IS/mono: vIS=%q mono=%v vIBSMono=%q vCBSMonoReten=%q", d.VISTot, d.TemMono, d.VIBSMonoTot, d.VCBSMonoRetenTot)
	}
}

var (
	brt = time.FixedZone("BRT", -3*3600)
	// Última e primeira dhEmi de cada lado do corte da NT 2026.010.
	antesRTC  = time.Date(2026, 11, 30, 23, 59, 59, 0, brt)
	depoisRTC = time.Date(2026, 12, 1, 0, 0, 0, 0, brt)
)

func emitidaEm(e builder.EntradaNFe, t time.Time) builder.EntradaNFe {
	e.DhEmi = t
	return e
}

// TestGerarDANFE_RTC confere no conteúdo do PDF o que a NT 2026.010 manda
// desenhar, e o que ela proíbe inventar (§4.4). O leiaute sai da dhEmi.
func TestGerarDANFE_RTC(t *testing.T) {
	// Rótulos que só existem num dos dois leiautes.
	antigo := []string{"CÁLCULO DO IMPOSTO", "B.CÁLC.ICMS"}
	novo := []string{"TOTAL DOS PRODUTOS E TOTAL DA NOTA", "TOTAL DO ICMS / IPI", "TOTAL DO IBS / CBS / IS",
		"BASES DE CÁLCULO", "VALOR DOS TRIBUTOS", "CÓDIGO DO REGIME TRIBUTÁRIO"}
	casos := []struct {
		nome        string
		xml         []byte
		tem, naoTem []string
	}{
		{
			// Reimpressão de nota de novembro, mesmo com IBS/CBS no XML.
			nome:   "antes-com-ibscbs",
			xml:    assinarEntrada(t, emitidaEm(entradaRTC(2, true), antesRTC)),
			tem:    antigo,
			naoTem: append(novo, "cClassTrib", "ITEM-001 - "),
		},
		{
			// CRT 1, sem grupo IBSCBS: o quadro do §4.1 sai, com os campos
			// em branco (§4.4); nada de cClassTrib nem linha de monofásico.
			nome:   "depois-simples-sem-ibscbs",
			xml:    assinarEntrada(t, emitidaEm(entradaDANFETeste(), depoisRTC)),
			tem:    append(novo, "1 - SIMPLES NACIONAL", "PERF-002 - PERFIL ESTRUTURAL L 2X2", "CST 0400", "IBS / CBS ", "IBS UF  / IPI "),
			naoTem: append(antigo, "cClassTrib", "MONOF", "VALOR DO FCP"),
		},
		{
			nome: "depois-com-ibscbs",
			xml:  assinarEntrada(t, emitidaEm(entradaRTC(1, true), depoisRTC)),
			tem: append(novo, "3 - REGIME NORMAL", "cClassTrib 000001", "IBS / CBS 843,50",
				"ICMS 12,00% / CBS 0,90%", "IBS UF 0,10% / IPI ", "IBS MUN 0,05% / IS ",
				"ICMS 120,00 / CBS 7,59", "(7,59)"),
			naoTem: append(antigo, "MONOF"),
		},
		{
			// Item 2 com gRed de 60%: §4.3 manda a alíquota efetiva.
			nome: "depois-gred",
			xml:  assinarEntrada(t, emitidaEm(entradaRTC(2, true), depoisRTC)),
			tem: []string{"ITEM-001 - PERFIL ESTRUTURAL", "cClassTrib 200009", "ICMS 12,00% / CBS 0,36%", "IBS UF 0,04% / IPI ",
				"IBS MUN 0,02% / IS ", "CBS 3,04", "(10,63)"},
		},
		{
			nome: "depois-is-mono",
			xml:  comISeMono(t, assinarEntrada(t, emitidaEm(entradaRTC(1, true), depoisRTC))),
			tem: []string{"IS 1.000,00", "IBS MUN 0,05% / IS 2,00%", "/ IS 20,00", "(20,00)",
				"VALOR DO IBS MONOF", "(3,00)", "(2,50)"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pdf, err := danfe.Gerar(c.xml, false)
			if err != nil {
				t.Fatal(err)
			}
			conteudo := decodificarStreamsPDF(t, pdf)
			for _, s := range c.tem {
				if !bytes.Contains(conteudo, pdfTexto(s)) {
					t.Errorf("PDF sem %q", s)
				}
			}
			for _, s := range c.naoTem {
				if bytes.Contains(conteudo, pdfTexto(s)) {
					t.Errorf("PDF não deveria ter %q", s)
				}
			}
			salvarProva(t, "danfe-rtc-"+c.nome+".pdf", pdf)
		})
	}
}

// pdfTexto passa o texto para cp1252, que é como o fpdf grava as fontes core.
func pdfTexto(s string) []byte {
	var b []byte
	for _, r := range s {
		switch r {
		case 'Á':
			b = append(b, 0xC1)
		case 'Ó':
			b = append(b, 0xD3)
		case 'Í':
			b = append(b, 0xCD)
		default:
			b = append(b, string(r)...)
		}
	}
	return b
}

// TestGerarDANFE_RTC_QuebraPagina: com 80 itens o leiaute novo quebra
// página sem perder item, e toda folha repete o cabeçalho com o CRT (§3.5
// do Anexo II, que a NT não muda).
func TestGerarDANFE_RTC_QuebraPagina(t *testing.T) {
	const n = 80
	pdf, err := danfe.Gerar(assinarEntrada(t, emitidaEm(entradaRTC(n, true), depoisRTC)), false)
	if err != nil {
		t.Fatal(err)
	}
	paginas := contarPaginas(pdf)
	if paginas < 2 {
		t.Fatalf("80 itens em %d folha", paginas)
	}
	conteudo := decodificarStreamsPDF(t, pdf)
	for _, c := range []struct {
		s    string
		quer int
	}{
		{"NCM 72162100", n},
		{"cClassTrib 000001", n / 2},
		{"cClassTrib 200009", n / 2},
		{"REGIME NORMAL", paginas},
		{"CHAVE DE ACESSO", paginas},
		{"VALOR DOS TRIBUTOS", paginas},
	} {
		if got := bytes.Count(conteudo, []byte(c.s)); got != c.quer {
			t.Errorf("%q aparece %d vezes, esperado %d (%d folhas)", c.s, got, c.quer, paginas)
		}
	}
	t.Logf("80 itens: %d folhas", paginas)
	salvarProva(t, "danfe-rtc-80-itens.pdf", pdf)

	antes, err := danfe.Gerar(assinarEntrada(t, emitidaEm(entradaRTC(n, true), antesRTC)), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("80 itens no leiaute antigo: %d folhas", contarPaginas(antes))
	salvarProva(t, "danfe-antigo-80-itens.pdf", antes)
}

// salvarProva grava o PDF em DANFE_SALVAR_DIR, quando definido — é como os
// PDFs de prova saem para conferência visual.
func salvarProva(t *testing.T, nome string, pdf []byte) {
	t.Helper()
	dir := os.Getenv("DANFE_SALVAR_DIR")
	if dir == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, nome), pdf, 0644); err != nil {
		t.Fatal(err)
	}
}
