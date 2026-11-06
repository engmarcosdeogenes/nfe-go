package cte

import (
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

const (
	nsCTe  = "http://www.portalfiscal.inf.br/cte"
	layout = "2006-01-02T15:04:05-07:00"
	baseQR = "https://dfe-portal.svrs.rs.gov.br/cte/qrCode"
)

var (
	reChave  = regexp.MustCompile(`^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$`) // TChDFe
	reCFOP   = regexp.MustCompile(`^[123567][0-9]([0-9][1-9]|[1-9][0-9])$`)
	reEmail  = regexp.MustCompile(`^[^@]+@[^.]+\..+$`)
	reFoneT  = regexp.MustCompile(`^[0-9]{6,14}$`)
	reFoneRT = regexp.MustCompile(`^[0-9]{7,12}$`)
	reCNPJ   = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)
	reHora   = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$`)
	reRNTRC  = regexp.MustCompile(`^([0-9]{8}|ISENTO)$`)
	reNOcc   = regexp.MustCompile(`^[1-9][0-9]{0,5}$`)
	tipograf = strings.NewReplacer("–", "-", "—", "-", "“", `"`, "”", `"`, "‘", "'", "’", "'", "…", "...", " ", " ")
)

// URLQRCode é o conteúdo de infCTeSupl/qrCodCTe pelo portal SVRS (tpEmis
// diferente de 2). Outras UFs: o autorizador define a base; use
// Entrada.URLQRCode.
func URLQRCode(chave, tpAmb string) string {
	return urlQR(baseQR, chave, tpAmb)
}

// URLQRCodeContingencia é o QR Code da contingência offline (tpEmis=2): o
// da emissão normal mais &sign=<assinatura RSA-SHA1 da chave, base64>,
// calculada pelo chamador.
func URLQRCodeContingencia(chave, tpAmb, assinaturaB64 string) string {
	return URLQRCode(chave, tpAmb) + "&sign=" + assinaturaB64
}

func urlQR(base, chave, tpAmb string) string {
	return strings.TrimRight(base, "?") + "?chCTe=" + chave + "&tpAmb=" + tpAmb
}

// gen acumula o XML e o primeiro erro de validação.
type gen struct {
	b   strings.Builder
	err error

	tribFed *TribFed // infTribFed (só CT-e OS); imp o escreve entre ICMSUFFim e IBSCBS
}

func (g *gen) fail(f string, a ...any) {
	if g.err == nil {
		g.err = fmt.Errorf("cte: "+f, a...)
	}
}
func (g *gen) raw(s string)  { g.b.WriteString(s) }
func (g *gen) open(t string) { g.raw("<" + t + ">") }
func (g *gen) end(t string)  { g.raw("</" + t + ">") }
func (g *gen) el(t, v string) {
	g.raw("<" + t + ">")
	_ = xml.EscapeText(&g.b, []byte(v))
	g.raw("</" + t + ">")
}

// txt normaliza (espaços, aspas tipográficas) e confere tamanho e Latin-1,
// o que TString do XSD ([!-ÿ][ -ÿ]*[!-ÿ]) exige; fora disso a SEFAZ devolve
// 225 sem dizer o campo.
func (g *gen) txt(nome, v string, min, max int) string {
	v = strings.Join(strings.Fields(tipograf.Replace(v)), " ")
	if n := utf8.RuneCountInString(v); n < min || n > max {
		g.fail("%s: tem %d caracteres, precisa de %d a %d", nome, n, min, max)
	}
	for _, r := range v {
		if r > 'ÿ' {
			g.fail("%s: caractere %q fora do Latin-1", nome, r)
			break
		}
	}
	return v
}

func (g *gen) opt(t, nome, v string, min, max int) {
	if strings.TrimSpace(v) != "" {
		g.el(t, g.txt(nome, v, min, max))
	}
}

func digitos(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func soDigitos(s string, n int) bool { return len(s) == n && digitos(s) == s }

// limpa tira pontuação de CNPJ/CPF e põe em maiúscula (CNPJ alfanumérico).
func limpa(s string) string {
	return strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(s))
}

func (g *gen) uf(nome, v string) {
	if _, ok := builder.EstadoCodigo[v]; !ok && v != "EX" {
		g.fail("%s: UF %q inválida", nome, v)
	}
}

func (g *gen) mun(nome, cod, xnome string) (string, string) {
	if !soDigitos(cod, 7) {
		g.fail("%s: código IBGE %q precisa de 7 dígitos", nome, cod)
	}
	return cod, g.txt(nome+" (nome)", xnome, 2, 60)
}

func (g *gen) inteiro(nome, v string, min, max int) string {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < min || n > max {
		g.fail("%s: %q fora de %d-%d", nome, v, min, max)
	}
	return strconv.Itoa(n)
}

// docID valida CNPJ (14) ou CPF (11) e devolve a tag e o valor.
func (g *gen) docID(nome, v string) (tag, val string) {
	v = limpa(v)
	switch {
	case len(v) == 11 && digitos(v) == v:
		return "CPF", v
	case reCNPJ.MatchString(v):
		return "CNPJ", v
	}
	g.fail("%s: %q não é CNPJ (14) nem CPF (11)", nome, v)
	return "CNPJ", v
}

// ie normaliza IE (TIeDest): dígitos ou ISENTO.
func (g *gen) ie(nome, v string, destino bool) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	if v == "ISENTO" {
		if !destino {
			g.fail("%s: ISENTO só vale para rem/exped/receb/dest/toma4", nome)
		}
		return v
	}
	d := digitos(v)
	if len(d) < 2 || len(d) > 14 {
		g.fail("%s: IE precisa de 2 a 14 dígitos", nome)
	}
	return d
}

func (g *gen) cep(nome, v string) string {
	c := builder.FormatarCEP(v)
	if c != "" && len(c) != 8 {
		g.fail("%s: CEP precisa de 8 dígitos", nome)
	}
	return c
}

func (g *gen) fone(nome, v string, re *regexp.Regexp) string {
	f := digitos(v)
	if f != "" && !re.MatchString(f) {
		g.fail("%s: fone inválido", nome)
	}
	return f
}

func (g *gen) email(nome, v string) string {
	m := g.txt(nome, v, 1, 60)
	if !reEmail.MatchString(m) {
		g.fail("%s inválido", nome)
	}
	return m
}

func (g *gen) data(nome, v string) string {
	if _, err := time.Parse("2006-01-02", v); err != nil || !strings.HasPrefix(v, "20") {
		g.fail("%s: data %q inválida (AAAA-MM-DD, ano 20xx)", nome, v)
	}
	return v
}

func (g *gen) chave(nome, v string) string {
	if !reChave.MatchString(v) {
		g.fail("%s: chave %q precisa de 44 caracteres (TChDFe)", nome, v)
	}
	return v
}

// cents arredonda para centavos (comparações monetárias sem ruído de float).
func cents(v float64) int64 { return int64(math.Round(v * 100)) }

// perto: diferença de até tol centavos.
func perto(a, b float64, tol int64) bool {
	d := cents(a) - cents(b)
	return d >= -tol && d <= tol
}

// m formata TDec_1302 (valor >= 0, 13 inteiros, 2 decimais).
func (g *gen) m(nome string, v float64) string {
	if v < 0 || v >= 1e13 || math.IsNaN(v) {
		g.fail("%s: valor %v fora de 0..9999999999999.99", nome, v)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// mpos formata TDec_1302Opc: o XSD não aceita zero.
func (g *gen) mpos(nome string, v float64) string {
	if cents(v) <= 0 {
		g.fail("%s: precisa ser maior que zero (TDec_1302Opc não aceita 0)", nome)
	}
	return g.m(nome, v)
}

// p formata TDec_0302 (percentual 0..999.99).
func (g *gen) p(nome string, v float64) string {
	if v < 0 || v >= 1000 || math.IsNaN(v) {
		g.fail("%s: percentual %v fora de 0..999.99", nome, v)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// ppos formata TDec_0302Opc (pRedBC): >0.
func (g *gen) ppos(nome string, v float64) string {
	if cents(v) <= 0 || v > 100 {
		g.fail("%s: precisa estar entre 0.01 e 100", nome)
	}
	return g.p(nome, v)
}

// prtc formata TDec_0302_04RTC: 2 a 4 decimais.
func (g *gen) prtc(nome string, v float64) string {
	if v < 0 || v >= 1000 || math.IsNaN(v) {
		g.fail("%s: alíquota %v fora de 0..999.9999", nome, v)
	}
	s := strconv.FormatFloat(v, 'f', 4, 64)
	for strings.HasSuffix(s, "0") && len(s)-strings.IndexByte(s, '.') > 3 {
		s = s[:len(s)-1]
	}
	return s
}

func (g *gen) qtd4(nome string, v float64) string {
	if v < 0 || v >= 1e11 {
		g.fail("%s: quantidade %v fora de 0..99999999999.9999", nome, v)
	}
	return strconv.FormatFloat(v, 'f', 4, 64)
}

func inSet(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// endereco escreve TEndereco/TEndeEmi. emit: sem cPais/xPais, com fone.
func (g *gen) endereco(tag, nome string, en Endereco, emit bool) {
	g.open(tag)
	max := 255
	if emit {
		max = 60
	}
	g.el("xLgr", g.txt(nome+" xLgr", en.Logradouro, 2, max))
	g.el("nro", g.txt(nome+" nro", en.Numero, 1, 60))
	g.opt("xCpl", nome+" xCpl", en.Complemento, 1, 60)
	g.el("xBairro", g.txt(nome+" xBairro", en.Bairro, 2, 60))
	c, x := g.mun(nome+" município", en.CodigoMun, en.Municipio)
	g.el("cMun", c)
	g.el("xMun", x)
	if cep := g.cep(nome, en.CEP); cep != "" {
		g.el("CEP", cep)
	}
	g.uf(nome+" UF", en.UF)
	if emit && en.UF == "EX" {
		g.fail("%s: enderEmit não aceita UF EX", nome)
	}
	g.el("UF", en.UF)
	if emit {
		if f := g.fone(nome, en.Fone, reFoneT); f != "" {
			g.el("fone", f)
		}
	} else {
		if en.CodPais != "" {
			if p := digitos(en.CodPais); len(p) < 1 || len(p) > 4 {
				g.fail("%s: cPais 1 a 4 dígitos", nome)
			} else {
				g.el("cPais", p)
			}
		}
		g.opt("xPais", nome+" xPais", en.NomePais, 2, 60)
	}
	g.end(tag)
}

// parte escreve rem/exped/receb/dest/toma4 (mesma sequência no XSD).
func (g *gen) parte(tag, endTag, nome string, cnpj, cpf, ie, xnome, fant, fone, email, isuf string, en Endereco, fantOK bool) {
	g.open(tag)
	g.parteCorpo(endTag, nome, cnpj, cpf, ie, xnome, fant, fone, email, isuf, en, fantOK)
	g.end(tag)
}

func (g *gen) parteCorpo(endTag, nome string, cnpj, cpf, ie, xnome, fant, fone, email, isuf string, en Endereco, fantOK bool) {
	switch {
	case cnpj != "" && cpf != "":
		g.fail("%s: informe CNPJ ou CPF, não os dois", nome)
	case cnpj != "":
		_, v := g.docID(nome, cnpj)
		g.el("CNPJ", v)
	case cpf != "":
		_, v := g.docID(nome, cpf)
		g.el("CPF", v)
	case en.UF == "EX":
		g.raw("<CNPJ></CNPJ>") // TCnpjOpc aceita vazio para o exterior
	default:
		g.fail("%s: CNPJ ou CPF obrigatório (vazio só com UF EX)", nome)
	}
	if v := g.ie(nome+" IE", ie, true); v != "" {
		g.el("IE", v)
	}
	g.el("xNome", g.txt(nome+" xNome", xnome, 2, 60))
	if fant != "" {
		if !fantOK {
			g.fail("%s: xFant não existe neste grupo", nome)
		}
		g.el("xFant", g.txt(nome+" xFant", fant, 2, 60))
	}
	if f := g.fone(nome, fone, reFoneT); f != "" {
		g.el("fone", f)
	}
	if isuf != "" {
		if d := digitos(isuf); len(d) < 8 || len(d) > 9 {
			g.fail("%s: ISUF 8 a 9 dígitos", nome)
		} else {
			g.el("ISUF", d)
		}
	}
	g.endereco(endTag, nome, en, false)
	if email != "" {
		g.el("email", g.email(nome+" email", email))
	}
}

func fmtRat(v float64) string  { return strconv.FormatFloat(v, 'f', 3, 64) }
func fmtPeso(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }
