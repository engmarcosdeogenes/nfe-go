package sign

import "testing"

func TestCanonicalizarReferencias(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{`<a>1.1/2&#34; D&#39;AGUA</a>`, `<a>1.1/2" D'AGUA</a>`},
		{`<a>&amp; &lt; &gt; &#xD;</a>`, `<a>&amp; &lt; &gt; &#xD;</a>`},
		{`<a>x&#x9;y&#xA;z</a>`, "<a>x\ty\nz</a>"},
		{`<a b="x&#34;y&#39;z&gt;">t</a>`, `<a b="x&quot;y'z>">t</a>`},
		{`<a b="&amp;&lt;">sem mudança</a>`, `<a b="&amp;&lt;">sem mudança</a>`},
		{`<a>&#233;</a>`, `<a>&#233;</a>`}, // referência fora da tabela fica como está
	}
	for _, c := range casos {
		if got := string(canonicalizarReferencias([]byte(c.entrada))); got != c.esperado {
			t.Errorf("canonicalizarReferencias(%q) = %q, esperado %q", c.entrada, got, c.esperado)
		}
	}
}
