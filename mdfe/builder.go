package mdfe

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

const (
	nsMDFe = "http://www.portalfiscal.inf.br/mdfe"
	layout = "2006-01-02T15:04:05-07:00"
	baseQR = "https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode"
)

var (
	rePlaca  = regexp.MustCompile(`^([A-Z]{2,3}[0-9]{4}|[A-Z]{3,4}[0-9]{3}|[A-Z0-9]{7})$`) // TPlaca
	reRNTRC  = regexp.MustCompile(`^[0-9]{8}$`)                                            // TRNTRC
	reEmail  = regexp.MustCompile(`^[^@]+@[^.]+\..+$`)
	reFone   = regexp.MustCompile(`^[0-9]{7,12}$`)
	tipograf = strings.NewReplacer("–", "-", "—", "-", "“", `"`, "”", `"`, "‘", "'", "’", "'", "…", "...", " ", " ")
)

// URLQRCode é o conteúdo de infMDFeSupl/qrCodMDFe (tpEmis=1). O elemento é
// obrigatório desde a NT 2019.001 (MOC 3.00a); código exato da rejeição por
// QR Code ausente: não confirmado.
func URLQRCode(chave, tpAmb string) string {
	return baseQR + "?chMDFe=" + chave + "&tpAmb=" + tpAmb
}

// URLQRCodeContingencia é o QR Code de contingência offline (tpEmis=2): o da
// emissão normal mais &sign=<assinatura RSA-SHA1 da chave, base64>. O chamador
// calcula a assinatura com o certificado e passa em assinaturaB64.
func URLQRCodeContingencia(chave, tpAmb, assinaturaB64 string) string {
	return URLQRCode(chave, tpAmb) + "&sign=" + assinaturaB64
}

// gen acumula o XML e o primeiro erro de validação.
type gen struct {
	b   strings.Builder
	err error
}

func (g *gen) fail(f string, a ...any) {
	if g.err == nil {
		g.err = fmt.Errorf("mdfe: "+f, a...)
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
// o que o XSD (TString = [!-ÿ][ -ÿ]*[!-ÿ]) exige; fora disso a SEFAZ devolve
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

func (g *gen) uf(nome, v string) {
	if _, ok := builder.EstadoCodigo[v]; !ok {
		g.fail("%s: UF %q inválida", nome, v)
	}
}

func (g *gen) mun(nome, cod, xnome string) (string, string) {
	if !soDigitos(cod, 7) {
		g.fail("%s: código IBGE %q precisa de 7 dígitos", nome, cod)
	}
	return cod, g.txt(nome+" (nome)", xnome, 2, 60)
}

// inteiro valida faixa e devolve sem zeros à esquerda (TNF/TSerie do XSD).
func (g *gen) inteiro(nome, v string, min, max int) string {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < min || n > max {
		g.fail("%s: %q fora de %d-%d", nome, v, min, max)
	}
	return strconv.Itoa(n)
}

// docID valida CNPJ (14) ou CPF (11) e devolve a tag e o valor.
func (g *gen) docID(nome, v string) (tag, val string) {
	v = strings.ToUpper(builder.FormatarIE(v))
	switch len(v) {
	case 11:
		if digitos(v) != v {
			g.fail("%s: CPF %q deve ter só dígitos", nome, v)
		}
		return "CPF", v
	case 14:
		if !reCNPJ.MatchString(v) {
			g.fail("%s: CNPJ %q fora do padrão [A-Z0-9]{12}[0-9]{2}", nome, v)
		}
		return "CNPJ", v
	}
	g.fail("%s: %q não é CNPJ (14) nem CPF (11)", nome, v)
	return "CNPJ", v
}

func (g *gen) numMax(nome string, n, max int) string {
	if n < 0 || n > max {
		g.fail("%s: %d fora de 0-%d", nome, n, max)
	}
	return strconv.Itoa(n)
}

func (g *gen) veiculo(tag string, v Veiculo, tracao bool, cond []Condutor) {
	g.raw("<" + tag + ">")
	g.opt("cInt", tag+" cInt", v.CInt, 1, 10)
	placa := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(v.Placa))
	if !rePlaca.MatchString(placa) { // evita 225 por TPlaca fora do pattern
		g.fail("%s: placa %q inválida", tag, v.Placa)
	}
	g.el("placa", placa)
	g.opt("RENAVAM", tag+" RENAVAM", v.Renavam, 9, 11)
	g.el("tara", g.numMax(tag+" tara", v.TaraKG, 999999))
	// capKG é opcional na tração e obrigatório no reboque (XSD).
	if !tracao || v.CapKG > 0 {
		g.el("capKG", g.numMax(tag+" capKG", v.CapKG, 999999))
	}
	if v.CapM3 > 0 {
		g.el("capM3", g.numMax(tag+" capM3", v.CapM3, 999))
	}
	if p := v.Prop; p != nil {
		g.open("prop")
		t, id := g.docID(tag+" prop", firstNonEmpty(p.CNPJ, p.CPF))
		g.el(t, id)
		if !reRNTRC.MatchString(p.RNTRC) {
			g.fail("%s prop: RNTRC %q precisa de 8 dígitos", tag, p.RNTRC)
		}
		g.el("RNTRC", p.RNTRC)
		g.el("xNome", g.txt(tag+" prop xNome", p.Nome, 2, 60))
		if (p.IE == "") != (p.UF == "") {
			g.fail("%s prop: IE e UF vêm juntas ou nenhuma", tag)
		}
		if p.IE != "" {
			g.el("IE", builder.FormatarIE(p.IE))
			g.uf(tag+" prop UF", p.UF)
			g.el("UF", p.UF)
		}
		if p.TpProp != "0" && p.TpProp != "1" && p.TpProp != "2" {
			g.fail("%s prop: tpProp %q inválido", tag, p.TpProp)
		}
		g.el("tpProp", p.TpProp)
		g.end("prop")
	}
	if tracao {
		if len(cond) < 1 || len(cond) > 10 {
			g.fail("condutores: precisa de 1 a 10, veio %d", len(cond))
		}
		for _, c := range cond {
			g.open("condutor")
			g.el("xNome", g.txt("condutor xNome", c.Nome, 2, 60))
			cpf := builder.FormatarCPF(c.CPF)
			if len(cpf) != 11 {
				g.fail("condutor %q: CPF precisa de 11 dígitos", c.Nome)
			}
			g.el("CPF", cpf)
			g.end("condutor")
		}
		if !inSet(v.TpRod, "01", "02", "03", "04", "05", "06") {
			g.fail("tpRod %q inválido (01..06)", v.TpRod)
		}
		g.el("tpRod", v.TpRod)
	}
	if !inSet(v.TpCar, "00", "01", "02", "03", "04", "05") {
		g.fail("%s: tpCar %q inválido (00..05)", tag, v.TpCar)
	}
	g.el("tpCar", v.TpCar)
	if v.UF != "" {
		g.uf(tag+" UF", v.UF)
		g.el("UF", v.UF)
	}
	g.end(tag)
}

func inSet(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
