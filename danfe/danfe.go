// Package danfe gera o DANFE (Documento Auxiliar da NF-e) em PDF,
// no formato retrato A4 conforme o Manual de Integração da SEFAZ.
//
// Uso:
//
//	pdfBytes, err := danfe.Gerar(nfeXML)
package danfe

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
)

// Doc envolve *fpdf.Fpdf só pra passar todo texto por tr antes de desenhar.
// A fonte core "Times" do PDF (sem TTF embutida) espera bytes cp1252/WinAnsi,
// não UTF-8 — sem essa tradução, qualquer acento (ç, ã, é...) sai como
// mojibake no PDF final (ex: "Goiás" virava "GoiÃ¡s"), tanto nos rótulos fixos
// em português quanto nos dados dinâmicos (nome, endereço). Os outros métodos
// de *fpdf.Fpdf (Rect, SetFont, SetXY, Image...) não recebem texto do usuário
// e continuam expostos por embedding, sem precisar de wrapper.
type Doc struct {
	*fpdf.Fpdf
	tr func(string) string
}

func novoDoc(f *fpdf.Fpdf) *Doc {
	return &Doc{Fpdf: f, tr: f.UnicodeTranslatorFromDescriptor("")}
}

func (d *Doc) CellFormat(w, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) {
	d.Fpdf.CellFormat(w, h, d.tr(txtStr), borderStr, ln, alignStr, fill, link, linkStr)
}

func (d *Doc) MultiCell(w, h float64, txtStr, borderStr, alignStr string, fill bool) {
	d.Fpdf.MultiCell(w, h, d.tr(txtStr), borderStr, alignStr, fill)
}

// Gerar recebe o XML de uma NF-e (assinada ou nfeProc com protocolo)
// e retorna os bytes do PDF do DANFE. cancelada estampa a marca d'água
// diagonal "CANCELADA" — o XML autorizado não muda com o cancelamento
// (evento à parte), então sem isso o PDF de uma nota cancelada sai
// idêntico ao de uma nota válida.
func Gerar(nfeXML []byte, cancelada bool) ([]byte, error) {
	dados, err := ParseNFeXML(nfeXML)
	if err != nil {
		return nil, fmt.Errorf("danfe: %w", err)
	}
	return renderizar(dados, cancelada, nil, nil)
}

// GerarPrevia desenha o DANFE de uma nota que ainda NÃO foi transmitida, pra
// conferir o documento antes de mandar pra SEFAZ.
//
// Aceita o XML montado sem assinatura e sem protocolo -- é o mesmo caminho do
// EPEC, que também renderiza sem autorização. Sai com tarja de
// pré-visualização: o papel não pode ser confundido com DANFE válido, que só
// existe depois da autorização (e leva protocolo e chave consultável).
func GerarPrevia(nfeXML []byte, logo *Logo) ([]byte, error) {
	dados, err := ParseNFeXML(nfeXML)
	if err != nil {
		return nil, fmt.Errorf("danfe: previa: %w", err)
	}
	// O parser é tolerante de propósito (aceita XML parcial pra não quebrar
	// DANFE de nota antiga). Numa prévia isso sairia como papel em branco,
	// então aqui exige o mínimo que identifica uma NF-e.
	if dados.ChaveAcesso == "" || dados.EmitCNPJ == "" {
		return nil, fmt.Errorf("danfe: previa: XML não parece uma NF-e (sem chave de acesso ou emitente)")
	}
	return renderizarPrevia(dados, logo)
}

// Logo é a marca do emitente estampada no canto superior esquerdo do bloco
// "IDENTIFICAÇÃO DO EMITENTE" — Tipo é o que fpdf.ImageOptions espera
// ("PNG" ou "JPG"), Dados é o arquivo bruto (não recodificado).
type Logo struct {
	Dados []byte
	Tipo  string
}

// GerarComLogo é Gerar, mas estampando a marca do emitente — mesmo layout,
// função separada em vez de mudar a assinatura de Gerar pra não quebrar
// quem já consome a lib sem logo (ex: MetalurgicaBase).
func GerarComLogo(nfeXML []byte, cancelada bool, logo Logo) ([]byte, error) {
	dados, err := ParseNFeXML(nfeXML)
	if err != nil {
		return nil, fmt.Errorf("danfe: %w", err)
	}
	return renderizar(dados, cancelada, nil, &logo)
}

// InfoEPEC carrega os dados que a lei exige estampar no DANFE de uma NF-e
// emitida em contingência EPEC (tpEmis=4) — Ajuste SINIEF 07/05 e NT
// 2014/001 item 03.1a-F: frase fixa + protocolo do evento prévio + motivo +
// hora de entrada em contingência. Sem isso o documento impresso não é
// válido pra acompanhar a mercadoria enquanto a SEFAZ normal está fora do ar.
type InfoEPEC struct {
	Protocolo      string
	Motivo         string
	DhContingencia time.Time
}

// GerarComContingenciaEPEC gera o DANFE de uma NF-e que ainda não foi
// autorizada pela SEFAZ normal (só o EPEC foi registrado no Ambiente
// Nacional) — por isso recebe o XML assinado (não nfeProc: protocolo de
// autorização ainda não existe) e o resultado do registro do EPEC à parte.
func GerarComContingenciaEPEC(nfeXMLAssinado []byte, epec InfoEPEC) ([]byte, error) {
	dados, err := ParseNFeXML(nfeXMLAssinado)
	if err != nil {
		return nil, fmt.Errorf("danfe: %w", err)
	}
	return renderizar(dados, false, &epec, nil)
}

// renderMarcaCancelada estampa "CANCELADA" em vermelho, diagonal, sobre toda
// a página — mesmo padrão visual usado por DANFEs de mercado pra distinguir
// nota cancelada de nota válida à primeira vista.
func renderMarcaCancelada(pdf *Doc) {
	renderMarcaDiagonal(pdf, "CANCELADA", 60)
}

// renderMarcaDiagonal estampa a tarja diagonal do documento. Texto longo
// precisa de fonte menor pra não estourar a diagonal da folha, por isso o
// tamanho vem de fora.
func renderMarcaDiagonal(pdf *Doc, texto string, tamanhoFonte float64) {
	pdf.SetFont("Times", "B", tamanhoFonte)
	pdf.SetTextColor(200, 0, 0)
	pdf.SetAlpha(0.35, "Normal")

	centroX, centroY := larguraPage/2, 148.0
	pdf.TransformBegin()
	pdf.TransformRotate(45, centroX, centroY)
	pdf.SetXY(0, centroY-15)
	pdf.CellFormat(larguraPage, 30, texto, "", 0, "C", false, 0, "")
	pdf.TransformEnd()

	pdf.SetAlpha(1, "Normal")
	pdf.SetTextColor(0, 0, 0)
}

// renderContingenciaEPEC estampa a frase obrigatória (Ajuste SINIEF 07/05 +
// NT 2014/001, item 03.1a-F) mais protocolo/motivo/hora do EPEC — sem isso o
// DANFE impresso em contingência não vale pra acompanhar a mercadoria.
func renderContingenciaEPEC(pdf *Doc, e *InfoEPEC, y float64) float64 {
	altura := 15.0
	pdf.SetDrawColor(200, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Rect(margem, y, larguraUtil, altura, "D")

	pdf.SetFont("Times", "B", 8)
	pdf.SetTextColor(200, 0, 0)
	pdf.SetXY(margem+1, y+1)
	pdf.MultiCell(larguraUtil-2, 3.2,
		"DANFE IMPRESSO EM CONTINGENCIA - EPEC REGULARMENTE RECEBIDA PELA RECEITA FEDERAL DO BRASIL",
		"", "C", false)

	pdf.SetFont("Times", "", 7)
	pdf.SetXY(margem+1, y+8)
	pdf.MultiCell(larguraUtil-2, 3,
		fmt.Sprintf("Protocolo EPEC: %s   Motivo: %s   Entrada em contingencia: %s",
			e.Protocolo, e.Motivo, e.DhContingencia.Format("02/01/2006 15:04:05")),
		"", "L", false)

	pdf.SetDrawColor(0, 0, 0)
	pdf.SetTextColor(0, 0, 0)
	return y + altura + 1
}

// ajustarNaCaixa calcula o tamanho (mm) da imagem dentro de uma caixa
// wMax×hMax preservando proporção (contain, nunca distorce) — decodifica só
// o cabeçalho (image.DecodeConfig), não a imagem inteira. Devolve 0,0 se o
// arquivo não decodificar (logo ruim não pode derrubar a emissão de uma nota
// fiscal real, só sai sem logo).
func ajustarNaCaixa(logo *Logo, wMax, hMax float64) (largura, altura float64) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(logo.Dados))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0
	}
	proporcao := float64(cfg.Width) / float64(cfg.Height)
	largura, altura = wMax, wMax/proporcao
	if altura > hMax {
		altura = hMax
		largura = hMax * proporcao
	}
	return largura, altura
}

// ── Barcode Code 128 ──────────────────────────────────────────────────────────

func gerarBarcodeCode128(dados string) ([]byte, error) {
	bc, err := code128.Encode(dados)
	if err != nil {
		return nil, err
	}
	// Escala para largura razoável
	bcScaled, err := barcode.Scale(bc, 800, 60)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, bcScaled); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Formatação de valores ─────────────────────────────────────────────────────

func formatarMoeda(v float64) string {
	if v == 0 {
		return "0,00"
	}
	s := fmt.Sprintf("%.2f", v)
	// Trocar ponto por vírgula e adicionar separador de milhar
	partes := splitDecimal(s)
	inteiro := inserirPontos(partes[0])
	return inteiro + "," + partes[1]
}

func formatarQtd(v float64) string {
	return fmt.Sprintf("%.4f", v)
}

func formatarAliq(v float64) string {
	if v == 0 {
		return "0,00"
	}
	return fmt.Sprintf("%.2f", v)
}

func splitDecimal(s string) [2]string {
	for i, c := range s {
		if c == '.' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, "00"}
}

func inserirPontos(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	result := make([]byte, 0, n+n/3)
	for i := 0; i < n; i++ {
		if (n-i)%3 == 0 && i != 0 {
			result = append(result, '.')
		}
		result = append(result, s[i]) // s contém apenas dígitos ASCII — acesso por byte é seguro
	}
	return string(result)
}
