package mdfe

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

const chaveTeste = "52261011222333000181580010000000011123456780"

// scriptValSefazXSD valida um XML (ou, com 3º argumento, o 1º filho do 1º
// <detEvento>) contra um XSD. Imprime cada erro; sem saída = válido.
const scriptValSefazXSD = `
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

// valSefazXSD falha (não pula) se python+lxml não existir: sem validação o teste não prova nada.
func valSefazXSD(t *testing.T, xsd string, x []byte, filhoDeDetEvento bool) {
	t.Helper()
	py := pythonLxml(t)
	arq := filepath.Join(t.TempDir(), "x.xml")
	if err := os.WriteFile(arq, x, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-c", scriptValSefazXSD, filepath.Join("..", "testdata", "schemas", "mdfe", xsd), arq}
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

func fixture(t *testing.T, nome string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nome))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// srv devolve um servidor que responde `status`/`corpo` e guarda path, headers e corpo da requisição.
type reqVista struct {
	path, ct, action string
	corpo            []byte
}

func srv(t *testing.T, status int, corpo []byte) (*Cliente, *reqVista) {
	t.Helper()
	v := &reqVista{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v.path, v.ct, v.action = r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("SOAPAction")
		v.corpo, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		w.Write(corpo)
	}))
	t.Cleanup(s.Close)
	return NovoClienteHTTP("2", s.URL, s.Client()), v
}

func TestSefaz_NovoClienteTpAmbInvalido(t *testing.T) {
	pfx, _ := cert.GerarCertificadoTeste("11222333000181", "x")
	c, _ := cert.CarregarPFXBytes(pfx, "x")
	for _, a := range []string{"", "0", "3", "homolog"} {
		if _, err := NovoCliente(a, c); err == nil {
			t.Errorf("tpAmb %q deveria falhar", a)
		}
	}
	for _, a := range []string{"1", "2"} {
		if _, err := NovoCliente(a, c); err != nil {
			t.Errorf("tpAmb %q: %v", a, err)
		}
	}
}

func TestSefaz_URLeSOAPActionPorServicoEAmbiente(t *testing.T) {
	// SOAPAction não confirmada por WSDL (exige mTLS): valores = MOC 3.00b §4 + sped-mdfe.
	casos := []struct {
		s      servicoMDFe
		nome   string
		metodo string
	}{
		{svcRecepcaoSinc, "MDFeRecepcaoSinc", "mdfeRecepcao"},
		{svcConsulta, "MDFeConsulta", "mdfeConsultaMDF"},
		{svcRecepcaoEvento, "MDFeRecepcaoEvento", "mdfeRecepcaoEvento"},
		{svcStatusServico, "MDFeStatusServico", "mdfeStatusServicoMDF"},
		{svcConsNaoEnc, "MDFeConsNaoEnc", "mdfeConsNaoEnc"},
	}
	for _, k := range casos {
		if got, want := k.s.url("1", ""), "https://mdfe.svrs.rs.gov.br/ws/"+k.nome+"/"+k.nome+".asmx"; got != want {
			t.Errorf("prod %s: %s != %s", k.nome, got, want)
		}
		if got, want := k.s.url("2", ""), "https://mdfe-homologacao.svrs.rs.gov.br/ws/"+k.nome+"/"+k.nome+".asmx"; got != want {
			t.Errorf("homolog %s: %s != %s", k.nome, got, want)
		}
		if got, want := k.s.soapAction(), "http://www.portalfiscal.inf.br/mdfe/wsdl/"+k.nome+"/"+k.metodo; got != want {
			t.Errorf("action %s: %s != %s", k.nome, got, want)
		}
	}
}

func mdfeAssinadoTeste() []byte {
	return []byte(`<?xml version="1.0"?><MDFe xmlns="http://www.portalfiscal.inf.br/mdfe"><infMDFe Id="MDFe` + chaveTeste + `" versao="3.00"><tpAmb>2</tpAmb></infMDFe></MDFe>`)
}

func TestSefaz_AutorizarGzipBase64eProc(t *testing.T) {
	cl, v := srv(t, 200, fixture(t, "ret_autorizado.xml"))
	r, err := cl.Autorizar(context.Background(), mdfeAssinadoTeste())
	if err != nil {
		t.Fatal(err)
	}
	if v.path != "/ws/MDFeRecepcaoSinc/MDFeRecepcaoSinc.asmx" {
		t.Errorf("path %s", v.path)
	}
	wantAct := "http://www.portalfiscal.inf.br/mdfe/wsdl/MDFeRecepcaoSinc/mdfeRecepcao"
	if v.action != wantAct || !strings.HasPrefix(v.ct, "application/soap+xml; charset=utf-8") || !strings.Contains(v.ct, `action="`+wantAct+`"`) {
		t.Errorf("headers: ct=%q action=%q", v.ct, v.action)
	}
	// o corpo recebido: mdfeDadosMsg = base64(gzip(MDFe sem <?xml?>))
	b64 := strings.TrimSpace(string(elementoTexto(t, v.corpo, "mdfeDadosMsg")))
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	dec, _ := io.ReadAll(zr)
	if !bytes.HasPrefix(dec, []byte("<MDFe ")) || bytes.Contains(dec, []byte("<?xml")) {
		t.Errorf("payload descompactado inesperado: %s", dec)
	}
	if r.CStat != "100" || r.NProt != "926260000000001" || r.ChMDFe != chaveTeste || r.DhRecbto == "" {
		t.Errorf("retorno: %+v", r)
	}
	s := string(r.XMLProc)
	if !strings.HasPrefix(s, `<mdfeProc versao="3.00" xmlns="http://www.portalfiscal.inf.br/mdfe"><MDFe `) ||
		!strings.Contains(s, "<protMDFe ") || !strings.HasSuffix(s, "</protMDFe></mdfeProc>") {
		t.Errorf("XMLProc: %s", s)
	}
}

func elementoTexto(t *testing.T, soap []byte, nome string) []byte {
	t.Helper()
	e := primeiro(soap, nome)
	if e == nil {
		t.Fatalf("sem %s em %s", nome, soap)
	}
	i, j := bytes.IndexByte(e, '>'), bytes.LastIndex(e, []byte("</"))
	return e[i+1 : j]
}

func TestSefaz_AutorizarRejeicaoNaoEErroGo(t *testing.T) {
	cl, _ := srv(t, 200, fixture(t, "ret_rejeicao.xml"))
	r, err := cl.Autorizar(context.Background(), mdfeAssinadoTeste())
	if err != nil {
		t.Fatalf("rejeição não deve ser erro Go: %v", err)
	}
	if r.CStat != "611" || r.XMLProc != nil || !strings.Contains(r.XMotivo, "nao encerrado") {
		t.Errorf("%+v", r)
	}
}

func TestSefaz_ErrosDeTransporteEResposta(t *testing.T) {
	cl, _ := srv(t, 500, []byte("boom "+strings.Repeat("x", 2000)))
	_, err := cl.Autorizar(context.Background(), mdfeAssinadoTeste())
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") || len(err.Error()) > 900 {
		t.Errorf("HTTP 500: %v", err)
	}
	for _, lixo := range []string{"", "lixo <<<", "<a><b></a>", `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body></soap:Body></soap:Envelope>`} {
		cl, _ := srv(t, 200, []byte(lixo))
		if _, err := cl.Autorizar(context.Background(), mdfeAssinadoTeste()); err == nil {
			t.Errorf("lixo %q deveria dar erro", lixo)
		}
		if _, err := cl.Consultar(context.Background(), chaveTeste); err == nil {
			t.Errorf("consulta lixo %q deveria dar erro", lixo)
		}
		if _, _, err := cl.StatusServico(context.Background()); err == nil {
			t.Errorf("status lixo %q deveria dar erro", lixo)
		}
		if _, _, _, err := cl.ConsultarNaoEncerrados(context.Background(), "11222333000181"); err == nil {
			t.Errorf("naoenc lixo %q deveria dar erro", lixo)
		}
	}
}

func TestSefaz_Consultar(t *testing.T) {
	cl, v := srv(t, 200, fixture(t, "ret_consulta.xml"))
	r, err := cl.Consultar(context.Background(), chaveTeste)
	if err != nil {
		t.Fatal(err)
	}
	if v.path != "/ws/MDFeConsulta/MDFeConsulta.asmx" {
		t.Errorf("path %s", v.path)
	}
	valSefazXSD(t, "consSitMDFe_v3.00.xsd", primeiro(v.corpo, "consSitMDFe"), false)
	if r.CStat != "100" || r.NProt != "926260000000001" || len(r.Eventos) != 1 || !bytes.HasPrefix(r.ProtXML, []byte("<protMDFe")) {
		t.Errorf("%+v", r)
	}
	if !bytes.Contains(r.Eventos[0], []byte(`xmlns="http://www.portalfiscal.inf.br/mdfe"`)) {
		t.Errorf("evento sem xmlns: %s", r.Eventos[0])
	}
	if _, err := cl.Consultar(context.Background(), "123"); err == nil {
		t.Error("chave curta deveria falhar")
	}
}

func TestSefaz_StatusServico(t *testing.T) {
	cl, v := srv(t, 200, fixture(t, "ret_status.xml"))
	c, m, err := cl.StatusServico(context.Background())
	if err != nil || c != "107" || m == "" {
		t.Fatalf("%s %s %v", c, m, err)
	}
	valSefazXSD(t, "consStatServMDFe_v3.00.xsd", primeiro(v.corpo, "consStatServMDFe"), false)
}

func TestSefaz_ConsultarNaoEncerrados(t *testing.T) {
	cl, v := srv(t, 200, fixture(t, "ret_naoenc.xml"))
	c, _, it, err := cl.ConsultarNaoEncerrados(context.Background(), "11.222.333/0001-81")
	if err != nil || c != "111" || len(it) != 2 || it[0].ChMDFe != chaveTeste || it[1].NProt != "926260000000003" {
		t.Fatalf("%s %+v %v", c, it, err)
	}
	valSefazXSD(t, "consMDFeNaoEnc_v3.00.xsd", primeiro(v.corpo, "consMDFeNaoEnc"), false)
	if !bytes.Contains(v.corpo, []byte("CONSULTAR NÃO ENCERRADOS")) {
		t.Error("xServ")
	}
	// CPF
	if _, _, _, err := cl.ConsultarNaoEncerrados(context.Background(), "123.456.789-09"); err != nil {
		t.Fatal(err)
	}
	valSefazXSD(t, "consMDFeNaoEnc_v3.00.xsd", primeiro(v.corpo, "consMDFeNaoEnc"), false)
	if !bytes.Contains(v.corpo, []byte("<CPF>12345678909</CPF>")) {
		t.Error("CPF")
	}
	if _, _, _, err := cl.ConsultarNaoEncerrados(context.Background(), "123"); err == nil {
		t.Error("doc inválido")
	}
}
