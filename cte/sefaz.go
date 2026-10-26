package cte

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
)

const (
	maxResp = 16 << 20 // 16 MiB
)

// Cliente é o cliente SOAP 1.2 do CT-e 4.00 (autorizador escolhido pela UF do emitente).
type Cliente struct {
	uf, tpAmb string
	aut       autorizador
	base      string
	http      *http.Client
	agora     func() time.Time
	autorCPF  string // se preenchido, o autor dos eventos é este CPF (emitente PF)
}

// UsarAutorCPF faz os eventos saírem com <CPF> no autor (emitente pessoa física).
// Sem chamar, o autor é o CNPJ da chave (posições 7-20).
func (cl *Cliente) UsarAutorCPF(cpf string) { cl.autorCPF = digitos(cpf) }

// Autorizador devolve "SVRS", "SVSP", "MG", "MS", "MT", "PR", "SVC-RS" ou "SVC-SP".
func (cl *Cliente) Autorizador() string { return cl.aut.nome }

func novo(uf, tpAmb string, svc bool) (*Cliente, error) {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	if tpAmb != "1" && tpAmb != "2" {
		return nil, fmt.Errorf("cte: tpAmb inválido %q (use \"1\" ou \"2\")", tpAmb)
	}
	a, ok := autorizadorUF[uf]
	if svc {
		a, ok = svcUF(uf)
	}
	if !ok {
		return nil, fmt.Errorf("cte: UF desconhecida %q", uf)
	}
	return &Cliente{uf: uf, tpAmb: tpAmb, aut: a, agora: time.Now}, nil
}

func comTLS(cl *Cliente, c *cert.Certificado) (*Cliente, error) {
	if c == nil {
		return nil, fmt.Errorf("cte: certificado nulo")
	}
	cfg := c.TLSConfig().Clone()
	cfg.Renegotiation = tls.RenegotiateFreelyAsClient // como o DistDFe: alguns autorizadores renegociam
	cl.http = &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}, Timeout: 30 * time.Second}
	return cl, nil
}

// NovoCliente cria o cliente com TLS mútuo. uf é a sigla do emitente; tpAmb "1" produção, "2" homologação.
func NovoCliente(uf, tpAmb string, c *cert.Certificado) (*Cliente, error) {
	cl, err := novo(uf, tpAmb, false)
	if err != nil {
		return nil, err
	}
	return comTLS(cl, c)
}

// NovoClienteSVC usa a contingência SVC da UF (SVC-RS ou SVC-SP; mapeamento não confirmado).
func NovoClienteSVC(uf, tpAmb string, c *cert.Certificado) (*Cliente, error) {
	cl, err := novo(uf, tpAmb, true)
	if err != nil {
		return nil, err
	}
	return comTLS(cl, c)
}

// NovoClienteHTTP é para testes: baseURL (ex. httptest) substitui o host e mantém o caminho do autorizador.
// UF ou tpAmb inválidos caem em SP/"2" (sem erro, é só para teste).
func NovoClienteHTTP(uf, tpAmb, baseURL string, h *http.Client) *Cliente {
	cl, err := novo(uf, tpAmb, false)
	if err != nil {
		cl, _ = novo("SP", "2", false)
	}
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	cl.base, cl.http = baseURL, h
	return cl
}

// chamar envia <cteDadosMsg> (conteúdo já pronto) e devolve o innerXML do SOAP Body.
func (cl *Cliente) chamar(ctx context.Context, s servicoCTe, conteudo string) ([]byte, error) {
	corpo := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">` +
		`<soap12:Body><cteDadosMsg xmlns="` + nsWSDL(s) + `">` + conteudo + `</cteDadosMsg></soap12:Body></soap12:Envelope>`
	url := cl.aut.url(s, cl.tpAmb, cl.base)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("cte: criar request: %w", err)
	}
	act := s.soapAction()
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+act+`"`)
	req.Header.Set("SOAPAction", act)
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cte: HTTP %s: %w", s.nome, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResp))
	if err != nil {
		return nil, fmt.Errorf("cte: ler resposta: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cte: HTTP %d de %s: %s", resp.StatusCode, url, sfTruncar(b, 512))
	}
	var env struct {
		Body struct {
			Inner []byte `xml:",innerxml"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(b, &env); err != nil {
		return nil, fmt.Errorf("cte: resposta não é SOAP: %w (%s)", err, sfTruncar(b, 256))
	}
	return env.Body.Inner, nil
}

func sfTruncar(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

// sfElementos devolve o XML bruto dos elementos de nome local `nome` (sem descer dentro
// de um já achado). Se a abertura não declara xmlns, injeta o do CT-e (vinha do ancestral).
func sfElementos(b []byte, nome string) [][]byte {
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
		if i := bytes.IndexByte(raw, '<'); i > 0 {
			raw = raw[i:]
		}
		if fim := bytes.IndexByte(raw, '>'); fim > 0 && !bytes.Contains(raw[:fim], []byte("xmlns=")) {
			n := 1 + len(se.Name.Local)
			raw = append(append(append([]byte{}, raw[:n]...), []byte(` xmlns="`+nsCTe+`"`)...), raw[n:]...)
		}
		out = append(out, raw)
	}
}

func sfPrimeiro(b []byte, nome string) []byte {
	if e := sfElementos(b, nome); len(e) > 0 {
		return e[0]
	}
	return nil
}

func sfTexto(b []byte, nome string) string {
	e := sfPrimeiro(b, nome)
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

func sfSemDeclaracao(x []byte) []byte {
	t := bytes.TrimSpace(x)
	if bytes.HasPrefix(t, []byte("<?xml")) {
		if i := bytes.Index(t, []byte("?>")); i >= 0 {
			return bytes.TrimSpace(t[i+2:])
		}
	}
	return t
}

func sfExigir(inner []byte, raiz string) ([]byte, error) {
	r := sfPrimeiro(inner, raiz)
	if r == nil {
		return nil, fmt.Errorf("cte: resposta sem <%s>: %s", raiz, sfTruncar(inner, 256))
	}
	return r, nil
}

// sfEscapar escapa só &, < e > (detalhe de evento montado como string); gen.el do builder usa xml.EscapeText (também aspas e controles) -- mantidos separados.
func sfEscapar(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// sfUFdaChave devolve a sigla da UF pelo cUF (chave[:2]); "SP" se desconhecido.
func sfUFdaChave(chave string) string {
	for sigla, cod := range builder.EstadoCodigo {
		if cod == chave[:2] {
			return sigla
		}
	}
	return "SP"
}

// ── Autorização ──────────────────────────────────────────────────────────────

// RetornoAutorizacao é o resultado de CTeRecepcaoSincV4. XMLProc (cteProc 4.00 =
// CTe + protCTe) só vem preenchido quando cStat 100.
type RetornoAutorizacao struct {
	CStat, XMotivo, NProt, DhRecbto, ChCTe string
	XMLProc                                []byte
}

// Autorizar envia o CT-e assinado em GZip+Base64 (MOC CT-e 4.00 §3.4.1). Rejeições
// voltam em RetornoAutorizacao sem erro Go; erro Go é só transporte/resposta ilegível.
func (cl *Cliente) Autorizar(ctx context.Context, cteAssinado []byte) (*RetornoAutorizacao, error) {
	x := sfSemDeclaracao(cteAssinado)
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
	ret, err := sfExigir(inner, "retCTe")
	if err != nil {
		return nil, err
	}
	r := &RetornoAutorizacao{CStat: sfTexto(ret, "cStat"), XMotivo: sfTexto(ret, "xMotivo")}
	prot := sfPrimeiro(ret, "protCTe")
	if prot != nil {
		inf := sfPrimeiro(prot, "infProt")
		r.NProt, r.DhRecbto, r.ChCTe = sfTexto(inf, "nProt"), sfTexto(inf, "dhRecbto"), sfTexto(inf, "chCTe")
		if c := sfTexto(inf, "cStat"); c != "" {
			r.CStat, r.XMotivo = c, sfTexto(inf, "xMotivo")
		}
	}
	if r.CStat == "" {
		return nil, fmt.Errorf("cte: retCTe sem cStat: %s", sfTruncar(ret, 256))
	}
	if r.CStat == "100" && prot != nil {
		r.XMLProc = []byte(`<cteProc versao="4.00" xmlns="` + nsCTe + `">` + string(x) + string(prot) + `</cteProc>`)
	}
	return r, nil
}

// ── Consulta e status ────────────────────────────────────────────────────────

// RetornoConsulta é o resultado de consSitCTe 4.00.
type RetornoConsulta struct {
	CStat, XMotivo, NProt string
	ProtXML               []byte
	Eventos               [][]byte
}

// Consultar consulta a situação do CT-e pela chave (44 dígitos).
func (cl *Cliente) Consultar(ctx context.Context, chave string) (*RetornoConsulta, error) {
	if !reChave.MatchString(chave) {
		return nil, fmt.Errorf("cte: chave deve ter 44 dígitos")
	}
	inner, err := cl.chamar(ctx, svcConsulta, `<consSitCTe versao="4.00" xmlns="`+nsCTe+`"><tpAmb>`+cl.tpAmb+`</tpAmb><xServ>CONSULTAR</xServ><chCTe>`+chave+`</chCTe></consSitCTe>`)
	if err != nil {
		return nil, err
	}
	ret, err := sfExigir(inner, "retConsSitCTe")
	if err != nil {
		return nil, err
	}
	r := &RetornoConsulta{CStat: sfTexto(ret, "cStat"), XMotivo: sfTexto(ret, "xMotivo")}
	if r.CStat == "" {
		return nil, fmt.Errorf("cte: retConsSitCTe sem cStat")
	}
	if p := sfPrimeiro(ret, "protCTe"); p != nil {
		r.ProtXML = p
		r.NProt = sfTexto(p, "nProt")
	}
	r.Eventos = sfElementos(ret, "procEventoCTe")
	return r, nil
}

// StatusServico consulta o status do serviço da UF do cliente (consStatServCTe leva cUF).
func (cl *Cliente) StatusServico(ctx context.Context) (cstat, xmotivo string, err error) {
	cuf := builder.EstadoCodigo[cl.uf]
	inner, err := cl.chamar(ctx, svcStatusServico, `<consStatServCTe versao="4.00" xmlns="`+nsCTe+`"><tpAmb>`+cl.tpAmb+`</tpAmb><cUF>`+cuf+`</cUF><xServ>STATUS</xServ></consStatServCTe>`)
	if err != nil {
		return "", "", err
	}
	ret, err := sfExigir(inner, "retConsStatServCTe")
	if err != nil {
		return "", "", err
	}
	if cstat = sfTexto(ret, "cStat"); cstat == "" {
		return "", "", fmt.Errorf("cte: retConsStatServCTe sem cStat")
	}
	return cstat, sfTexto(ret, "xMotivo"), nil
}
