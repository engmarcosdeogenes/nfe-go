package sefaz

import (
	"strings"
	"unicode/utf8"
)

// trocasASCII mapeia o que editores e celulares inserem sozinhos (aspas curvas,
// travessão, reticências) para o equivalente ASCII, antes de descartar o que
// fica fora do Latin-1.
var trocasASCII = strings.NewReplacer(
	"“", `"`, "”", `"`, "„", `"`, "″", `"`, "«", `"`, "»", `"`,
	"‘", "'", "’", "'", "‚", "'", "′", "'", "´", "'",
	"‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "―", "-", "−", "-",
	"…", "...", "•", "-",
)

// NormalizarTexto deixa um texto livre (xCorrecao, xJust) dentro do TString
// dos XSD de evento: [!-ÿ]{1}[ -ÿ]*[!-ÿ]{1}. Quebra de linha/tab viram espaço,
// espaços repetidos colapsam, bordas são aparadas, aspas curvas/travessão/
// reticências viram ASCII e o que ainda ficar fora do Latin-1 imprimível
// (emoji, controles) é removido. Idempotente. Não escapa XML.
// shortcut: acento decomposto (e + U+0301, comum em macOS) perde o acento em vez de compor; usar golang.org/x/text/unicode/norm se aparecer.
func NormalizarTexto(s string) string {
	s = trocasASCII.Replace(s)
	b := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
		case r < 0x20 || r == 0xa0 || r == 0x2028 || r == 0x2029:
			b = append(b, ' ')
		case r >= 0x7f && r <= 0x9f, r > 0xff:
		default:
			b = append(b, r)
		}
	}
	return strings.Join(strings.Fields(string(b)), " ")
}

// tamanhoTexto conta caracteres (não bytes): "ção" tem 3 no XSD.
func tamanhoTexto(s string) int { return utf8.RuneCountInString(s) }
