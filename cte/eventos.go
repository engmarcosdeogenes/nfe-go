package cte

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// RetornoEvento é o resultado de CTeRecepcaoEventoV4. XMLProcEvento (procEventoCTe)
// só vem quando cStat 135/136.
type RetornoEvento struct {
	CStat, XMotivo, NProt, DhRegEvento, TpEvento string
	XMLProcEvento                                []byte
}

// Correcao é um item de infCorrecao da CC-e (limites do evCCeCTe: 20/20/500 chars).
type Correcao struct {
	Grupo, Campo, Valor string
	NroItem             string // opcional, "1".."99"
}

// textoCondUso é o xCondUso fixo do evCCeCTe_v4.00.xsd (variante sem acentos, aceita pelo XSD).
const textoCondUso = "A Carta de Correcao e disciplinada pelo Art. 58-B do CONVENIO/SINIEF 06/89: Fica permitida a utilizacao de carta de correcao, para regularizacao de erro ocorrido na emissao de documentos fiscais relativos a prestacao de servico de transporte, desde que o erro nao esteja relacionado com: I - as variaveis que determinam o valor do imposto tais como: base de calculo, aliquota, diferenca de preco, quantidade, valor da prestacao;II - a correcao de dados cadastrais que implique mudanca do emitente, tomador, remetente ou do destinatario;III - a data de emissao ou de saida."

func validarNProt(nProt string) error {
	if !soDigitos(nProt, 15) {
		return fmt.Errorf("cte: nProt deve ter 15 dígitos")
	}
	return nil
}

// Cancelar registra o cancelamento (110111); xJust 15 a 255 caracteres.
func (cl *Cliente) Cancelar(ctx context.Context, c *cert.Certificado, chave, nProt, xJust string) (*RetornoEvento, error) {
	if err := validarNProt(nProt); err != nil {
		return nil, err
	}
	xJust = strings.TrimSpace(xJust)
	if n := utf8.RuneCountInString(xJust); n < 15 || n > 255 {
		return nil, fmt.Errorf("cte: xJust deve ter 15 a 255 caracteres (tem %d)", n)
	}
	det := `<evCancCTe><descEvento>Cancelamento</descEvento><nProt>` + nProt + `</nProt><xJust>` + sfEscapar(xJust) + `</xJust></evCancCTe>`
	return cl.enviarEvento(ctx, c, chave, "110111", 1, det, false)
}

// CartaCorrecao registra a CC-e (110110); nSeq 1..20.
func (cl *Cliente) CartaCorrecao(ctx context.Context, c *cert.Certificado, chave string, correcoes []Correcao, nSeq int) (*RetornoEvento, error) {
	if len(correcoes) == 0 {
		return nil, fmt.Errorf("cte: CC-e exige ao menos uma correção")
	}
	if nSeq < 1 || nSeq > 20 {
		return nil, fmt.Errorf("cte: nSeq da CC-e deve estar entre 1 e 20")
	}
	var b strings.Builder
	b.WriteString(`<evCCeCTe><descEvento>Carta de Correcao</descEvento>`)
	for i, k := range correcoes {
		if n := utf8.RuneCountInString(k.Grupo); n < 1 || n > 20 {
			return nil, fmt.Errorf("cte: correção %d: grupoAlterado deve ter 1 a 20 caracteres", i+1)
		}
		if n := utf8.RuneCountInString(k.Campo); n < 1 || n > 20 {
			return nil, fmt.Errorf("cte: correção %d: campoAlterado deve ter 1 a 20 caracteres", i+1)
		}
		if n := utf8.RuneCountInString(k.Valor); n < 1 || n > 500 {
			return nil, fmt.Errorf("cte: correção %d: valorAlterado deve ter 1 a 500 caracteres", i+1)
		}
		b.WriteString(`<infCorrecao><grupoAlterado>` + sfEscapar(k.Grupo) + `</grupoAlterado><campoAlterado>` + sfEscapar(k.Campo) +
			`</campoAlterado><valorAlterado>` + sfEscapar(k.Valor) + `</valorAlterado>`)
		if k.NroItem != "" {
			if len(k.NroItem) > 2 || digitos(k.NroItem) != k.NroItem || strings.Trim(k.NroItem, "0") == "" {
				return nil, fmt.Errorf("cte: correção %d: nroItemAlterado deve ser 1 a 99", i+1)
			}
			b.WriteString(`<nroItemAlterado>` + k.NroItem + `</nroItemAlterado>`)
		}
		b.WriteString(`</infCorrecao>`)
	}
	b.WriteString(`<xCondUso>` + textoCondUso + `</xCondUso></evCCeCTe>`)
	return cl.enviarEvento(ctx, c, chave, "110110", nSeq, b.String(), false)
}

// EPEC são os dados do evEPECCTe (somente CT-e normal, tpCTe 0).
type EPEC struct {
	XJust                     string // 15..255
	VICMS, VICMSST, VTPrest   float64
	VCarga                    float64
	Toma                      string // 0 rem, 1 exp, 2 rec, 3 dest, 4 outro
	TomaUF                    string
	TomaCNPJ, TomaCPF, TomaIE string // CNPJ ou CPF (um só)
	Modal                     string // "01".."05"
	UFIni, UFFim              string
	DhEmi                     time.Time
}

// EPEC registra a EPEC (110113). ATENÇÃO: a NT CT-e 2026.004 v1.00 (set/2026) extingue
// EPEC/FS-DA com emissão offline; produção em 16/11/2026 e 14/12/2026 (homologação desde
// 13/10/2026). Mantido para o período de transição; data exata por UF não confirmada.
func (cl *Cliente) EPEC(ctx context.Context, c *cert.Certificado, chave string, d EPEC) (*RetornoEvento, error) {
	d.XJust = strings.TrimSpace(d.XJust)
	if n := utf8.RuneCountInString(d.XJust); n < 15 || n > 255 {
		return nil, fmt.Errorf("cte: xJust deve ter 15 a 255 caracteres (tem %d)", n)
	}
	if len(d.Toma) != 1 || d.Toma[0] < '0' || d.Toma[0] > '4' {
		return nil, fmt.Errorf("cte: toma deve ser 0 a 4")
	}
	var doc string
	switch {
	case d.TomaCNPJ != "" && d.TomaCPF == "" && soDigitos(d.TomaCNPJ, 14):
		doc = "<CNPJ>" + d.TomaCNPJ + "</CNPJ>"
	case d.TomaCPF != "" && d.TomaCNPJ == "" && soDigitos(d.TomaCPF, 11):
		doc = "<CPF>" + d.TomaCPF + "</CPF>"
	default:
		return nil, fmt.Errorf("cte: informe só o CNPJ (14) ou só o CPF (11) do tomador")
	}
	if builder.EstadoCodigo[d.TomaUF] == "" || builder.EstadoCodigo[d.UFIni] == "" || builder.EstadoCodigo[d.UFFim] == "" {
		return nil, fmt.Errorf("cte: UF do tomador, UFIni e UFFim devem ser siglas válidas")
	}
	if len(d.Modal) != 2 || d.Modal < "01" || d.Modal > "06" {
		return nil, fmt.Errorf("cte: modal deve ser 01 a 06")
	}
	if d.DhEmi.IsZero() {
		return nil, fmt.Errorf("cte: DhEmi obrigatório")
	}
	det := `<evEPECCTe><descEvento>EPEC</descEvento><xJust>` + sfEscapar(d.XJust) + `</xJust><vICMS>` + dec2(d.VICMS) + `</vICMS>`
	if d.VICMSST > 0 {
		det += `<vICMSST>` + dec2(d.VICMSST) + `</vICMSST>`
	}
	det += `<vTPrest>` + dec2(d.VTPrest) + `</vTPrest><vCarga>` + dec2(d.VCarga) + `</vCarga><toma4><toma>` + d.Toma +
		`</toma><UF>` + d.TomaUF + `</UF>` + doc
	if d.TomaIE != "" {
		det += `<IE>` + sfEscapar(d.TomaIE) + `</IE>`
	}
	det += `</toma4><modal>` + d.Modal + `</modal><UFIni>` + d.UFIni + `</UFIni><UFFim>` + d.UFFim + `</UFFim><tpCTe>0</tpCTe><dhEmi>` +
		d.DhEmi.Format("2006-01-02T15:04:05-07:00") + `</dhEmi></evEPECCTe>`
	return cl.enviarEvento(ctx, c, chave, "110113", 1, det, false)
}

func dec2(v float64) string { return fmt.Sprintf("%.2f", v) }

// PrestacaoDesacordo registra a prestação de serviço em desacordo (610110). O autor é o
// tomador: CNPJ do certificado (não o da chave). xObs 15 a 255 caracteres.
func (cl *Cliente) PrestacaoDesacordo(ctx context.Context, c *cert.Certificado, chave, obs string) (*RetornoEvento, error) {
	obs = strings.TrimSpace(obs)
	if n := utf8.RuneCountInString(obs); n < 15 || n > 255 {
		return nil, fmt.Errorf("cte: xObs deve ter 15 a 255 caracteres (tem %d)", n)
	}
	det := `<evPrestDesacordo><descEvento>Prestacao do Servico em Desacordo</descEvento><indDesacordoOper>1</indDesacordoOper><xObs>` + sfEscapar(obs) + `</xObs></evPrestDesacordo>`
	return cl.enviarEvento(ctx, c, chave, "610110", 1, det, true)
}

// CancelarPrestacaoDesacordo cancela o desacordo (610111); nProtEvento é o protocolo do 610110.
func (cl *Cliente) CancelarPrestacaoDesacordo(ctx context.Context, c *cert.Certificado, chave, nProtEvento string) (*RetornoEvento, error) {
	if err := validarNProt(nProtEvento); err != nil {
		return nil, err
	}
	det := `<evCancPrestDesacordo><descEvento>Cancelamento Prestacao do Servico em Desacordo</descEvento><nProtEvPrestDes>` + nProtEvento + `</nProtEvPrestDes></evCancPrestDesacordo>`
	return cl.enviarEvento(ctx, c, chave, "610111", 1, det, true)
}

// montarEvento gera o eventoCTe assinado (eventoCTe_v4.00.xsd). Id = "ID"+tpEvento+chave+nSeq(3).
// cOrgao = cUF da chave, inclusive via SVC (cOrgao do SVC no MOC: não confirmado).
func (cl *Cliente) montarEvento(c *cert.Certificado, chave, tpEvento string, nSeq int, det string, autorCert bool) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("cte: certificado nulo")
	}
	if !reChave.MatchString(chave) {
		return nil, fmt.Errorf("cte: chave deve ter 44 dígitos")
	}
	if nSeq < 1 || nSeq > 999 {
		return nil, fmt.Errorf("cte: nSeqEvento deve estar entre 1 e 999")
	}
	autor := "<CNPJ>" + chave[6:20] + "</CNPJ>"
	if cl.autorCPF != "" {
		if !soDigitos(cl.autorCPF, 11) {
			return nil, fmt.Errorf("cte: UsarAutorCPF exige CPF de 11 dígitos")
		}
		autor = "<CPF>" + cl.autorCPF + "</CPF>"
	}
	if autorCert { // eventos do tomador: autor é quem assina
		if len(c.CNPJ()) != 14 {
			return nil, fmt.Errorf("cte: certificado sem CNPJ para autor do evento")
		}
		autor = "<CNPJ>" + c.CNPJ() + "</CNPJ>"
	}
	dh := cl.agora().In(builder.FusoUF(sfUFdaChave(chave))).Format("2006-01-02T15:04:05-07:00")
	x := fmt.Sprintf(`<eventoCTe versao="4.00" xmlns="%s"><infEvento Id="ID%s%s%03d"><cOrgao>%s</cOrgao><tpAmb>%s</tpAmb>%s`+
		`<chCTe>%s</chCTe><dhEvento>%s</dhEvento><tpEvento>%s</tpEvento><nSeqEvento>%d</nSeqEvento>`+
		`<detEvento versaoEvento="4.00">%s</detEvento></infEvento></eventoCTe>`,
		nsCTe, tpEvento, chave, nSeq, chave[:2], cl.tpAmb, autor, chave, dh, tpEvento, nSeq, det)
	return sign.AssinarEventoCTe([]byte(x), c)
}

func (cl *Cliente) enviarEvento(ctx context.Context, c *cert.Certificado, chave, tpEvento string, nSeq int, det string, autorCert bool) (*RetornoEvento, error) {
	ev, err := cl.montarEvento(c, chave, tpEvento, nSeq, det, autorCert)
	if err != nil {
		return nil, err
	}
	inner, err := cl.chamar(ctx, svcRecepcaoEvento, string(ev))
	if err != nil {
		return nil, err
	}
	ret, err := sfExigir(inner, "retEventoCTe")
	if err != nil {
		return nil, err
	}
	inf := sfPrimeiro(ret, "infEvento")
	r := &RetornoEvento{
		CStat: sfTexto(inf, "cStat"), XMotivo: sfTexto(inf, "xMotivo"), NProt: sfTexto(inf, "nProt"),
		DhRegEvento: sfTexto(inf, "dhRegEvento"), TpEvento: sfTexto(inf, "tpEvento"),
	}
	if r.CStat == "" {
		return nil, fmt.Errorf("cte: retEventoCTe sem cStat: %s", sfTruncar(ret, 256))
	}
	if r.CStat == "135" || r.CStat == "136" {
		r.XMLProcEvento = []byte(`<procEventoCTe versao="4.00" xmlns="` + nsCTe + `">` + string(ev) + string(ret) + `</procEventoCTe>`)
	}
	return r, nil
}
