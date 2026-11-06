package danfe_test

import (
	"os"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/danfe"
)

// O <cobr> que o builder monta (EntradaCobranca) chega no quadro "Fatura /
// Duplicatas" do DANFE, e a nota de entrada (tpNF=0) sai marcada como entrada.
// DANFE_SALVAR=1 grava o PDF em testdata/ para conferir a olho.
func TestDANFE_DuplicatasEEntradaVindasDoBuilder(t *testing.T) {
	e := entradaDANFETeste()
	e.TpNF = "0"
	for i := range e.Itens {
		e.Itens[i].CFOP = "1102"
	}
	venc := e.DhEmi
	e.Cobranca = &builder.EntradaCobranca{NFat: "42", VOrig: 3400, Duplicatas: []builder.EntradaDuplicata{
		{DVenc: venc.AddDate(0, 0, 30), VDup: 1133.33},
		{DVenc: venc.AddDate(0, 0, 60), VDup: 1133.33},
		{DVenc: venc.AddDate(0, 0, 90), VDup: 1133.34},
	}}
	xmlAssinado := assinarEntrada(t, e)

	dados, err := danfe.ParseNFeXML(xmlAssinado)
	if err != nil {
		t.Fatalf("ParseNFeXML: %v", err)
	}
	if dados.TipoNF != "0" {
		t.Errorf("TipoNF = %q, esperava 0 (entrada)", dados.TipoNF)
	}
	if len(dados.Duplicatas) != 3 || dados.Duplicatas[0].Num != "001" || dados.Duplicatas[2].Valor != 1133.34 {
		t.Fatalf("duplicatas no DANFE = %+v", dados.Duplicatas)
	}
	if want := venc.AddDate(0, 0, 30).Format("02/01/2006"); dados.Duplicatas[0].Vencimento != want {
		t.Errorf("vencimento = %q, esperava %q", dados.Duplicatas[0].Vencimento, want)
	}
	pdf, err := danfe.Gerar(xmlAssinado, false)
	if err != nil || len(pdf) == 0 {
		t.Fatalf("Gerar: %v", err)
	}
	if os.Getenv("DANFE_SALVAR") != "" {
		_ = os.WriteFile("testdata/danfe_duplicatas_entrada.pdf", pdf, 0o644)
	}
}
