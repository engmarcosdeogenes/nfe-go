package builder

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// conferirXML é a última rede antes da assinatura: percorre o XML montado e
// recusa o que o schema da SEFAZ derrubaria com cStat 225 ("Falha no Schema
// XML", sem dizer o campo) — elemento de texto vazio, caractere fora do
// Latin-1 e texto maior que o limite do campo. Não substitui o XSD (sem cgo
// não há validador aqui), cobre as classes que já derrubaram nota real:
// cEnq vazio, xLgr/xNome vazios, quebra de linha no infCpl.
func conferirXML(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	type no struct {
		nome   string
		texto  strings.Builder
		filhos bool
	}
	var pilha []*no
	caminho := func() string {
		partes := make([]string, len(pilha))
		for i, n := range pilha {
			partes[i] = n.nome
		}
		return strings.Join(partes, "/")
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("XML montado ilegível: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(pilha) > 0 {
				pilha[len(pilha)-1].filhos = true
			}
			pilha = append(pilha, &no{nome: t.Name.Local})
		case xml.CharData:
			if len(pilha) > 0 {
				pilha[len(pilha)-1].texto.Write(t)
			}
		case xml.EndElement:
			n := pilha[len(pilha)-1]
			if !n.filhos {
				if err := conferirTexto(n.nome, n.texto.String()); err != nil {
					return fmt.Errorf("%s (%s): %w", rotuloCampo(n.nome), caminho(), err)
				}
			}
			pilha = pilha[:len(pilha)-1]
		}
	}
}

func conferirTexto(nome, v string) error {
	if v == "" {
		return fmt.Errorf("está vazio — preencha ou deixe o campo de fora")
	}
	if v != strings.TrimSpace(v) {
		return fmt.Errorf("começa ou termina com espaço")
	}
	for _, r := range v {
		if r < ' ' || r > 'ÿ' {
			return fmt.Errorf("tem o caractere %q, que a SEFAZ não aceita (troque ou apague)", r)
		}
	}
	if max, ok := tamanhoMaximo[nome]; ok && utf8.RuneCountInString(v) > max {
		return fmt.Errorf("tem %d caracteres, o máximo é %d", utf8.RuneCountInString(v), max)
	}
	return nil
}

// tamanhoMaximo dos campos de texto livre (leiaute NF-e 4.00). Os códigos
// (CNPJ, CFOP, NCM…) já saem formatados pelo builder.
var tamanhoMaximo = map[string]int{
	"natOp": 60, "xNome": 60, "xFant": 60, "xLgr": 60, "nro": 60, "xCpl": 60,
	"xBairro": 60, "xMun": 60, "xPais": 60, "email": 60, "cProd": 60,
	"xProd": 120, "uCom": 6, "uTrib": 6, "infAdProd": 500,
	"infCpl": 5000, "infAdFisco": 2000, "xJust": 256,
}

var rotulos = map[string]string{
	"natOp": "Natureza da operação", "xNome": "Nome", "xFant": "Nome fantasia",
	"xLgr": "Logradouro", "nro": "Número do endereço", "xCpl": "Complemento",
	"xBairro": "Bairro", "xMun": "Município", "email": "E-mail",
	"cProd": "Código do produto", "xProd": "Descrição do produto",
	"uCom": "Unidade", "uTrib": "Unidade tributável", "infAdProd": "Informação adicional do item",
	"infCpl": "Informações complementares", "infAdFisco": "Informações ao fisco",
	"cEnq": "Enquadramento do IPI (cEnq)", "cBenef": "Código de benefício fiscal (cBenef)",
	"IE": "Inscrição estadual", "CEP": "CEP", "fone": "Telefone",
}

func rotuloCampo(nome string) string {
	if r, ok := rotulos[nome]; ok {
		return r
	}
	return nome
}

// normalizarTexto é o que aparar aplica em toda string da entrada: espaço,
// tab e quebra de linha viram um espaço só, pontas aparadas, e a pontuação
// "tipográfica" que vem colada do Word/WhatsApp vira a equivalente Latin-1.
// Nenhuma dessas trocas muda o sentido; todas derrubavam a nota.
func normalizarTexto(s string) string {
	return strings.Join(strings.Fields(substituirTipografia.Replace(s)), " ")
}

var substituirTipografia = strings.NewReplacer(
	"–", "-", "—", "-", "−", "-",
	"“", `"`, "”", `"`, "‘", "'", "’", "'",
	"…", "...", " ", " ", "•", "-",
)
