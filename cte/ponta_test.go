package cte

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/cte/dacte"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// Build -> assinatura -> verificação -> XSD (Signature real) -> Autorizar
// (httptest, cStat 100) -> DACTE -> Cancelar e CartaCorrecao (httptest, 135).
func TestPonta_BuildAssinaAutorizaDACTEEventos(t *testing.T) {
	e := entradaBase()
	normal(&e) // regime normal, ICMS00 + IBS/CBS
	e.Toma3 = "3"
	e.Docs.NFe = []DocNFe{{Chave: ch("1")}, {Chave: ch("2")}}
	x, chave, err := Build(e)
	if err != nil {
		t.Fatal(err)
	}
	c := certSF(t)
	ass, err := sign.AssinarCTe(x, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := sign.VerificarAssinaturaNS(ass, "<infCte ", nsCTe, c); err != nil {
		t.Fatalf("assinatura: %v", err)
	}
	s := string(ass)
	if i, j := strings.Index(s, "</infCTeSupl>"), strings.Index(s, "<Signature"); i < 0 || j < i {
		t.Fatalf("Signature deve vir depois de </infCTeSupl> (%d, %d)", i, j)
	}
	validarXSD(t, ass)

	cl, _ := srvSF(t, "GO", 200, fixtureSF(t, "ret_autorizado.xml"))
	r, err := cl.Autorizar(context.Background(), ass)
	if err != nil {
		t.Fatal(err)
	}
	if r.CStat != "100" || len(r.XMLProc) == 0 {
		t.Fatalf("esperava 100 com cteProc: %+v", r)
	}
	pdf, err := dacte.Gerar(r.XMLProc, dacte.Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Error("saída não é PDF")
	}

	ev, _ := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	ctx := context.Background()
	if r, err := ev.Cancelar(ctx, c, chave, nProtSF, "Cancelamento por erro de digitacao"); err != nil || r.CStat != "135" {
		t.Errorf("Cancelar: %+v %v", r, err)
	}
	cc := []Correcao{{Grupo: "toma3", Campo: "toma", Valor: "3"}}
	if r, err := ev.CartaCorrecao(ctx, c, chave, cc, 1); err != nil || r.CStat != "135" {
		t.Errorf("CartaCorrecao: %+v %v", r, err)
	}
}
