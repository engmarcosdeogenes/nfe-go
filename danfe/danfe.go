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
	qrcode "github.com/skip2/go-qrcode"
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

// ── NFC-e — Cupom 80mm ────────────────────────────────────────────────────────

const (
	cupomLarg = 80.0                    // largura do papel cupom em mm
	cupomMarg = 3.0                     // margem lateral em mm
	cupomLW   = cupomLarg - 2*cupomMarg // largura útil = 74mm
)

// GerarDANFENFCe gera o cupom fiscal digital 80mm para NFC-e (mod=65).
// O XML de entrada pode ser <NFe> simples ou <nfeProc> com protocolo.
func GerarDANFENFCe(xmlNFeProc []byte) ([]byte, error) {
	dados, err := ParseNFeXML(xmlNFeProc)
	if err != nil {
		return nil, fmt.Errorf("danfe: nfce: %w", err)
	}
	return renderizarCupom(dados)
}

func renderizarCupom(d *DadosDANFE) ([]byte, error) {
	pdf := novoDoc(fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: cupomLarg, Ht: 500.0},
	}))
	pdf.SetMargins(cupomMarg, cupomMarg, cupomMarg)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	y := cupomMarg

	// Watermark SEM VALOR FISCAL (homologação)
	if d.TpAmb == "2" {
		pdf.SetFont("Arial", "B", 14)
		pdf.SetTextColor(210, 210, 210)
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(cupomLW, 6, "SEM VALOR FISCAL", "", 2, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 7
	}

	// Contingência offline NFC-e (tpEmis=9) — frase obrigatória no DANFE
	// enquanto a nota não foi transmitida e autorizada.
	if d.TpEmis == "9" {
		pdf.SetFont("Arial", "B", 8)
		pdf.SetXY(cupomMarg, y)
		pdf.MultiCell(cupomLW, 3.5, "EMITIDA EM CONTINGÊNCIA", "", "C", false)
		pdf.SetX(cupomMarg)
		pdf.SetFont("Arial", "", 6.5)
		pdf.MultiCell(cupomLW, 3, "Pendente de autorização pela SEFAZ", "", "C", false)
		y = pdf.GetY() + 1
	}

	y = cupomCabecalho(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomItens(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomTotais(pdf, d, y)
	y = cupomPagamento(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomQRCode(pdf, d, y)
	cupomChaveAcesso(pdf, d, y)

	if pdf.Err() {
		return nil, fmt.Errorf("danfe: nfce: fpdf: %s", pdf.Error())
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("danfe: nfce: output: %w", err)
	}
	return buf.Bytes(), nil
}

func cupomCabecalho(pdf *Doc, d *DadosDANFE, y float64) float64 {
	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}

	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(cupomMarg, y)
	pdf.MultiCell(cupomLW, 4, nome, "", "C", false)
	y = pdf.GetY()

	pdf.SetFont("Arial", "", 7)
	pdf.SetX(cupomMarg)
	pdf.CellFormat(cupomLW, 3.5, "CNPJ: "+d.EmitCNPJ, "", 2, "C", false, 0, "")

	end := d.EmitEnd
	if end.Logradouro != "" {
		logr := end.Logradouro
		if end.Numero != "" {
			logr += ", " + end.Numero
		}
		pdf.SetX(cupomMarg)
		pdf.CellFormat(cupomLW, 3.5, logr, "", 2, "C", false, 0, "")
	}

	bairroMun := end.Bairro
	if end.Municipio != "" {
		if bairroMun != "" {
			bairroMun += " - "
		}
		bairroMun += end.Municipio
		if end.UF != "" {
			bairroMun += "/" + end.UF
		}
	}
	if bairroMun != "" {
		pdf.SetX(cupomMarg)
		pdf.CellFormat(cupomLW, 3.5, bairroMun, "", 2, "C", false, 0, "")
	}

	if d.NatOp != "" {
		pdf.SetFont("Arial", "", 6)
		pdf.SetX(cupomMarg)
		pdf.CellFormat(cupomLW, 3, d.NatOp, "", 2, "C", false, 0, "")
	}

	return pdf.GetY()
}

func cupomSeparador(pdf *Doc, y float64) float64 {
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.2)
	pdf.Line(cupomMarg, y+1, cupomMarg+cupomLW, y+1)
	return y + 3
}

func cupomItens(pdf *Doc, d *DadosDANFE, y float64) float64 {
	pdf.SetFont("Arial", "B", 7)
	pdf.SetXY(cupomMarg, y)
	pdf.CellFormat(cupomLW, 4, "ITEM  DESCRIÇÃO", "", 2, "L", false, 0, "")
	y = pdf.GetY()

	for _, item := range d.Itens {
		// Linha 1: número e nome do produto
		pdf.SetFont("Arial", "", 7)
		pdf.SetXY(cupomMarg, y)
		pdf.MultiCell(cupomLW, 3.5, fmt.Sprintf("%d. %s", item.Num, item.XProd), "", "L", false)
		y = pdf.GetY()

		// Linha 2: qtd × vUnit = vProd
		linha2 := fmt.Sprintf("   %s x %s = %s",
			formatarQtdCupom(item.Qtd),
			formatarMoeda(item.VUnit),
			formatarMoeda(item.VProd))
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(cupomLW, 3.5, linha2, "", 2, "L", false, 0, "")
		y = pdf.GetY() + 0.5
	}
	return y
}

func cupomTotais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	wL := cupomLW * 0.55
	wR := cupomLW - wL

	// row é uma closure que captura y por referência — cada chamada avança y
	row := func(label, valor string, bold bool) {
		estilo := ""
		if bold {
			estilo = "B"
		}
		pdf.SetFont("Arial", estilo, 8)
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(wL, 4.5, label, "", 0, "L", false, 0, "")
		pdf.CellFormat(wR, 4.5, valor, "", 2, "R", false, 0, "")
		y = pdf.GetY()
	}

	row("Subtotal", formatarMoeda(d.VProd), false)
	if d.VDesc > 0 {
		row("Desconto", "-"+formatarMoeda(d.VDesc), false)
	}
	if d.VFrete > 0 {
		row("Frete", formatarMoeda(d.VFrete), false)
	}
	row("TOTAL", formatarMoeda(d.VNF), true)

	return y
}

func cupomPagamento(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if len(d.Pagamentos) == 0 {
		return y
	}

	pdf.SetFont("Arial", "B", 7)
	pdf.SetXY(cupomMarg, y)
	pdf.CellFormat(cupomLW, 4, "PAGAMENTO", "", 2, "L", false, 0, "")
	y = pdf.GetY()

	wL := cupomLW * 0.60
	wR := cupomLW - wL
	for _, p := range d.Pagamentos {
		pdf.SetFont("Arial", "", 8)
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(wL, 4.5, p.Forma, "", 0, "L", false, 0, "")
		pdf.CellFormat(wR, 4.5, formatarMoeda(p.Valor), "", 2, "R", false, 0, "")
		y = pdf.GetY()
	}
	return y
}

func cupomQRCode(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if d.QrCode == "" {
		return y
	}

	qrPNG, err := qrcode.Encode(d.QrCode, qrcode.Medium, 200)
	if err != nil {
		return y
	}

	const tamQR = 45.0 // mm — centralizado na largura útil de 74mm
	xQR := cupomMarg + (cupomLW-tamQR)/2
	pdf.RegisterImageOptionsReader("nfce_qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))
	pdf.Image("nfce_qr", xQR, y, tamQR, tamQR, false, "", 0, "")

	return y + tamQR + 2
}

func cupomChaveAcesso(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if d.ChaveAcesso == "" {
		return y
	}
	pdf.SetFont("Arial", "", 6)
	pdf.SetXY(cupomMarg, y)
	pdf.MultiCell(cupomLW, 3, formatarChave(d.ChaveAcesso), "", "C", false)
	return pdf.GetY()
}

func formatarQtdCupom(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.3f", v)
}
