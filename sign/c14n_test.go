package sign_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// Caso real (nota 110 da ATUAR, 11/11/2026): produto com polegada no nome saía
// com &#34; no XML, o digest era calculado sobre esses bytes e a SEFAZ, que
// calcula sobre a forma canônica ("), devolvia cStat 202. O digest aqui é
// conferido contra a C14N do lxml, não contra o próprio código.
func TestDigestIgualAoC14NComAspasNoTexto(t *testing.T) {
	xmlBytes, c := gerarXMLeTeste(t)
	xmlBytes = []byte(strings.Replace(string(xmlBytes),
		"<natOp>VENDA DE MERCADORIA</natOp>", // em homologação o builder troca o xProd
		`<natOp>VENDA ELETRODUTO 1.1/2&#34; CAIXA &#34;IP54&#34; D&#39;AGUA &amp; CIA</natOp>`, 1))
	if !strings.Contains(string(xmlBytes), "&#34;IP54") {
		t.Fatal("o XML de teste não ficou com as aspas escapadas")
	}

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatalf("AssinarNFe: %v", err)
	}
	if err := sign.VerificarAssinatura(assinado, c); err != nil {
		t.Fatalf("VerificarAssinatura: %v", err)
	}

	m := regexp.MustCompile(`<DigestValue>(.*?)</DigestValue>`).FindSubmatch(assinado)
	if m == nil {
		t.Fatal("sem DigestValue")
	}
	if got := digestC14NLxml(t, assinado); got != string(m[1]) {
		t.Fatalf("DigestValue %s difere do C14N oficial %s (a SEFAZ devolveria cStat 202)", m[1], got)
	}
}

const scriptDigestC14N = `
import sys, re, hashlib, base64
from lxml import etree
raw = open(sys.argv[1], 'rb').read()
# infNFe isolado com o xmlns herdado explícito: o c14n do lxml num elemento
# interno marca os filhos com xmlns="" e não serve de referência.
inf = re.search(rb'<infNFe .*?</infNFe>', raw, re.S).group(0)
inf = inf.replace(b'<infNFe ', b'<infNFe xmlns="http://www.portalfiscal.inf.br/nfe" ', 1)
c14n = etree.tostring(etree.fromstring(inf), method='c14n')
print(base64.b64encode(hashlib.sha1(c14n).digest()).decode(), end='')
`

// digestC14NLxml calcula o DigestValue como a SEFAZ: C14N inclusiva do infNFe.
// Sem python + lxml o teste falha (não pula): sem isso ele não prova nada.
func digestC14NLxml(t *testing.T, assinado []byte) string {
	t.Helper()
	var py string
	for _, cand := range []string{os.Getenv("NFE_PYTHON"), "python3", "python"} {
		if cand == "" {
			continue
		}
		if p, err := exec.LookPath(cand); err == nil && exec.Command(p, "-c", "import lxml").Run() == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Fatal("instale python3 + lxml ou defina NFE_PYTHON")
	}
	arq := filepath.Join(t.TempDir(), "nfe.xml")
	if err := os.WriteFile(arq, assinado, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-c", scriptDigestC14N, arq).CombinedOutput() // #nosec G204 -- python do ambiente de teste
	if err != nil {
		t.Fatalf("lxml: %v\n%s", err, out)
	}
	return string(out)
}
