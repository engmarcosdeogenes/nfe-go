package builder_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Valida o XML de cada variante contra o XSD oficial (PL_010 v1.30). O que a
// SEFAZ recusa com cStat 225 sem dizer o campo aparece aqui com linha e motivo.
// Pula quando não há python com lxml. Falta <Signature> é esperado: o builder
// não assina.
const scriptXSD = `
import sys
from lxml import etree
sch = etree.XMLSchema(etree.parse(sys.argv[1]))
doc = etree.parse(sys.argv[2])
sch.validate(doc)
for e in sch.error_log:
    print(e.line, e.message)
`

func validarXSD(t *testing.T, xmlBytes []byte) {
	t.Helper()
	py := ""
	for _, cand := range []string{"python3", "python"} {
		if p, err := exec.LookPath(cand); err == nil && exec.Command(p, "-c", "import lxml").Run() == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Skip("python com lxml não encontrado (pip install lxml)")
	}
	dir := t.TempDir()
	arq := filepath.Join(dir, "nfe.xml")
	if err := os.WriteFile(arq, xmlBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	saida, err := exec.Command(py, "-c", scriptXSD, filepath.Join("testdata", "xsd", "nfe_v4.00.xsd"), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, saida)
	}
	for _, linha := range strings.Split(strings.TrimSpace(string(saida)), "\n") {
		if linha != "" && !strings.Contains(linha, "Signature") {
			t.Errorf("XSD: %s", linha)
		}
	}
}

func TestXSD_VariantesDoBuilder(t *testing.T) {
	item := func(nome string, icms builder.EntradaICMS, cfop string) builder.EntradaItem {
		return builder.EntradaItem{
			CProd: "P1", CEAN: "SEM GTIN", Nome: nome, NCM: "73089090", CFOP: cfop,
			Unidade: "UN", Quantidade: 2, VUnitario: 100, ICMS: icms,
		}
	}
	com := func(base builder.EntradaNFe, it builder.EntradaItem) builder.EntradaNFe {
		base.Itens = []builder.EntradaItem{it}
		return base
	}

	ibs := item("IBSCBS", builder.EntradaICMS{CST: "00", Aliq: 12}, "5102")
	ibs.IBSCBS = &builder.EntradaIBSCBS{CST: "000", ClassTrib: "000001", AliqIBSUF: 0.1, AliqCBS: 0.9}
	ipi := item("IPI", builder.EntradaICMS{CST: "00", Aliq: 12}, "5101")
	ipi.IPI = &builder.EntradaIPI{CEnq: "999", CST: "50", Aliq: 5}
	difal := item("DIFAL", builder.EntradaICMS{CST: "00", Aliq: 7}, "6108")
	difal.ICMSUFDest = &builder.EntradaICMSUFDest{AliqInterna: 18, AliqInterestadual: 7, AliqFCP: 2}

	destISENTO := entradaCRT3ComItemPadrao()
	destISENTO.Dest.IndIEDest = "9"
	destISENTO.Dest.IE = "ISENTO"
	destCPF := entradaExemplo()
	destCPF.Dest.CNPJ, destCPF.Dest.CPF, destCPF.Dest.IndIEDest, destCPF.Dest.IE = "", "12345678901", "9", ""
	cartao := entradaExemplo()
	cartao.Pagamento = []builder.EntradaPagamento{{Forma: "03", Valor: 200, TBand: "01", CNPJCredenciadora: "11222333000181"}}
	interestadual := com(entradaCRT3(), difal)
	interestadual.Dest.End.UF = "SP"
	interestadual.Dest.IndIEDest = "9"

	casos := map[string]builder.EntradaNFe{
		"exemplo":       entradaExemplo(),
		"crt3":          entradaCRT3ComItemPadrao(),
		"crt3_ibscbs":   com(entradaCRT3(), ibs),
		"crt3_ipi":      com(entradaCRT3(), ipi),
		"crt3_cst10_st": com(entradaCRT3(), item("ST", builder.EntradaICMS{CST: "10", Aliq: 12, PMVAST: 40, AliqST: 18}, "6401")),
		"crt3_cst20":    com(entradaCRT3(), item("RED", builder.EntradaICMS{CST: "20", Aliq: 12, PRedBC: 10}, "5102")),
		"crt3_cst40":    com(entradaCRT3(), item("ISE", builder.EntradaICMS{CST: "40"}, "5102")),
		"crt3_cst60":    com(entradaCRT3(), item("ST60", builder.EntradaICMS{CST: "60", VBCSTRet: 700, PST: 18, VICMSSTRet: 126}, "5405")),
		"crt3_difal":    interestadual,
		"dest_isento":   destISENTO,
		"dest_cpf":      destCPF,
		"pagto_cartao":  cartao,
		"nfce":          entradaNFCe(),
		"nfce_csosn101": com(entradaNFCe(), item("SN101", builder.EntradaICMS{CSOSN: "101", Aliq: 2.5}, "5102")),
		"nfce_csosn201": com(entradaNFCe(), item("SN201", builder.EntradaICMS{CSOSN: "201", Aliq: 2.5, PMVAST: 40, AliqST: 18}, "5102")),
		"nfce_csosn500": com(entradaNFCe(), item("SN500", builder.EntradaICMS{CSOSN: "500", VBCSTRet: 9, PST: 19, VICMSSTRet: 1.71}, "5405")),
	}
	for nome, entrada := range casos {
		t.Run(nome, func(t *testing.T) {
			xmlBytes, _, err := builder.Build(entrada)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			validarXSD(t, xmlBytes)
		})
	}
}
