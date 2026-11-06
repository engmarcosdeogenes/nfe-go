package cte

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/cte/dacte"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// BuildOS -> assinatura -> verificação -> XSD (Signature real) -> AutorizarOS
// (httptest, cStat 100) -> cteOSProc no XSD -> DACTE OS -> Cancelar (135).
func TestPonta_OS_BuildAssinaAutorizaDACTE(t *testing.T) {
	e := entradaOSBase()
	normalOS(&e)
	e.DocRef = []DocRefOS{{ChBPe: ch("6")}}
	x, chave, err := BuildOS(e)
	if err != nil {
		t.Fatal(err)
	}
	c := certSF(t)
	ass, err := sign.AssinarCTeOS(x, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := sign.VerificarAssinaturaNS(ass, "<infCte ", nsCTe, c); err != nil {
		t.Fatalf("assinatura: %v", err)
	}
	s := string(ass)
	i, j, k := strings.Index(s, "</infCTeSupl>"), strings.Index(s, "<Signature"), strings.Index(s, "</CTeOS>")
	if i < 0 || j < i || k < j || !strings.HasSuffix(s, "</Signature></CTeOS>") {
		t.Fatalf("Signature deve ficar entre </infCTeSupl> e </CTeOS> (%d, %d, %d)", i, j, k)
	}
	validarXSDOS(t, ass)

	resp := bytes.ReplaceAll(fixtureSF(t, "ret_os_autorizado.xml"), []byte("52261011222333000181670010000000771123456780"), []byte(chave))
	cl, _ := srvSF(t, "GO", 200, resp)
	r, err := cl.AutorizarOS(context.Background(), ass)
	if err != nil {
		t.Fatal(err)
	}
	if r.CStat != "100" || r.ChCTe != chave || len(r.XMLProc) == 0 {
		t.Fatalf("esperava 100 com cteOSProc: %+v", r)
	}
	valSF(t, "procCTeOS_v4.00.xsd", r.XMLProc, false)

	pdf, err := dacte.GerarOS(r.XMLProc, dacte.Opcoes{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Error("saída não é PDF")
	}

	ev, _ := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	if r, err := ev.Cancelar(context.Background(), c, chave, nProtSF, "Cancelamento por erro de digitacao"); err != nil || r.CStat != "135" {
		t.Errorf("Cancelar: %+v %v", r, err)
	}
}

// AssinarCTe (57) não serve ao OS: anexaria a Signature fora da raiz.
func TestSign_CTeOSNaoUsaAssinarCTe(t *testing.T) {
	x, _, err := BuildOS(entradaOSBase())
	if err != nil {
		t.Fatal(err)
	}
	ass, err := sign.AssinarCTe(x, certSF(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(string(ass), "</Signature></CTeOS>") {
		t.Error("AssinarCTe não deveria posicionar a Signature dentro de CTeOS")
	}
}
