package cte

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const chaveSF = "52261011222333000181570010000000011123456780"

const scriptValSF = `
import sys, copy
from lxml import etree
sch = etree.XMLSchema(etree.parse(sys.argv[1]))
doc = etree.parse(sys.argv[2])
if len(sys.argv) > 3:
    det = doc.getroot().find('.//{*}detEvento')
    doc = etree.ElementTree(copy.deepcopy(det[0]))
sch.validate(doc)
for e in sch.error_log:
    print(e.line, e.message)
`

// valSF valida x contra o XSD (ou o 1º filho de detEvento). Falha sem python+lxml.
func valSF(t *testing.T, xsd string, x []byte, filhoDeDetEvento bool) {
	t.Helper()
	py := pythonLxml(t)
	arq := filepath.Join(t.TempDir(), "x.xml")
	if err := os.WriteFile(arq, x, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-c", scriptValSF, filepath.Join("..", "testdata", "schemas", "cte", xsd), arq}
	if filhoDeDetEvento {
		args = append(args, "filho")
	}
	saida, err := exec.Command(py, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("validador: %v\n%s", err, saida)
	}
	if s := strings.TrimSpace(string(saida)); s != "" {
		t.Errorf("XSD %s inválido:\n%s\nXML: %s", xsd, s, x)
	}
}

func fixtureSF(t *testing.T, nome string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nome))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type reqVistaSF struct {
	path, ct, action string
	corpo            []byte
}

func srvSF(t *testing.T, uf string, status int, corpo []byte) (*Cliente, *reqVistaSF) {
	t.Helper()
	v := &reqVistaSF{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v.path, v.ct, v.action = r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("SOAPAction")
		v.corpo, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		w.Write(corpo)
	}))
	t.Cleanup(s.Close)
	return NovoClienteHTTP(uf, "2", s.URL, s.Client()), v
}

func cteAssinadoSF() []byte {
	return []byte(`<?xml version="1.0"?><CTe xmlns="http://www.portalfiscal.inf.br/cte"><infCte Id="CTe` + chaveSF + `" versao="4.00"><ide><tpAmb>2</tpAmb></ide></infCte></CTe>`)
}

// conteudoSOAP devolve o texto de <cteDadosMsg> (base64) ou o XML interno.
func dadosMsgSF(t *testing.T, soap []byte) string {
	t.Helper()
	var env struct {
		Body struct {
			Msg struct {
				Inner string `xml:",innerxml"`
			} `xml:"cteDadosMsg"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(soap, &env); err != nil {
		t.Fatal(err)
	}
	return env.Body.Msg.Inner
}

func TestSefaz_AutorizarGzipBase64eProc(t *testing.T) {
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_autorizado.xml"))
	r, err := cl.Autorizar(context.Background(), cteAssinadoSF())
	if err != nil {
		t.Fatal(err)
	}
	if v.path != "/ws/CTeRecepcaoSincV4/CTeRecepcaoSincV4.asmx" {
		t.Errorf("path %s", v.path)
	}
	wantAct := "http://www.portalfiscal.inf.br/cte/wsdl/CTeRecepcaoSincV4/cteRecepcao"
	if v.action != wantAct || !strings.Contains(v.ct, `action="`+wantAct+`"`) || !strings.Contains(v.ct, "application/soap+xml") {
		t.Errorf("action/ct: %q %q", v.action, v.ct)
	}
	zb, err := base64.StdEncoding.DecodeString(dadosMsgSF(t, v.corpo))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zb))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	enviado, _ := io.ReadAll(zr)
	if !strings.HasPrefix(string(enviado), "<CTe ") || !strings.Contains(string(enviado), chaveSF) || strings.Contains(string(enviado), "<?xml") {
		t.Errorf("CTe enviado: %s", enviado)
	}
	if r.CStat != "100" || r.NProt != "926260000000001" || r.ChCTe != chaveSF || r.DhRecbto == "" {
		t.Errorf("%+v", r)
	}
	px := string(r.XMLProc)
	if !strings.HasPrefix(px, `<cteProc versao="4.00" xmlns="http://www.portalfiscal.inf.br/cte"><CTe `) || !strings.Contains(px, "<protCTe") || !strings.HasSuffix(px, "</protCTe></cteProc>") {
		t.Errorf("cteProc: %s", px)
	}
	if xml.Unmarshal(r.XMLProc, &struct{ XMLName xml.Name }{}) != nil {
		t.Error("cteProc mal formado")
	}
}

func TestSefaz_AutorizarRejeicaoNaoEErroGo(t *testing.T) {
	cl, _ := srvSF(t, "SP", 200, fixtureSF(t, "ret_rejeicao.xml"))
	r, err := cl.Autorizar(context.Background(), cteAssinadoSF())
	if err != nil || r.CStat != "539" || r.XMLProc != nil || !strings.Contains(r.XMotivo, "Duplicidade") {
		t.Errorf("%+v %v", r, err)
	}
}

func TestSefaz_ErrosDeTransporteEResposta(t *testing.T) {
	ctx := context.Background()
	cl, _ := srvSF(t, "SP", 500, []byte("boom"))
	if _, err := cl.Autorizar(ctx, cteAssinadoSF()); err == nil {
		t.Error("HTTP 500")
	}
	for _, lixo := range []string{"", "lixo <<<", "<html>oi</html>", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body/></soap:Envelope>`} {
		cl, _ := srvSF(t, "SP", 200, []byte(lixo))
		if _, err := cl.Autorizar(ctx, cteAssinadoSF()); err == nil {
			t.Errorf("lixo %q deveria dar erro", lixo)
		}
		if _, err := cl.Consultar(ctx, chaveSF); err == nil {
			t.Errorf("consulta lixo %q", lixo)
		}
		if _, _, err := cl.StatusServico(ctx); err == nil {
			t.Errorf("status lixo %q", lixo)
		}
	}
}

func TestXSD_SefazConsultaEStatus(t *testing.T) {
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_consulta.xml"))
	r, err := cl.Consultar(context.Background(), chaveSF)
	if err != nil || r.CStat != "100" || r.NProt != "926260000000001" || r.ProtXML == nil {
		t.Fatalf("%+v %v", r, err)
	}
	if v.path != "/ws/CTeConsultaV4/CTeConsultaV4.asmx" || v.action != "http://www.portalfiscal.inf.br/cte/wsdl/CTeConsultaV4/cteConsultaCT" {
		t.Errorf("%s %s", v.path, v.action)
	}
	valSF(t, "consSitCTe_v4.00.xsd", []byte(dadosMsgSF(t, v.corpo)), false)
	if _, err := cl.Consultar(context.Background(), "123"); err == nil {
		t.Error("chave curta")
	}

	cl, v = srvSF(t, "GO", 200, fixtureSF(t, "ret_status.xml"))
	cs, xm, err := cl.StatusServico(context.Background())
	if err != nil || cs != "107" || xm == "" {
		t.Fatalf("%s %s %v", cs, xm, err)
	}
	if !strings.Contains(string(v.corpo), "<cUF>52</cUF>") {
		t.Errorf("cUF: %s", v.corpo)
	}
	valSF(t, "consStatServCTe_v4.00.xsd", []byte(dadosMsgSF(t, v.corpo)), false)
}
