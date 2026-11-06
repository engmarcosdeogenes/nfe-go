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
		if !soDigitos(d.CMunDescarga, 7) || !tamanho(d.XMunDescarga, 2, 60) || !soDigitos(d.ChNFe, 44) {
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

// CompPag é um componente do pagamento (tpComp 01 vale-pedágio, 02 impostos, 03 despesas, 04 frete, 99 outros).
type CompPag struct {
	TpComp string
	VComp  float64
	XComp  string // opcional
}

// PrazoPag é uma parcela (infPrazo).
type PrazoPag struct {
	NParcela int
	DVenc    string // AAAA-MM-DD
	VParcela float64
}

// InfPagamento é o grupo infPag. Informe exatamente um entre CPF, CNPJ, IDEstrangeiro
// e exatamente uma forma de recebimento: PIX, CNPJIPEF ou CodBanco+CodAgencia.
type InfPagamento struct {
	XNome                               string
	CPF, CNPJ, IDEstrangeiro            string
	Comp                                []CompPag
	VContrato                           float64
	IndPag                              string // 0 à vista, 1 a prazo
	VAdiant                             float64
	IndAntecipaAdiant                   bool
	InfPrazo                            []PrazoPag
	TpAntecip                           string // opcional: 0, 1 ou 2
	CodBanco, CodAgencia, CNPJIPEF, PIX string
}

func (p InfPagamento) xml() (string, error) {
	var b strings.Builder
	b.WriteString("<infPag>")
	if p.XNome != "" {
		if !tamanho(p.XNome, 2, 60) {
			return "", fmt.Errorf("mdfe: infPag.xNome 2-60 caracteres")
		}
		b.WriteString("<xNome>" + escapar(p.XNome) + "</xNome>")
	}
	switch {
	case p.CPF != "" && p.CNPJ == "" && p.IDEstrangeiro == "" && soDigitos(p.CPF, 11):
		b.WriteString("<CPF>" + p.CPF + "</CPF>")
	case p.CNPJ != "" && p.CPF == "" && p.IDEstrangeiro == "" && len(p.CNPJ) == 14:
		b.WriteString("<CNPJ>" + p.CNPJ + "</CNPJ>")
	case p.IDEstrangeiro != "" && p.CPF == "" && p.CNPJ == "":
		b.WriteString("<idEstrangeiro>" + escapar(p.IDEstrangeiro) + "</idEstrangeiro>")
	default:
		return "", fmt.Errorf("mdfe: infPag exige exatamente um de CPF(11), CNPJ(14) ou IDEstrangeiro")
	}
	if len(p.Comp) == 0 {
		return "", fmt.Errorf("mdfe: infPag exige ao menos 1 Comp")
	}
	for _, k := range p.Comp {
		b.WriteString("<Comp><tpComp>" + k.TpComp + "</tpComp><vComp>" + dec(k.VComp) + "</vComp>")
		if k.XComp != "" {
			b.WriteString("<xComp>" + escapar(k.XComp) + "</xComp>")
		}
		b.WriteString("</Comp>")
	}
	b.WriteString("<vContrato>" + dec(p.VContrato) + "</vContrato><indPag>" + p.IndPag + "</indPag>")
	if p.VAdiant > 0 {
		b.WriteString("<vAdiant>" + dec(p.VAdiant) + "</vAdiant>")
	}
	if p.IndAntecipaAdiant {
		b.WriteString("<indAntecipaAdiant>1</indAntecipaAdiant>")
	}
	for _, pr := range p.InfPrazo {
		b.WriteString(fmt.Sprintf("<infPrazo><nParcela>%03d</nParcela><dVenc>%s</dVenc><vParcela>%s</vParcela></infPrazo>", pr.NParcela, pr.DVenc, dec(pr.VParcela)))
	}
	if p.TpAntecip != "" {
		b.WriteString("<tpAntecip>" + p.TpAntecip + "</tpAntecip>")
	}
	switch {
	case p.PIX != "":
		b.WriteString("<infBanc><PIX>" + escapar(p.PIX) + "</PIX></infBanc>")
	case p.CNPJIPEF != "":
		b.WriteString("<infBanc><CNPJIPEF>" + p.CNPJIPEF + "</CNPJIPEF></infBanc>")
	case p.CodBanco != "" && p.CodAgencia != "":
		b.WriteString("<infBanc><codBanco>" + p.CodBanco + "</codBanco><codAgencia>" + p.CodAgencia + "</codAgencia></infBanc>")
	default:
		return "", fmt.Errorf("mdfe: infPag exige infBanc (PIX, CNPJIPEF ou CodBanco+CodAgencia)")
	}
	b.WriteString("</infPag>")
	return b.String(), nil
}

func pagamentos(ps []InfPagamento) (string, error) {
	if len(ps) == 0 {
		return "", fmt.Errorf("mdfe: informe ao menos 1 infPag")
	}
	var b strings.Builder
	for _, p := range ps {
		x, err := p.xml()
		if err != nil {
			return "", err
		}
		b.WriteString(x)
	}
	return b.String(), nil
}

// PagamentoOperacao registra o pagamento da operação de transporte (110116, evPagtoOperMDFe).
func (cl *Cliente) PagamentoOperacao(ctx context.Context, c *cert.Certificado, chave, nProt string, qtdViagens, nroViagem int, pags []InfPagamento, nSeq int) (*RetornoEvento, error) {
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
func (cl *Cliente) AlteracaoPagamento(ctx context.Context, c *cert.Certificado, chave, nProt string, pags []InfPagamento, nSeq int) (*RetornoEvento, error) {
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
