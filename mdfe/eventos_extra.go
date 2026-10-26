package mdfe

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

// DocInclusao é um documento (NF-e) do evento Inclusão de DF-e (110115).
type DocInclusao struct{ CMunDescarga, XMunDescarga, ChNFe string }

// IncluirDFe registra a inclusão de DF-e (110115, evIncDFeMDFe). nSeq 1..999.
func (cl *Cliente) IncluirDFe(ctx context.Context, c *cert.Certificado, chave, nProt, cMunCarrega, xMunCarrega string, docs []DocInclusao, nSeq int) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) || !soDigitos(cMunCarrega, 7) || !tamanho(xMunCarrega, 2, 60) || len(docs) == 0 {
		return nil, fmt.Errorf("mdfe: IncluirDFe exige nProt(15), cMunCarrega(7), xMunCarrega(2-60) e ao menos 1 documento")
	}
	var b strings.Builder
	b.WriteString(`<evIncDFeMDFe><descEvento>Inclusao DF-e</descEvento><nProt>` + nProt + `</nProt><cMunCarrega>` + cMunCarrega +
		`</cMunCarrega><xMunCarrega>` + escapar(xMunCarrega) + `</xMunCarrega>`)
	for _, d := range docs {
		if !soDigitos(d.CMunDescarga, 7) || !tamanho(d.XMunDescarga, 2, 60) || !reChave.MatchString(d.ChNFe) {
			return nil, fmt.Errorf("mdfe: IncluirDFe: documento inválido %+v", d)
		}
		b.WriteString(`<infDoc><cMunDescarga>` + d.CMunDescarga + `</cMunDescarga><xMunDescarga>` + escapar(d.XMunDescarga) +
			`</xMunDescarga><chNFe>` + d.ChNFe + `</chNFe></infDoc>`)
	}
	b.WriteString(`</evIncDFeMDFe>`)
	return cl.enviarEvento(ctx, c, chave, "110115", nSeq, b.String())
}

// ConfirmaServico registra a confirmação do serviço de transporte (110117, evConfirmaServMDFe).
func (cl *Cliente) ConfirmaServico(ctx context.Context, c *cert.Certificado, chave, nProt string, nSeq int) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) {
		return nil, fmt.Errorf("mdfe: nProt deve ter 15 dígitos")
	}
	return cl.enviarEvento(ctx, c, chave, "110117", nSeq,
		`<evConfirmaServMDFe><descEvento>Confirmacao Servico Transporte</descEvento><nProt>`+nProt+`</nProt></evConfirmaServMDFe>`)
}

// ── Pagamento (110116 e 110118) ──────────────────────────────────────────────

func pagamentos(ps []Pagamento) (string, error) {
	if len(ps) == 0 {
		return "", fmt.Errorf("mdfe: informe ao menos 1 infPag")
	}
	var g gen
	for _, p := range ps {
		if p.IndAltoDesemp { // indAltoDesemp só existe no infPag do modal rodoviário
			return "", fmt.Errorf("mdfe: IndAltoDesemp não existe no infPag dos eventos")
		}
		g.infPag(p)
	}
	if g.err != nil {
		return "", g.err
	}
	return g.b.String(), nil
}

// PagamentoOperacao registra o pagamento da operação de transporte (110116, evPagtoOperMDFe).
func (cl *Cliente) PagamentoOperacao(ctx context.Context, c *cert.Certificado, chave, nProt string, qtdViagens, nroViagem int, pags []Pagamento, nSeq int) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) || qtdViagens < 1 || qtdViagens > 99999 || nroViagem < 1 || nroViagem > 99999 {
		return nil, fmt.Errorf("mdfe: PagamentoOperacao: nProt(15), qtdViagens e nroViagem (1-99999) obrigatórios")
	}
	p, err := pagamentos(pags)
	if err != nil {
		return nil, err
	}
	det := fmt.Sprintf(`<evPagtoOperMDFe><descEvento>Pagamento Operacao MDF-e</descEvento><nProt>%s</nProt><infViagens><qtdViagens>%05d</qtdViagens><nroViagem>%05d</nroViagem></infViagens>%s</evPagtoOperMDFe>`,
		nProt, qtdViagens, nroViagem, p)
	return cl.enviarEvento(ctx, c, chave, "110116", nSeq, det)
}

// AlteracaoPagamento registra a alteração do pagamento do serviço (110118, evAlteracaoPagtoServMDFe).
func (cl *Cliente) AlteracaoPagamento(ctx context.Context, c *cert.Certificado, chave, nProt string, pags []Pagamento, nSeq int) (*RetornoEvento, error) {
	if !soDigitos(nProt, 15) {
		return nil, fmt.Errorf("mdfe: nProt deve ter 15 dígitos")
	}
	p, err := pagamentos(pags)
	if err != nil {
		return nil, err
	}
	det := `<evAlteracaoPagtoServMDFe><descEvento>Alteracao Pagamento Servico MDFe</descEvento><nProt>` + nProt + `</nProt>` + p + `</evAlteracaoPagtoServMDFe>`
	return cl.enviarEvento(ctx, c, chave, "110118", nSeq, det)
}

func dec(v float64) string { return fmt.Sprintf("%.2f", v) }

func tamanho(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}
