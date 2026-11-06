package mdfe

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

// RetornoEvento é o resultado de MDFeRecepcaoEvento. XMLProcEvento (procEventoMDFe)
// só vem quando cStat 135/136.
type RetornoEvento struct {
	CStat, XMotivo, NProt, DhRegEvento, TpEvento string
	XMLProcEvento                                []byte
}

// Encerrar registra o encerramento (110112). nSeqEvento é sempre 1 (rej 636 se repetir).
func (cl *Cliente) Encerrar(ctx context.Context, c *cert.Certificado, chave, nProt string, dtEnc time.Time, cUF, cMun string) (*RetornoEvento, error) {
	return cl.encerrar(ctx, c, chave, nProt, dtEnc, cUF, cMun, false)
}

// EncerrarPorTerceiro encerra um MDF-e de outro emitente (NT 2024.001, indEncPorTerceiro=1).
// O autor do evento passa a ser o CNPJ do certificado (o terceiro). Autor/regras 524/525
// vêm da NT, não de XSD: confirmar em homologação.
func (cl *Cliente) EncerrarPorTerceiro(ctx context.Context, c *cert.Certificado, chave, nProt string, dtEnc time.Time, cUF, cMun string) (*RetornoEvento, error) {
	return cl.encerrar(ctx, c, chave, nProt, dtEnc, cUF, cMun, true)
}

func (cl *Cliente) encerrar(ctx context.Context, c *cert.Certificado, chave, nProt string, dtEnc time.Time, cUF, cMun string, terceiro bool) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) {
		return nil, fmt.Errorf("mdfe: nProt deve ter 15 dígitos")
	}
	if !soDigitos(cUF, 2) || !soDigitos(cMun, 7) {
		return nil, fmt.Errorf("mdfe: cUF (2 dígitos) e cMun (7 dígitos) obrigatórios")
	}
	det := `<evEncMDFe><descEvento>Encerramento</descEvento><nProt>` + nProt + `</nProt><dtEnc>` +
		dtEnc.Format("2006-01-02") + `</dtEnc><cUF>` + cUF + `</cUF><cMun>` + cMun + `</cMun>`
	if terceiro {
		det += `<indEncPorTerceiro>1</indEncPorTerceiro>`
	}
	return cl.enviarEvento(ctx, c, chave, "110112", 1, det+`</evEncMDFe>`, terceiro)
}

// Cancelar registra o cancelamento (110111). Só até 24 h da autorização (MOC 3.00b
// Visão Geral §6.1): depois disso a SEFAZ rejeita (220 prazo; 215 justificativa fora do tamanho).
func (cl *Cliente) Cancelar(ctx context.Context, c *cert.Certificado, chave, nProt, xJust string) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) {
		return nil, fmt.Errorf("mdfe: nProt deve ter 15 dígitos")
	}
	xJust = strings.TrimSpace(xJust)
	if n := utf8.RuneCountInString(xJust); n < 15 || n > 255 {
		return nil, fmt.Errorf("mdfe: xJust deve ter 15 a 255 caracteres (tem %d)", n)
	}
	det := `<evCancMDFe><descEvento>Cancelamento</descEvento><nProt>` + nProt + `</nProt><xJust>` + escapar(xJust) + `</xJust></evCancMDFe>`
	return cl.enviarEvento(ctx, c, chave, "110111", 1, det)
}

// IncluirCondutor registra a inclusão de condutor (110114); nSeq 1..999.
func (cl *Cliente) IncluirCondutor(ctx context.Context, c *cert.Certificado, chave, nome, cpf string, nSeq int) (*RetornoEvento, error) {
	nome = strings.TrimSpace(nome)
	if n := utf8.RuneCountInString(nome); n < 2 || n > 60 {
		return nil, fmt.Errorf("mdfe: nome do condutor deve ter 2 a 60 caracteres")
	}
	if !soDigitos(cpf, 11) {
		return nil, fmt.Errorf("mdfe: cpf deve ter 11 dígitos")
	}
	det := `<evIncCondutorMDFe><descEvento>Inclusao Condutor</descEvento><condutor><xNome>` + escapar(nome) +
		`</xNome><CPF>` + cpf + `</CPF></condutor></evIncCondutorMDFe>`
	return cl.enviarEvento(ctx, c, chave, "110114", nSeq, det)
}

func escapar(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// montarEvento gera o eventoMDFe assinado (estrutura de eventoMDFe_v3.00.xsd).
func (cl *Cliente) montarEvento(c *cert.Certificado, chave, tpEvento string, nSeq int, det string, autorCert ...bool) ([]byte, error) {
	if !reChave.MatchString(chave) {
		return nil, fmt.Errorf("mdfe: chave deve ter 44 dígitos")
	}
	if nSeq < 1 || nSeq > 999 {
		return nil, fmt.Errorf("mdfe: nSeqEvento deve estar entre 1 e 999")
	}
	// Autor = emitente (dígitos 7-20 da chave), não o titular do certificado: filial pode usar cert da matriz.
	autor := "<CNPJ>" + chave[6:20] + "</CNPJ>"
	// Emitente PF: a chave leva 000+CPF nas posições 7-20; o chamador opta por UsarAutorCPF.
	if cl.autorCPF != "" {
		if !soDigitos(cl.autorCPF, 11) {
			return nil, fmt.Errorf("mdfe: UsarAutorCPF exige CPF de 11 dígitos")
		}
		autor = "<CPF>" + cl.autorCPF + "</CPF>"
	}
	if len(autorCert) > 0 && autorCert[0] { // encerramento por terceiro: autor é quem assina, não o emitente
		if len(c.CNPJ()) != 14 {
			return nil, fmt.Errorf("mdfe: certificado sem CNPJ para autor do evento")
		}
		autor = "<CNPJ>" + c.CNPJ() + "</CNPJ>"
	}
	uf := "SP"
	for sigla, cod := range builder.EstadoCodigo {
		if cod == chave[:2] {
			uf = sigla
		}
	}
	dh := cl.agora().In(builder.FusoUF(uf)).Format("2006-01-02T15:04:05-07:00")
	x := fmt.Sprintf(`<eventoMDFe versao="3.00" xmlns="%s"><infEvento Id="ID%s%s%03d"><cOrgao>%s</cOrgao><tpAmb>%s</tpAmb>%s`+
		`<chMDFe>%s</chMDFe><dhEvento>%s</dhEvento><tpEvento>%s</tpEvento><nSeqEvento>%d</nSeqEvento>`+
		`<detEvento versaoEvento="3.00">%s</detEvento></infEvento></eventoMDFe>`,
		nsMDFe, tpEvento, chave, nSeq, chave[:2], cl.tpAmb, autor, chave, dh, tpEvento, nSeq, det)
	return sign.AssinarEventoMDFe([]byte(x), c)
}

func (cl *Cliente) enviarEvento(ctx context.Context, c *cert.Certificado, chave, tpEvento string, nSeq int, det string, autorCert ...bool) (*RetornoEvento, error) {
	ev, err := cl.montarEvento(c, chave, tpEvento, nSeq, det, autorCert...)
	if err != nil {
		return nil, err
	}
	inner, err := cl.chamar(ctx, svcRecepcaoEvento, string(ev))
	if err != nil {
		return nil, err
	}
	ret, err := exigirRetorno(inner, "retEventoMDFe")
	if err != nil {
		return nil, err
	}
	inf := primeiro(ret, "infEvento")
	r := &RetornoEvento{
		CStat: texto(inf, "cStat"), XMotivo: texto(inf, "xMotivo"), NProt: texto(inf, "nProt"),
		DhRegEvento: texto(inf, "dhRegEvento"), TpEvento: texto(inf, "tpEvento"),
	}
	if r.CStat == "" {
		return nil, fmt.Errorf("mdfe: retEventoMDFe sem cStat: %s", truncar(ret, 256))
	}
	if r.CStat == "135" || r.CStat == "136" {
		r.XMLProcEvento = []byte(`<procEventoMDFe versao="3.00" xmlns="` + nsMDFe + `">` + string(ev) + string(ret) + `</procEventoMDFe>`)
	}
	return r, nil
}
