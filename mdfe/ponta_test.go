package mdfe

import (
	"bytes"
	"context"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/mdfe/damdfe"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// Build -> assinatura -> verificação -> XSD (com Signature real) -> Autorizar
// (httptest, cStat 100) -> DAMDFE. Prova que builder, sign e damdfe encaixam.
func TestPonta_BuildAssinaAutorizaDAMDFE(t *testing.T) {
	e := entradaBase()
	e.Descargas[0].NFes = append(e.Descargas[0].NFes, chaveNFe("2"))
	x, _, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	c := certEvTeste(t)
	ass, err := sign.AssinarMDFe(x, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := sign.VerificarAssinaturaNS(ass, "<infMDFe ", nsMDFe, c); err != nil {
		t.Fatalf("assinatura: %v", err)
	}
	validarXSD(t, ass)

	cl, _ := srv(t, 200, fixture(t, "ret_autorizado.xml"))
	r, err := cl.Autorizar(context.Background(), ass)
	if err != nil {
		t.Fatal(err)
	}
	if r.CStat != "100" || len(r.XMLProc) == 0 {
		t.Fatalf("esperava 100 com mdfeProc: %+v", r)
	}
	pdf, err := damdfe.Gerar(r.XMLProc, damdfe.Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Errorf("saída não é PDF")
	}
}
