package mdfe

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

const (
	nsWSDL  = "http://www.portalfiscal.inf.br/mdfe/wsdl/"
	maxResp = 16 << 20 // 16 MiB
)

// reChave: TChNFe do XSD; as posições do CNPJ (7-18) aceitam letras (NT 2025.001).
var reChave = regexp.MustCompile(`^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$`)
var reCNPJ = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)

// servicoMDFe descreve um webservice do autorizador único SVRS (MOC MDF-e 3.00b).
type servicoMDFe struct{ nome, metodo string }

var (
	svcRecepcaoSinc   = servicoMDFe{"MDFeRecepcaoSinc", "mdfeRecepcao"}
	svcConsulta       = servicoMDFe{"MDFeConsulta", "mdfeConsultaMDF"}
	svcRecepcaoEvento = servicoMDFe{"MDFeRecepcaoEvento", "mdfeRecepcaoEvento"}
	svcStatusServico  = servicoMDFe{"MDFeStatusServico", "mdfeStatusServicoMDF"}
	svcConsNaoEnc     = servicoMDFe{"MDFeConsNaoEnc", "mdfeConsNaoEnc"}
)

// soapAction: SOAPAction não confirmada por WSDL (exige mTLS) — fonte MOC 3.00b §4
// + sped-mdfe; confirmar no 1º teste com certificado.
func (s servicoMDFe) soapAction() string { return nsWSDL + s.nome + "/" + s.metodo }

func (s servicoMDFe) url(tpAmb, base string) string {
	if base == "" {
		base = "https://mdfe.svrs.rs.gov.br"
		if tpAmb == "2" {
			base = "https://mdfe-homologacao.svrs.rs.gov.br"
		}
	}
	return strings.TrimRight(base, "/") + "/ws/" + s.nome + "/" + s.nome + ".asmx"
}

// Cliente é o cliente SOAP 1.2 do MDF-e (SVRS, todas as UFs).
type Cliente struct {
	tpAmb    string
	base     string
	http     *http.Client
	agora    func() time.Time
	autorCPF string // se preenchido, o autor dos eventos é este CPF (emitente PF)
}

// UsarAutorCPF faz os eventos saírem com <CPF> no autor (emitente pessoa física).
// Sem chamar, o autor é o CNPJ da chave (posições 7-20). CPF inválido faz o evento falhar.
func (cl *Cliente) UsarAutorCPF(cpf string) { cl.autorCPF = digitos(cpf) }

// NovoCliente cria o cliente com TLS mútuo. tpAmb: "1" produção, "2" homologação.
func NovoCliente(tpAmb string, c *cert.Certificado) (*Cliente, error) {
	if tpAmb != "1" && tpAmb != "2" {
		return nil, fmt.Errorf("mdfe: tpAmb inválido %q (use \"1\" ou \"2\")", tpAmb)
	}
	if c == nil {
		return nil, fmt.Errorf("mdfe: certificado nulo")
	}
	h := &http.Client{Transport: &http.Transport{TLSClientConfig: c.TLSConfig()}, Timeout: 30 * time.Second}
	return &Cliente{tpAmb: tpAmb, http: h, agora: time.Now}, nil
}

// NovoClienteHTTP é para testes: baseURL (ex. httptest) substitui o host e mantém /ws/<S>/<S>.asmx.
func NovoClienteHTTP(tpAmb, baseURL string, h *http.Client) *Cliente {
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	return &Cliente{tpAmb: tpAmb, base: baseURL, http: h, agora: time.Now}
}

// chamar envia <mdfeDadosMsg> (conteúdo já pronto) e devolve o innerXML do SOAP Body.
func (cl *Cliente) chamar(ctx context.Context, s servicoMDFe, conteudo string) ([]byte, error) {
	corpo := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">` +
		`<soap12:Body><mdfeDadosMsg xmlns="` + nsWSDL + s.nome + `">` + conteudo + `</mdfeDadosMsg></soap12:Body></soap12:Envelope>`
	url := s.url(cl.tpAmb, cl.base)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("mdfe: criar request: %w", err)
	}
	act := s.soapAction()
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+act+`"`)
	req.Header.Set("SOAPAction", act)
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mdfe: HTTP %s: %w", s.nome, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResp))
	if err != nil {
		return nil, fmt.Errorf("mdfe: ler resposta: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mdfe: HTTP %d de %s: %s", resp.StatusCode, url, truncar(b, 512))
	}
	var env struct {
		Body struct {
			Inner []byte `xml:",innerxml"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("mdfe: resposta não é SOAP: %w (%s)", err, truncar(b, 256))
	}
	return env.Body.Inner, nil
}

func truncar(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

// elementos devolve o XML bruto de todos os elementos de nome local `nome` (sem
// descer dentro de um já encontrado). Se a abertura não declara xmlns, injeta o do
// MDF-e, pois ele vinha herdado do ancestral.
func elementos(b []byte, nome string) [][]byte {
	var out [][]byte
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		ini := d.InputOffset()
		t, err := d.Token()
		if err != nil {
			return out
		}
		se, ok := t.(xml.StartElement)
		if !ok || se.Name.Local != nome {
			continue
		}
		if d.Skip() != nil {
			return out
		}
		raw := b[ini:d.InputOffset()]
		if i := bytes.IndexByte(raw, '<'); i > 0 { // texto antes da abertura
			raw = raw[i:]
		}
		if fim := bytes.IndexByte(raw, '>'); fim > 0 && !bytes.Contains(raw[:fim], []byte("xmlns=")) {
			n := 1 + len(se.Name.Local)
			raw = append(append(append([]byte{}, raw[:n]...), []byte(` xmlns="`+nsMDFe+`"`)...), raw[n:]...)
		}
		out = append(out, raw)
	}
}

func primeiro(b []byte, nome string) []byte {
	if e := elementos(b, nome); len(e) > 0 {
		return e[0]
	}
	return nil
}

func texto(b []byte, nome string) string {
	e := primeiro(b, nome)
	if e == nil {
		return ""
	}
	var v struct {
		V string `xml:",chardata"`
	}
	if xml.Unmarshal(e, &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.V)
}

func semDeclaracao(x []byte) []byte {
	t := bytes.TrimSpace(x)
	if bytes.HasPrefix(t, []byte("<?xml")) {
		if i := bytes.Index(t, []byte("?>")); i >= 0 {
			return bytes.TrimSpace(t[i+2:])
		}
	}
	return t
}

func exigirRetorno(inner []byte, raiz string) ([]byte, error) {
	r := primeiro(inner, raiz)
	if r == nil {
		return nil, fmt.Errorf("mdfe: resposta sem <%s>: %s", raiz, truncar(inner, 256))
	}
	return r, nil
}

// ── Autorização ──────────────────────────────────────────────────────────────

// RetornoAutorizacao é o resultado de MDFeRecepcaoSinc. XMLProc (mdfeProc =
// MDFe + protMDFe) só vem preenchido quando cStat 100.
type RetornoAutorizacao struct {
	CStat, XMotivo, NProt, DhRecbto, ChMDFe string
	XMLProc                                 []byte
}

// Autorizar envia o MDF-e assinado (GZip+Base64, MOC §3.4.1). Rejeições (cStat 2xx)
// voltam em RetornoAutorizacao sem erro Go; erro Go é só transporte/resposta ilegível.
func (cl *Cliente) Autorizar(ctx context.Context, mdfeAssinado []byte) (*RetornoAutorizacao, error) {
	x := semDeclaracao(mdfeAssinado)
	var zb bytes.Buffer
	zw := gzip.NewWriter(&zb)
	if _, err := zw.Write(x); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	inner, err := cl.chamar(ctx, svcRecepcaoSinc, base64.StdEncoding.EncodeToString(zb.Bytes()))
	if err != nil {
		return nil, err
	}
	ret, err := exigirRetorno(inner, "retMDFe")
	if err != nil {
		return nil, err
	}
	r := &RetornoAutorizacao{CStat: texto(ret, "cStat"), XMotivo: texto(ret, "xMotivo")}
	if r.CStat == "" {
		return nil, fmt.Errorf("mdfe: retMDFe sem cStat: %s", truncar(ret, 256))
	}
	prot := primeiro(ret, "protMDFe")
	if prot != nil {
		inf := primeiro(prot, "infProt")
		r.NProt, r.DhRecbto, r.ChMDFe = texto(inf, "nProt"), texto(inf, "dhRecbto"), texto(inf, "chMDFe")
		if c := texto(inf, "cStat"); c != "" {
			r.CStat, r.XMotivo = c, texto(inf, "xMotivo")
		}
	}
	if r.CStat == "100" && prot != nil {
		r.XMLProc = []byte(`<mdfeProc versao="3.00" xmlns="` + nsMDFe + `">` + string(x) + string(prot) + `</mdfeProc>`)
	}
	return r, nil
}

// ── Consulta ─────────────────────────────────────────────────────────────────

// RetornoConsulta é o resultado de consSitMDFe 3.00.
type RetornoConsulta struct {
	CStat, XMotivo, NProt string
	ProtXML               []byte
	Eventos               [][]byte
}

// Consultar consulta a situação do MDF-e pela chave (44 dígitos).
func (cl *Cliente) Consultar(ctx context.Context, chave string) (*RetornoConsulta, error) {
	if !reChave.MatchString(chave) {
		return nil, fmt.Errorf("mdfe: chave deve ter 44 dígitos")
	}
	inner, err := cl.chamar(ctx, svcConsulta, `<consSitMDFe versao="3.00" xmlns="`+nsMDFe+`"><tpAmb>`+cl.tpAmb+`</tpAmb><xServ>CONSULTAR</xServ><chMDFe>`+chave+`</chMDFe></consSitMDFe>`)
	if err != nil {
		return nil, err
	}
	ret, err := exigirRetorno(inner, "retConsSitMDFe")
	if err != nil {
		return nil, err
	}
	r := &RetornoConsulta{CStat: texto(ret, "cStat"), XMotivo: texto(ret, "xMotivo")}
	if r.CStat == "" {
		return nil, fmt.Errorf("mdfe: retConsSitMDFe sem cStat")
	}
	if p := primeiro(ret, "protMDFe"); p != nil {
		r.ProtXML = p
		r.NProt = texto(p, "nProt")
	}
	r.Eventos = elementos(ret, "procEventoMDFe")
	return r, nil
}

// ── Status e não encerrados ──────────────────────────────────────────────────

// StatusServico consulta o status do serviço.
func (cl *Cliente) StatusServico(ctx context.Context) (cstat, xmotivo string, err error) {
	inner, err := cl.chamar(ctx, svcStatusServico, `<consStatServMDFe versao="3.00" xmlns="`+nsMDFe+`"><tpAmb>`+cl.tpAmb+`</tpAmb><xServ>STATUS</xServ></consStatServMDFe>`)
	if err != nil {
		return "", "", err
	}
	ret, err := exigirRetorno(inner, "retConsStatServMDFe")
	if err != nil {
		return "", "", err
	}
	if cstat = texto(ret, "cStat"); cstat == "" {
		return "", "", fmt.Errorf("mdfe: retConsStatServMDFe sem cStat")
	}
	return cstat, texto(ret, "xMotivo"), nil
}

// NaoEncerrado é um MDF-e autorizado e ainda não encerrado.
type NaoEncerrado struct{ ChMDFe, NProt string }

// ConsultarNaoEncerrados lista MDF-e não encerrados do emitente (CNPJ 14 ou CPF 11).
func (cl *Cliente) ConsultarNaoEncerrados(ctx context.Context, cnpjOuCpf string) (cstat, xmotivo string, itens []NaoEncerrado, err error) {
	doc := apenasAlnum(cnpjOuCpf)
	var tag string
	switch {
	case soDigitos(doc, 11):
		tag = "CPF"
	case len(doc) == 14: // CNPJ pode ser alfanumérico (NT 2025.001)
		tag = "CNPJ"
	default:
		return "", "", nil, fmt.Errorf("mdfe: documento deve ser CPF (11) ou CNPJ (14)")
	}
	inner, err := cl.chamar(ctx, svcConsNaoEnc, `<consMDFeNaoEnc versao="3.00" xmlns="`+nsMDFe+`"><tpAmb>`+cl.tpAmb+`</tpAmb><xServ>CONSULTAR NÃO ENCERRADOS</xServ><`+tag+`>`+doc+`</`+tag+`></consMDFeNaoEnc>`)
	if err != nil {
		return "", "", nil, err
	}
	ret, err := exigirRetorno(inner, "retConsMDFeNaoEnc")
	if err != nil {
		return "", "", nil, err
	}
	if cstat = texto(ret, "cStat"); cstat == "" {
		return "", "", nil, fmt.Errorf("mdfe: retConsMDFeNaoEnc sem cStat")
	}
	for _, it := range elementos(ret, "infMDFe") {
		itens = append(itens, NaoEncerrado{ChMDFe: texto(it, "chMDFe"), NProt: texto(it, "nProt")})
	}
	return cstat, texto(ret, "xMotivo"), itens, nil
}

func soDigitos(s string, n int) bool {
	return len(s) == n && digitos(s) == s
}

func apenasAlnum(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			return r
		case r >= 'a' && r <= 'z':
			return r - 32
		}
		return -1
	}, s)
}
