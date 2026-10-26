package damdfe

import (
	"bytes"
	"os"
	"regexp"
	"testing"
)

var rePag = regexp.MustCompile(`/Type /Page[^s]`)

func gerar(t *testing.T, arq string, op Opcoes) []byte {
	t.Helper()
	compressao = false
	t.Cleanup(func() { compressao = true })
	x, err := os.ReadFile("testdata/" + arq)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := Gerar(x, op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("não é PDF")
	}
	return pdf
}

func tem(t *testing.T, pdf []byte, s string) {
	t.Helper()
	if !bytes.Contains(pdf, []byte(s)) {
		t.Errorf("PDF não contém %q", s)
	}
}

func TestLotacaoHomologacao(t *testing.T) {
	pdf := gerar(t, "lotacao_1nfe.xml", Opcoes{})
	if n := len(rePag.FindAll(pdf, -1)); n != 1 {
		t.Errorf("páginas = %d, quer 1", n)
	}
	tem(t, pdf, "3526 1012 3456 7800 0195 5800 1000 0000 0110 0000 0019")
	tem(t, pdf, "ABC1D23")
	tem(t, pdf, "AMBIENTE DE HOMOLOG")
	tem(t, pdf, "135260000000001")
	if bytes.Contains(pdf, []byte("CANCELADO")) {
		t.Error("marca CANCELADO sem pedir")
	}
}

func TestLote300(t *testing.T) {
	pdf := gerar(t, "lote_300nfe.xml", Opcoes{Cancelado: true, Encerrado: true})
	n := len(rePag.FindAll(pdf, -1))
	if n < 3 {
		t.Errorf("páginas = %d, quer >= 3", n)
	}
	tem(t, pdf, "Folha 1/"+string(rune('0'+n)))
	tem(t, pdf, "Folha 2/"+string(rune('0'+n)))
	tem(t, pdf, "XYZ9K88")
	tem(t, pdf, "REB1A11")
	tem(t, pdf, "Maria Souza")
	tem(t, pdf, "CANCELADO")
	tem(t, pdf, "ENCERRADO")
	if bytes.Contains(pdf, []byte("HOMOLOG")) {
		t.Error("faixa de homologação em tpAmb=1")
	}
}

func TestPreviaSemProtocolo(t *testing.T) {
	x := regexp.MustCompile(`(?s)<protMDFe.*</protMDFe>`).ReplaceAll(mustRead(t, "lotacao_1nfe.xml"), nil)
	x = bytes.Replace(x, []byte("<mdfeProc"), []byte("<mdfeProc"), 1)
	compressao = false
	defer func() { compressao = true }()
	pdf, err := Gerar(x, Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	tem(t, pdf, "SEM VALOR FISCAL - PR")
}

func mustRead(t *testing.T, a string) []byte {
	b, err := os.ReadFile("testdata/" + a)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestXMLInvalido(t *testing.T) {
	for _, x := range []string{"", "<mdfeProc><MDFe>", "<outra/>", `<MDFe xmlns="http://www.portalfiscal.inf.br/mdfe"><infMDFe Id="MDFe12"/></MDFe>`} {
		if _, err := Gerar([]byte(x), Opcoes{}); err == nil {
			t.Errorf("%q: esperava erro", x)
		}
	}
}

func TestAmostra(t *testing.T) {
	if os.Getenv("DAMDFE_AMOSTRA") == "" {
		t.Skip()
	}
	x := mustRead(t, "completo_tpemit1.xml")
	pdf, err := Gerar(x, Opcoes{Encerrado: true})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(os.Getenv("DAMDFE_AMOSTRA"), pdf, 0o644)
}

// QR Code é obrigatório (MOC DAMDFE §5): barcode + QR = 2 imagens por documento.
func TestQRDesenhado(t *testing.T) {
	for _, a := range []string{"lote_300nfe.xml", "lotacao_1nfe.xml"} {
		pdf := gerar(t, a, Opcoes{})
		if n := bytes.Count(pdf, []byte("/Subtype /Image")); n != 2 {
			t.Errorf("%s: imagens = %d, quer 2 (barcode + QR)", a, n)
		}
	}
}

// tpEmit=1 com todos os grupos opcionais: cada texto precisa sair no PDF.
func TestCompletoTpEmit1(t *testing.T) {
	pdf := gerar(t, "completo_tpemit1.xml", Opcoes{})
	for _, s := range []string{
		"123456789012", "11.222.333/0001-81", // CIOT + responsável
		"COMP-777", "22.333.444/0001-55", "350,00", // vale-pedágio
		"Contratante Alfa SA", "CT-2026-01", "9000,00", // contratante + contrato
		"Pagamento do frete", "Forma: ", "vista", // infPag resumido
		"Seguradora Delta", "APOL-998877", "AVERB-0001", // seguro
		"Antonio Terceiro", "529.982.247-25", "87654321", // proprietário terceiro
		"2 - TAC", "Prestador de servi", // tpTransp / tpEmit
		"Carga geral de teste", "73089090", // prodPred
		"Sao Paulo, Campinas", "UFs de percurso: MG, GO",
		"CT-e 3526 1012 3456 7800 0195 5700 1000 0000 0110 0000 0017",
		"NF-e 3526 1012 3456 7800 0195 5500 1000 0000 0110 0000 0011",
		"MDF-e 3526 1012 3456 7800 0195 5800 1000 0000 0210 0000 0024",
		"Qtde CT-e: 1     Qtde NF-e: 1     Qtde MDF-e: 1",
	} {
		tem(t, pdf, s)
	}
}
