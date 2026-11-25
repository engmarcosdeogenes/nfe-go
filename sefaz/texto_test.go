package sefaz_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// pythonLxml acha um python com lxml: NFE_PYTHON, python3, python. Falha (não pula)
// se não houver, porque sem validação XSD o teste não prova nada.
func pythonLxml(t *testing.T) string {
	t.Helper()
	for _, c := range []string{os.Getenv("NFE_PYTHON"), "python3", "python"} {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil && exec.Command(p, "-c", "import lxml").Run() == nil {
			return p
		}
	}
	t.Fatal("instale python3 + lxml ou defina NFE_PYTHON")
	return ""
}

const scriptXSDEvento = `
import sys
from lxml import etree
sch = etree.XMLSchema(etree.parse(sys.argv[1]))
sch.validate(etree.parse(sys.argv[2]))
for e in sch.error_log:
    print(e.line, e.message)
`

// validarEnvEvento valida o <envEvento> do corpo SOAP enviado contra o XSD oficial.
func validarEnvEvento(t *testing.T, enviado, xsd string) string {
	t.Helper()
	i, j := strings.Index(enviado, "<envEvento"), strings.Index(enviado, "</envEvento>")
	if i < 0 || j < 0 {
		t.Fatalf("sem envEvento em %s", enviado)
	}
	env := enviado[i : j+len("</envEvento>")]
	arq := filepath.Join(t.TempDir(), "env.xml")
	if err := os.WriteFile(arq, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(pythonLxml(t), "-c", scriptXSDEvento, filepath.Join("..", "testdata", "schemas", "nfe", xsd), arq).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, out)
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		t.Errorf("XSD %s inválido:\n%s\n%s", xsd, s, env)
	}
	return env
}

func TestNormalizarTexto(t *testing.T) {
	casos := []struct{ nome, in, want string }{
		{"enter tab e espaços", "Corrigir o\r\nendereço\tpara  Rua A & B  ", "Corrigir o endereço para Rua A & B"},
		{"aspas curvas", "“x” ‘y’", `"x" 'y'`},
		{"travessão e reticências", "a – b — c…", "a - b - c..."},
		{"emoji", "ok 😀 feito", "ok feito"},
		{"acentos", "ação é çã", "ação é çã"},
	}
	for _, c := range casos {
		got := sefaz.NormalizarTexto(c.in)
		if got != c.want {
			t.Errorf("%s: %q, quero %q", c.nome, got, c.want)
		}
		if again := sefaz.NormalizarTexto(got); again != got {
			t.Errorf("%s: não idempotente: %q", c.nome, again)
		}
	}
}

const textoSujo = "Corrigir endereço do destinatário:\nRua “Sete” & Cia — nº 10 <fundos> \t "

func TestCartaCorrecao_TextoSujo_XSD(t *testing.T) {
	falsa := &sefazEventoFalsa{cStat: "135"}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, falsa)
	c := certTeste(t)
	if _, err := cl.CartaCorrecao(context.Background(), c, chaveNFe, textoSujo, sefaz.XCondUsoCartaCorrecao, 1); err != nil {
		t.Fatal(err)
	}
	env := validarEnvEvento(t, falsa.enviado, filepath.Join("cce", "envCCe_v1.00.xsd"))
	i := strings.Index(env, "<xCorrecao>")
	j := strings.Index(env, "</xCorrecao>")
	if i < 0 || j < 0 {
		t.Fatalf("sem xCorrecao: %s", env)
	}
	got := env[i+len("<xCorrecao>") : j]
	// xml.EscapeText emite &#34; para aspas duplas
	want := "Corrigir endereço do destinatário: Rua &#34;Sete&#34; &amp; Cia - nº 10 &lt;fundos&gt;"
	if got != want {
		t.Errorf("xCorrecao = %q, quero %q", got, want)
	}
	if err := sign.VerificarAssinaturaNS([]byte(env), "<infEvento ", "http://www.portalfiscal.inf.br/nfe", c); err != nil {
		t.Errorf("assinatura: %v", err)
	}
}

func TestCartaCorrecao_TamanhoAposNormalizar(t *testing.T) {
	falsa := &sefazEventoFalsa{cStat: "135"}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, falsa)
	c := certTeste(t)
	curto := strings.Repeat("a", 14) + strings.Repeat(" \n", 50)
	if _, err := cl.CartaCorrecao(context.Background(), c, chaveNFe, curto, sefaz.XCondUsoCartaCorrecao, 1); err == nil || !strings.Contains(err.Error(), "mínimo 15") {
		t.Errorf("esperava erro mínimo 15, veio %v", err)
	}
	if _, err := cl.CartaCorrecao(context.Background(), c, chaveNFe, strings.Repeat("a", 1000)+"   \n ", sefaz.XCondUsoCartaCorrecao, 1); err != nil {
		t.Errorf("1000 chars + espaços deveria passar: %v", err)
	}
	if _, err := cl.CartaCorrecao(context.Background(), c, chaveNFe, strings.Repeat("a", 1001), sefaz.XCondUsoCartaCorrecao, 1); err == nil {
		t.Error("1001 chars deveria falhar")
	}
}

func TestCancelar_TextoSujo_XSD(t *testing.T) {
	falsa := &sefazEventoFalsa{cStat: "135"}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, falsa)
	if _, err := cl.Cancelar(context.Background(), chaveNFe, "152260000000111", "Erro de digitação\r\nno pedido “A” & B — cancelar \t ", certTeste(t)); err != nil {
		t.Fatal(err)
	}
	env := validarEnvEvento(t, falsa.enviado, filepath.Join("canc", "envEventoCancNFe_v1.00.xsd"))
	if !strings.Contains(env, "<xJust>Erro de digitação no pedido &#34;A&#34; &amp; B - cancelar</xJust>") {
		t.Errorf("xJust inesperado: %s", env)
	}
}
