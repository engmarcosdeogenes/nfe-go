package dacte

import (
	"bytes"
	"os"
	"regexp"
	"testing"
)

var rePag = regexp.MustCompile(`/Type /Page[^s]`)

func ler(t *testing.T, arq string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + arq)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func gerarBytes(t *testing.T, x []byte, op Opcoes) []byte {
	t.Helper()
	compressao = false
	t.Cleanup(func() { compressao = true })
	pdf, err := Gerar(x, op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("não é PDF")
	}
	return pdf
}

func tem(t *testing.T, pdf []byte, s ...string) {
	t.Helper()
	for _, x := range s {
		if !bytes.Contains(pdf, []byte(x)) {
			t.Errorf("PDF não contém %q", x)
		}
	}
}

func TestSimplesHomologacao(t *testing.T) {
	pdf := gerarBytes(t, ler(t, "cte_simples_homolog.xml"), Opcoes{})
	if n := len(rePag.FindAll(pdf, -1)); n != 1 {
		t.Errorf("páginas = %d, quer 1", n)
	}
	tem(t, pdf, "3526 1012 3456 7800 0195 5700 1000 0000 1110 0000 0119",
		"COMERCIO DESTINO LTDA", "Destinat", "1.500,00", "90 - Simples Nacional", "12345678",
		"AMBIENTE DE HOMOLOG", "135260000000123", "PESO BRUTO: 1.200,0000 KG", "Folha 1/1", "PED-778")
	if bytes.Contains(pdf, []byte("CANCELADO")) {
		t.Error("marca CANCELADO sem pedir")
	}
}

func TestNormalIBSCBSToma4(t *testing.T) {
	pdf := gerarBytes(t, ler(t, "cte_normal_ibscbs_toma4.xml"), Opcoes{})
	tem(t, pdf, "TOMADOR OUTROS COMERCIAL SA", "99.888.777/0001-66", "Outros", "00", "180,00", "12,00",
		"000001", "13,50", "EXPEDIDOR LOGISTICA", "RECEBEDOR ARMAZENS", "Ordem de coleta 1/55")
	if bytes.Contains(pdf, []byte("HOMOLOG")) {
		t.Error("faixa de homologação em tpAmb=1")
	}
}

func TestSubcontratacao300(t *testing.T) {
	pdf := gerarBytes(t, ler(t, "cte_subcontratacao_300nfe.xml"), Opcoes{Cancelado: true})
	n := len(rePag.FindAll(pdf, -1))
	if n < 4 {
		t.Errorf("páginas = %d, quer >= 4", n)
	}
	tem(t, pdf, "Folha 1/", "Folha 2/", "3526 1012 3456 7800 0195 5500 1000 0003 0010 0000 3000",
		"1 - Subcontrata", "TRANSPORTADORA ANTERIOR SA", "3126 1044 5556 6600 0122 5700 1000 0000 0910 0000 0090",
		"CANCELADO", "20,00", "144,00")
}

func TestPreviaSemProtocolo(t *testing.T) {
	x := regexp.MustCompile(`(?s)<protCTe.*</protCTe>`).ReplaceAll(ler(t, "cte_normal_ibscbs_toma4.xml"), nil)
	x = bytes.Replace(x, []byte("<cteProc"), []byte("<cteProc"), 1)
	pdf := gerarBytes(t, x, Opcoes{})
	tem(t, pdf, "SEM VALOR FISCAL - PR")
}

func TestXMLInvalido(t *testing.T) {
	for _, x := range []string{"", "<cteProc><CTe>", "<outra/>",
		`<CTe xmlns="http://www.portalfiscal.inf.br/cte"><infCte Id="CTe12"/></CTe>`,
		`<CTe xmlns="http://www.portalfiscal.inf.br/cte"><infCte Id="CTe35261012345678000195670010000000111000000119"><ide><mod>67</mod></ide></infCte></CTe>`} {
		if _, err := Gerar([]byte(x), Opcoes{}); err == nil {
			t.Errorf("%q: esperava erro", x)
		}
	}
}

// QR Code (MOC DACTE §2.19) + código de barras: 2 imagens por folha, registradas uma vez.
func TestQRDesenhado(t *testing.T) {
	for _, a := range []string{"cte_subcontratacao_300nfe.xml", "cte_simples_homolog.xml"} {
		pdf := gerarBytes(t, ler(t, a), Opcoes{})
		if n := bytes.Count(pdf, []byte("/Subtype /Image")); n != 2 {
			t.Errorf("%s: imagens = %d, quer 2 (barcode + QR)", a, n)
		}
	}
}

func TestAmostra(t *testing.T) {
	if os.Getenv("DACTE_AMOSTRA") == "" {
		t.Skip()
	}
	pdf, err := Gerar(ler(t, "cte_normal_ibscbs_toma4.xml"), Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(os.Getenv("DACTE_AMOSTRA"), pdf, 0o644)
}

func TestICMSUFFim(t *testing.T) {
	x := bytes.Replace(ler(t, "cte_normal_ibscbs_toma4.xml"), []byte("<IBSCBS>"),
		[]byte("<ICMSUFFim><vBCUFFim>1500.00</vBCUFFim><pFCPUFFim>2.00</pFCPUFFim><pICMSUFFim>18.00</pICMSUFFim><pICMSInter>12.00</pICMSInter>"+
			"<vFCPUFFim>30.00</vFCPUFFim><vICMSUFFim>63.00</vICMSUFFim><vICMSUFIni>27.00</vICMSUFIni></ICMSUFFim><IBSCBS>"), 1)
	tem(t, gerarBytes(t, x, Opcoes{}), "ICMS UF fim", "63,00", "27,00", "18,00", "30,00")
}
