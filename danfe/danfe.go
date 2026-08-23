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
// A fonte core "Arial" do PDF (sem TTF embutida) espera bytes cp1252/WinAnsi,
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

// ── Constantes de layout ──────────────────────────────────────────────────────

const (
	margem      = 5.0  // margem lateral em mm
	larguraPage = 210.0
	larguraUtil = larguraPage - 2*margem
)

// ── Renderização ──────────────────────────────────────────────────────────────

func renderizar(d *DadosDANFE, cancelada bool, epec *InfoEPEC, logo *Logo) ([]byte, error) {
	pdf := novoDoc(fpdf.New("P", "mm", "A4", ""))
	pdf.SetMargins(margem, margem, margem)
	pdf.SetAutoPageBreak(true, 5)
	pdf.AddPage()

	// Fonte padrão
	pdf.SetFont("Arial", "", 7)

	// ── Bloco 0: Canhoto de recebimento ───────────────────────────────────────
	y := renderCanhoto(pdf, d)

	// ── Bloco 1: Cabeçalho ────────────────────────────────────────────────────
	y = renderCabecalho(pdf, d, y, logo)

	// ── Bloco 1b: Natureza da operação + protocolo, IE/IM/IEST/CNPJ ──────────
	y = renderNatureza(pdf, d, y)

	// gapSecao é o respiro entre um bloco titulado e o próximo -- sem isso os
	// blocos ficavam com a borda de baixo de um colada na borda de cima do
	// título seguinte, sem separação visual nenhuma (achado real comparando
	// com o modelo de referência).
	const gapSecao = 2.0

	// ── Bloco 3: Destinatário ─────────────────────────────────────────────────
	y = renderDestinatario(pdf, d, y) + gapSecao

	// ── Bloco 4: Itens ────────────────────────────────────────────────────────
	y = renderItens(pdf, d, y) + gapSecao

	// ── Bloco 5: Cálculo do imposto ───────────────────────────────────────────
	y = renderTotais(pdf, d, y) + gapSecao

	// ── Bloco 6: Transporte ───────────────────────────────────────────────────
	y = renderTransporte(pdf, d, y)

	// ── Bloco 7: Duplicatas (cobrança parcelada) ──────────────────────────────
	if len(d.Duplicatas) > 0 {
		y = renderDuplicatas(pdf, d, y+gapSecao)
	}

	// ── Bloco 8: Dados de pagamento ───────────────────────────────────────────
	if len(d.Pagamentos) > 0 {
		y = renderPagamento(pdf, d, y+gapSecao)
	}

	// ── Bloco 9: Dados adicionais ─────────────────────────────────────────────
	y = renderDadosAdicionais(pdf, d, y+gapSecao)

	if epec != nil {
		y = renderContingenciaEPEC(pdf, epec, y)
	}

	renderRodape(pdf, y)

	if cancelada {
		renderMarcaCancelada(pdf)
	}

	if pdf.Err() {
		return nil, fmt.Errorf("danfe: fpdf: %s", pdf.Error())
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("danfe: output: %w", err)
	}
	return buf.Bytes(), nil
}

// ── Canhoto de recebimento ───────────────────────────────────────────────────

// renderCanhoto desenha o "canhoto" — faixa destacável no topo do DANFE onde
// o recebedor assina confirmando o recebimento, com linha tracejada de corte
// abaixo. É seção obrigatória do layout oficial (Manual de Orientação do
// Contribuinte da NF-e), ausente até 17/08/2026 — não aparecia em nenhuma
// DANFE gerada pelo sistema, só nas do concorrente (Alterdata).
func renderCanhoto(pdf *Doc, d *DadosDANFE) float64 {
	y := margem
	lw := larguraUtil
	altCanhoto := 16.0

	wEsq := lw * 0.78
	wDir := lw - wEsq

	setarBorda(pdf)
	pdf.Rect(margem, y, wEsq, altCanhoto, "D")
	pdf.Rect(margem+wEsq, y, wDir, altCanhoto, "D")

	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}
	endDest := d.DestEnd
	endStr := endDest.Logradouro
	if endDest.Numero != "" {
		endStr += ", " + endDest.Numero
	}
	if endDest.Bairro != "" {
		endStr += " - " + endDest.Bairro
	}
	if endDest.Municipio != "" {
		endStr += " - " + endDest.Municipio
	}
	if endDest.UF != "" {
		endStr += "/" + endDest.UF
	}
	recebemos := fmt.Sprintf(
		"RECEBEMOS DE %s OS PRODUTOS/SERVIÇOS CONSTANTES NA NOTA FISCAL ELETRÔNICA INDICADA AO LADO - DESTINATÁRIO: %s - %s - EMISSÃO: %s - VALOR TOTAL: R$ %s",
		nome, d.DestNome, endStr, d.DataEmissao, formatarMoeda(d.VNF),
	)
	pdf.SetFont("Arial", "", 6)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(margem+1, y+0.5)
	pdf.MultiCell(wEsq-2, 3, recebemos, "", "L", false)

	wData := wEsq * 0.3
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(margem+1, y+altCanhoto-4)
	pdf.CellFormat(wData-1, 3, "DATA DE RECEBIMENTO", "", 0, "L", false, 0, "")
	pdf.SetXY(margem+wData+1, y+altCanhoto-4)
	pdf.CellFormat(wEsq-wData-2, 3, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "", 0, "L", false, 0, "")
	setarBorda(pdf)
	pdf.Line(margem+wData, y+altCanhoto-5, margem+wData, y+altCanhoto)

	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(margem+wEsq+1, y+1)
	pdf.CellFormat(wDir-2, 4, "NF-e", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 6)
	pdf.SetX(margem + wEsq + 1)
	pdf.CellFormat(wDir-2, 3.5, "Nº: "+d.NumeroNota, "", 2, "L", false, 0, "")
	pdf.SetX(margem + wEsq + 1)
	pdf.CellFormat(wDir-2, 3.5, "Série: "+d.Serie, "", 2, "L", false, 0, "")

	y += altCanhoto

	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.15)
	pdf.SetDashPattern([]float64{1, 1}, 0)
	pdf.Line(margem, y+1.5, margem+lw, y+1.5)
	pdf.SetDashPattern([]float64{}, 0)

	return y + 3
}

// ── Funções de helpers de desenho ─────────────────────────────────────────────

func setarBorda(pdf *Doc) {
	pdf.SetDrawColor(150, 150, 150)
	pdf.SetLineWidth(0.2)
}

func celulaCampo(pdf *Doc, x, y, w, h float64, label, valor string) {
	setarBorda(pdf)
	pdf.Rect(x, y, w, h, "D")
	pdf.SetXY(x+1, y+0.5)
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(w-2, 3, label, "", 0, "L", false, 0, "")
	pdf.SetXY(x+1, y+4)
	pdf.SetFont("Arial", "B", 7)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(w-2, 4, valor, "", 0, "L", false, 0, "")
}

// ── Cabeçalho ─────────────────────────────────────────────────────────────────

func renderCabecalho(pdf *Doc, d *DadosDANFE, y float64, logo *Logo) float64 {
	lw := larguraUtil

	altCab := 34.0

	// Bloco emitente (esq 38%)
	wEmit := lw * 0.38
	setarBorda(pdf)
	pdf.Rect(margem, y, wEmit, altCab, "D")
	pdf.SetFont("Arial", "", 5.5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(margem+1, y+0.5)
	pdf.CellFormat(wEmit-2, 3, "IDENTIFICAÇÃO DO EMITENTE", "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	// Quando há logo, reserva uma faixa fixa à esquerda (slot, não o tamanho
	// real da imagem — mantém o texto sempre no mesmo lugar independente da
	// proporção do arquivo enviado) e desenha nome/endereço só na largura
	// restante, à direita do slot.
	xTexto := margem + 1
	wTexto := wEmit - 2
	if logo != nil {
		const wSlot, altSlot = 14.0, 12.0
		if largura, altura := ajustarNaCaixa(logo, wSlot, altSlot); largura > 0 {
			imgName := fmt.Sprintf("logo_emit_%p", logo)
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: logo.Tipo}, bytes.NewReader(logo.Dados))
			xImg := margem + 1 + (wSlot-largura)/2
			yImg := y + 5 + (altSlot-altura)/2
			pdf.Image(imgName, xImg, yImg, largura, altura, false, "", 0, "")
			xTexto = margem + 1 + wSlot + 2
			wTexto = wEmit - 2 - wSlot - 2
		}
	}

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(xTexto, y+5)
	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}
	pdf.MultiCell(wTexto, 5, nome, "", "C", false)
	pdf.SetFont("Arial", "", 6)
	end := d.EmitEnd
	pdf.SetX(xTexto)
	pdf.CellFormat(wTexto, 3.5, end.Logradouro+", "+end.Numero, "", 2, "C", false, 0, "")
	pdf.SetX(xTexto)
	pdf.CellFormat(wTexto, 3.5, end.Bairro+" - "+end.CEP, "", 2, "C", false, 0, "")
	pdf.SetX(xTexto)
	pdf.CellFormat(wTexto, 3.5, end.Municipio+" - "+end.UF+"  Fone/Fax: "+end.Fone, "", 2, "C", false, 0, "")

	// Bloco central DANFE (centro 16%)
	wCentro := lw * 0.16
	xCentro := margem + wEmit
	setarBorda(pdf)
	pdf.Rect(xCentro, y, wCentro, altCab, "D")
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(xCentro, y+2)
	pdf.CellFormat(wCentro, 4, "DANFE", "", 2, "C", false, 0, "")
	pdf.SetFont("Arial", "", 5.5)
	pdf.SetX(xCentro)
	// 3 linhas @ 3mm = 9mm, encosta exatamente no início do bloco Saída/Entrada
	// (y+15) -- sem folga daria overlap, como aconteceu quando essa legenda
	// virou 3 linhas (era 2) e continuou com o mesmo y fixo de antes.
	pdf.MultiCell(wCentro, 3, "Documento auxiliar\nda Nota Fiscal\nEletrônica", "", "C", false)
	pdf.SetFont("Arial", "", 6)
	saida, entrada, digito := "0", "0", "1"
	if d.TipoNF == "0" {
		entrada = "1"
	} else {
		saida = "1"
	}
	pdf.SetXY(xCentro+1, y+15)
	pdf.MultiCell(wCentro-9, 3, "Saída: "+saida+"\nEntrada: "+entrada, "", "L", false)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(xCentro+wCentro-7, y+15)
	pdf.CellFormat(6, 8, digito, "1", 0, "C", false, 0, "")
	pdf.SetFont("Arial", "", 6)
	pdf.SetXY(xCentro+1, y+24)
	nfNum := fmt.Sprintf("%09s", d.NumeroNota)
	if len(nfNum) == 9 {
		nfNum = nfNum[0:3] + "." + nfNum[3:6] + "." + nfNum[6:9]
	}
	pdf.MultiCell(wCentro-2, 3, "Nº. "+nfNum+"\nSérie "+fmt.Sprintf("%03s", d.Serie)+"\nFolha 1/1", "", "L", false)

	// Bloco chave de acesso + barcode (dir 46%)
	wDir := lw - wEmit - wCentro
	xDir := xCentro + wCentro
	setarBorda(pdf)
	pdf.Rect(xDir, y, wDir, altCab, "D")

	wBarcode := 70.0
	if d.ChaveAcesso != "" {
		barcodeImg, err := gerarBarcodeCode128(d.ChaveAcesso)
		if err == nil {
			imgName := "barcode_chave"
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(barcodeImg))
			xBarcode := xDir + (wDir-wBarcode)/2
			pdf.Image(imgName, xBarcode, y+1.5, wBarcode, altBarcode, false, "", 0, "")
		}
	}

	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(xDir+1, y+12.5)
	pdf.CellFormat(wDir-2, 3, "CHAVE DE ACESSO", "", 2, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 7)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(xDir + 1)
	pdf.CellFormat(wDir-2, 4, formatarChave(d.ChaveAcesso), "", 2, "C", false, 0, "")
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(xDir + 1)
	pdf.MultiCell(wDir-2, 3, "Consulta de autenticidade no portal nacional da NF-e\nwww.nfe.fazenda.gov.br ou no site da Sefaz Autorizadora", "", "C", false)
	pdf.SetTextColor(0, 0, 0)

	// Homologação watermark — posição fixa no centro vertical da página (não
	// relativa ao y do cabeçalho), pra não precisar reajustar toda vez que um
	// bloco novo (ex: canhoto) mudar a altura do que vem antes.
	if d.TpAmb == "2" {
		pdf.SetFont("Arial", "B", 48)
		pdf.SetTextColor(220, 220, 220)
		pdf.SetXY(margem, 140)
		pdf.CellFormat(lw, 20, "SEM VALOR FISCAL", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	return y + altCab
}

// ── Natureza da operação ──────────────────────────────────────────────────────

func renderNatureza(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	wNat := lw * 0.68
	protocolo := d.NumProtocolo
	if protocolo != "" && d.DataAutorizacao != "" {
		protocolo += " - " + d.DataAutorizacao
	}
	celulaCampo(pdf, margem, y, wNat, 8, "NATUREZA DA OPERAÇÃO", d.NatOp)
	celulaCampo(pdf, margem+wNat, y, lw-wNat, 8, "PROTOCOLO DE AUTORIZAÇÃO DE USO", protocolo)
	y += 8

	// Coluna de Inscrição Municipal só aparece quando a empresa tem uma --
	// modelo real (Alterdata) some com a caixa em vez de mostrar vazia,
	// redistribuindo a largura entre as 3 restantes.
	if d.EmitIM == "" {
		wIE := lw * 0.22
		wIEST := lw * 0.40
		wCNPJ := lw - wIE - wIEST
		celulaCampo(pdf, margem, y, wIE, 8, "INSCRIÇÃO ESTADUAL", d.EmitIE)
		celulaCampo(pdf, margem+wIE, y, wIEST, 8, "INSCRIÇÃO ESTADUAL SUB. TRIBUTÁRIA", d.EmitIEST)
		celulaCampo(pdf, margem+wIE+wIEST, y, wCNPJ, 8, "CNPJ / CPF", d.EmitCNPJ)
		return y + 8
	}
	wIE := lw * 0.22
	wIM := lw * 0.20
	wIEST := lw * 0.30
	wCNPJ := lw - wIE - wIM - wIEST
	celulaCampo(pdf, margem, y, wIE, 8, "INSCRIÇÃO ESTADUAL", d.EmitIE)
	celulaCampo(pdf, margem+wIE, y, wIM, 8, "INSCRIÇÃO MUNICIPAL", d.EmitIM)
	celulaCampo(pdf, margem+wIE+wIM, y, wIEST, 8, "INSCRIÇÃO ESTADUAL SUB. TRIBUTÁRIA", d.EmitIEST)
	celulaCampo(pdf, margem+wIE+wIM+wIEST, y, wCNPJ, 8, "CNPJ / CPF", d.EmitCNPJ)
	return y + 8
}

// altBarcode é a altura de referência (em mm/4, escala usada no cabeçalho)
// da barra do Code128 — mínimo oficial 0,8cm (MOC 7.0 Anexo II - Manual de
// Especificações Técnicas do DANFE e Código de Barras, item 2 "Código de
// Barras"). Achado real: a versão antiga usava 7mm, abaixo do mínimo.
const altBarcode = 10.0

// ── Destinatário ──────────────────────────────────────────────────────────────

func renderDestinatario(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil
	alt := 22.0

	// Rótulo da seção
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DESTINATÁRIO / REMETENTE", "1", 2, "C", true, 0, "")
	y += 4

	// Linha 1: nome, CNPJ/CPF, data emissão
	wNome := lw * 0.55
	wDoc := lw * 0.27
	wData := lw - wNome - wDoc
	doc := d.DestCNPJ
	labelDoc := "CNPJ"
	if doc == "" {
		doc = d.DestCPF
		labelDoc = "CPF"
	}
	celulaCampo(pdf, margem, y, wNome, 9, "NOME / RAZÃO SOCIAL", d.DestNome)
	celulaCampo(pdf, margem+wNome, y, wDoc, 9, labelDoc, doc)
	celulaCampo(pdf, margem+wNome+wDoc, y, wData, 9, "DATA DA EMISSÃO", d.DataEmissao)
	y += 9

	// Linha 2: endereço, bairro, CEP, data de saída/entrada
	end := d.DestEnd
	endStr := end.Logradouro
	if end.Numero != "" {
		endStr += ", " + end.Numero
	}
	if end.Complemento != "" {
		endStr += " - " + end.Complemento
	}
	wEnd := lw * 0.42
	wBairro := lw * 0.28
	wCEP := lw * 0.15
	wDataSai := lw - wEnd - wBairro - wCEP
	celulaCampo(pdf, margem, y, wEnd, 9, "ENDEREÇO", endStr)
	celulaCampo(pdf, margem+wEnd, y, wBairro, 9, "BAIRRO / DISTRITO", end.Bairro)
	celulaCampo(pdf, margem+wEnd+wBairro, y, wCEP, 9, "CEP", end.CEP)
	celulaCampo(pdf, margem+wEnd+wBairro+wCEP, y, wDataSai, 9, "DATA DA SAÍDA/ENTRADA", d.DataSaida)
	y += 9

	// Linha 3: município, UF, fone, indicador IE, IE, hora de saída/entrada
	wMun := lw * 0.28
	wUF := lw * 0.06
	wFone := lw * 0.14
	wIndIE := lw * 0.20
	wIE := lw * 0.16
	wHoraSai := lw - wMun - wUF - wFone - wIndIE - wIE
	celulaCampo(pdf, margem, y, wMun, 9, "MUNICÍPIO", end.Municipio)
	celulaCampo(pdf, margem+wMun, y, wUF, 9, "UF", end.UF)
	celulaCampo(pdf, margem+wMun+wUF, y, wFone, 9, "FONE/FAX", end.Fone)
	celulaCampo(pdf, margem+wMun+wUF+wFone, y, wIndIE, 9, "INDICADOR IE", descricaoIndIEDest(d.DestIndIEDest))
	celulaCampo(pdf, margem+wMun+wUF+wFone+wIndIE, y, wIE, 9, "INSCRIÇÃO ESTADUAL", d.DestIE)
	celulaCampo(pdf, margem+wMun+wUF+wFone+wIndIE+wIE, y, wHoraSai, 9, "HORA DA SAÍDA/ENTRADA", d.HoraSaida)
	y += 9

	_ = alt
	return y
}

// ── Itens ─────────────────────────────────────────────────────────────────────

func renderItens(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	// Cabeçalho da tabela
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DADOS DOS PRODUTOS / SERVIÇOS", "1", 2, "C", true, 0, "")
	y += 4

	// Colunas: código, descrição, NCM, CST/CSOSN, CFOP, un, qtd, v.unit, v.total, BC ICMS, v.ICMS, v.IPI, al.ICMS, al.IPI
	const idxDescricao = 1
	cols := []struct {
		label string
		w     float64
		align string
	}{
		{"CÓD.PROD.", 14, "C"},
		{"DESCRIÇÃO DO PRODUTO / SERVIÇO", 46, "L"},
		{"NCM/SH", 13, "C"},
		{"CST", 10, "C"},
		{"CFOP", 9, "C"},
		{"UN", 7, "C"},
		{"QUANT", 11, "R"},
		{"VL UNIT", 13, "R"},
		{"VL TOTAL", 13, "R"},
		{"BC ICMS", 13, "R"},
		{"VL ICMS", 13, "R"},
		{"VL IPI", 12, "R"},
		{"ALQ ICMS", 13, "R"},
		{"ALQ IPI", 13, "R"},
	}

	// Todas as colunas ocupam a altura cheia do cabeçalho (5mm) -- só as 2
	// últimas (ICMS/IPI) se dividem em 2 sub-linhas por baixo de "ALÍQUOTAS"
	// mesclada, igual ao modelo de mercado usado como referência (a versão
	// anterior desenhava uma linha em branco por cima de TODAS as colunas,
	// não só dessas 2).
	const altCabecalhoItens = 5.0
	pdf.SetFont("Arial", "B", 4.5)
	pdf.SetFillColor(245, 245, 245)
	x := margem
	for i, c := range cols {
		pdf.SetXY(x, y)
		if i == len(cols)-2 {
			pdf.CellFormat(cols[i].w+cols[i+1].w, altCabecalhoItens/2, "ALÍQUOTAS", "1", 0, "C", true, 0, "")
		} else if i < len(cols)-1 {
			pdf.CellFormat(c.w, altCabecalhoItens, c.label, "1", 0, "C", true, 0, "")
		}
		x += c.w
	}
	x = margem + (lw - cols[len(cols)-2].w - cols[len(cols)-1].w)
	pdf.SetXY(x, y+altCabecalhoItens/2)
	pdf.CellFormat(cols[len(cols)-2].w, altCabecalhoItens/2, cols[len(cols)-2].label, "1", 0, "C", true, 0, "")
	pdf.SetXY(x+cols[len(cols)-2].w, y+altCabecalhoItens/2)
	pdf.CellFormat(cols[len(cols)-1].w, altCabecalhoItens/2, cols[len(cols)-1].label, "1", 0, "C", true, 0, "")
	y += altCabecalhoItens

	// Linhas de itens
	pdf.SetFont("Arial", "", 6)
	for _, item := range d.Itens {
		// Verificar se precisa de nova página
		if y > 250 {
			pdf.AddPage()
			y = margem
		}

		x = margem
		vals := []string{
			item.CProd,
			item.XProd,
			item.NCM,
			item.CST,
			item.CFOP,
			item.Unidade,
			formatarQtd(item.Qtd),
			formatarMoeda(item.VUnit),
			formatarMoeda(item.VProd),
			formatarMoeda(item.VBC),
			formatarMoeda(item.ICMS),
			formatarMoeda(item.IPI),
			formatarAliq(item.AliqICMS),
			formatarAliq(item.AliqIPI),
		}

		// Calcular altura necessária para o nome do produto
		altLinha := 5.0
		descW := cols[idxDescricao].w - 2
		pdf.SetFont("Arial", "", 6)
		linhasDesc := pdf.SplitLines([]byte(pdf.tr(item.XProd)), descW)
		if len(linhasDesc) > 1 {
			altLinha = float64(len(linhasDesc)) * 3.5
		}

		for i, c := range cols {
			pdf.SetXY(x, y)
			if i == idxDescricao {
				// MultiCell desenha a própria borda por linha (altura fixa
				// 3.5mm), que não bate com altLinha (altura real da linha,
				// já contando o maior wrap entre as colunas) -- sobrava um
				// vão em branco entre a borda da descrição e a borda da
				// linha. Retângulo à parte, na altura certa, texto sem
				// borda própria por cima.
				setarBorda(pdf)
				pdf.Rect(x, y, c.w, altLinha, "D")
				pdf.SetFont("Arial", "", 6)
				pdf.MultiCell(c.w, 3.5, vals[i], "", "L", false)
				pdf.SetFont("Arial", "", 6)
			} else {
				pdf.CellFormat(c.w, altLinha, vals[i], "1", 0, c.align, false, 0, "")
			}
			x += c.w
		}
		y += altLinha
	}

	return y
}

// ── Totais ────────────────────────────────────────────────────────────────────

func renderTotais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "CÁLCULO DO IMPOSTO", "1", 2, "C", true, 0, "")
	y += 4

	// Grade de 7 colunas, mesmo layout/rótulo do modelo de mercado usado como
	// referência (inclui PIS/COFINS, que não são campo oficial obrigatório do
	// leiaute DANFE mas o mercado padronizou mostrar).
	w := lw / 7
	campos1 := [][2]string{
		{"BASE DE CÁLC. DE ICMS", formatarMoeda(d.VBC)},
		{"VALOR ICMS", formatarMoeda(d.VICMS)},
		{"BASE DE CÁLC. DE ICMS ST", formatarMoeda(d.VBCST)},
		{"VALOR ICMS ST", formatarMoeda(d.VST)},
		{"VALOR PIS", formatarMoeda(d.VPIS)},
		{"VALOR COFINS", formatarMoeda(d.VCOFINS)},
		{"VALOR TOTAL DOS PRODUTOS", formatarMoeda(d.VProd)},
	}
	for i, c := range campos1 {
		celulaCampo(pdf, margem+float64(i)*w, y, w, 9, c[0], c[1])
	}
	y += 9

	campos2 := [][2]string{
		{"VALOR DESCONTO", formatarMoeda(d.VDesc)},
		{"VALOR FRETE", formatarMoeda(d.VFrete)},
		{"VALOR SEGURO", formatarMoeda(d.VSeg)},
		{"VALOR DESP. ACESSÓRIAS", formatarMoeda(d.VOutro)},
		{"VALOR IPI", formatarMoeda(d.VIPI)},
		{"VALOR IMP. IMPORT.", formatarMoeda(d.VII)},
		{"VALOR TOTAL DA NOTA", formatarMoeda(d.VNF)},
	}
	for i, c := range campos2 {
		celulaCampo(pdf, margem+float64(i)*w, y, w, 9, c[0], c[1])
	}
	y += 9

	// Linha extra só quando a nota tem partilha de ICMS interestadual
	// (DIFAL/EC 87 ou FCP) -- o modelo de referência não tem essa linha
	// porque a nota dele não precisava. Continua saindo quando o dado
	// existe, só não ocupa espaço quando não existe.
	if d.VICMSUFRemet != 0 || d.VFCPUFDest != 0 || d.VICMSUFDest != 0 || d.VTotTrib != 0 {
		wDifal := lw / 4
		camposDifal := [][2]string{
			{"V. ICMS UF REMET.", formatarMoeda(d.VICMSUFRemet)},
			{"V. FCP UF DEST.", formatarMoeda(d.VFCPUFDest)},
			{"V. ICMS UF DEST.", formatarMoeda(d.VICMSUFDest)},
			{"V. TOT. TRIB.", formatarMoeda(d.VTotTrib)},
		}
		for i, c := range camposDifal {
			celulaCampo(pdf, margem+float64(i)*wDifal, y, wDifal, 9, c[0], c[1])
		}
		y += 9
	}

	return y
}

// ── Transporte ────────────────────────────────────────────────────────────────

func renderTransporte(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "TRANSPORTADOR / VOLUMES TRANSPORTADOS", "1", 2, "C", true, 0, "")
	y += 4

	// Sem transportador nem volumes de verdade -- colapsa pra 1 linha só
	// (mesmo comportamento do modelo de referência: não desenha 3 linhas de
	// caixa vazia quando não tem o que mostrar).
	temTransportador := d.TranspNome != "" || d.TranspCNPJ != "" || d.TranspEnd != ""
	temVolumes := len(d.Volumes) > 0
	if !temTransportador && !temVolumes {
		celulaCampo(pdf, margem, y, lw, 9, "FRETE POR CONTA", descricaoModFrete(d.ModFrete))
		return y + 9
	}

	// Linha 1: Nome/RS | Frete | Código ANTT | Placa | UF | CNPJ/CPF
	wNome := lw * 0.32
	wFrete := lw * 0.16
	wANTT := lw * 0.14
	wPlaca := lw * 0.12
	wUFVeic := lw * 0.06
	wCNPJ := lw - wNome - wFrete - wANTT - wPlaca - wUFVeic
	celulaCampo(pdf, margem, y, wNome, 9, "NOME / RAZÃO SOCIAL", d.TranspNome)
	celulaCampo(pdf, margem+wNome, y, wFrete, 9, "FRETE", descricaoModFrete(d.ModFrete))
	celulaCampo(pdf, margem+wNome+wFrete, y, wANTT, 9, "CÓDIGO ANTT", d.TranspANTT)
	celulaCampo(pdf, margem+wNome+wFrete+wANTT, y, wPlaca, 9, "PLACA DO VEÍCULO", d.TranspPlaca)
	celulaCampo(pdf, margem+wNome+wFrete+wANTT+wPlaca, y, wUFVeic, 9, "UF", d.TranspPlacaUF)
	celulaCampo(pdf, margem+wNome+wFrete+wANTT+wPlaca+wUFVeic, y, wCNPJ, 9, "CNPJ / CPF", d.TranspCNPJ)
	y += 9

	// Linha 2: Endereço | Município | UF | IE
	wEnd := lw * 0.45
	wMun := lw * 0.30
	wUF := lw * 0.08
	wIE := lw - wEnd - wMun - wUF
	celulaCampo(pdf, margem, y, wEnd, 9, "ENDEREÇO", d.TranspEnd)
	celulaCampo(pdf, margem+wEnd, y, wMun, 9, "MUNICÍPIO", d.TranspMun)
	celulaCampo(pdf, margem+wEnd+wMun, y, wUF, 9, "UF", d.TranspUF)
	celulaCampo(pdf, margem+wEnd+wMun+wUF, y, wIE, 9, "INSCRIÇÃO ESTADUAL", d.TranspIE)
	y += 9

	// Linha 3: Volumes
	wQ := lw / 6
	var qStr, espStr, marcaStr, numStr, pesoB, pesoL string
	if len(d.Volumes) > 0 {
		v := d.Volumes[0]
		qStr = fmt.Sprintf("%g", v.Quantidade)
		espStr = v.Especie
		marcaStr = v.Marca
		numStr = v.Numeracao
		pesoB = fmt.Sprintf("%.3f", v.PesoBruto)
		pesoL = fmt.Sprintf("%.3f", v.PesoLiq)
	}
	celulaCampo(pdf, margem, y, wQ, 9, "QUANTIDADE", qStr)
	celulaCampo(pdf, margem+wQ, y, wQ, 9, "ESPÉCIE", espStr)
	celulaCampo(pdf, margem+2*wQ, y, wQ, 9, "MARCA", marcaStr)
	celulaCampo(pdf, margem+3*wQ, y, wQ, 9, "NUMERAÇÃO", numStr)
	celulaCampo(pdf, margem+4*wQ, y, wQ, 9, "PESO BRUTO", pesoB)
	celulaCampo(pdf, margem+5*wQ, y, wQ, 9, "PESO LÍQUIDO", pesoL)
	y += 9

	return y
}

// ── Duplicatas ────────────────────────────────────────────────────────────────

func renderDuplicatas(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	// Cabeçalho da seção
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DUPLICATAS", "1", 2, "C", true, 0, "")
	y += 4

	const maxPorLinha = 8
	const altCelula = 10.0

	for inicio := 0; inicio < len(d.Duplicatas); inicio += maxPorLinha {
		fim := inicio + maxPorLinha
		if fim > len(d.Duplicatas) {
			fim = len(d.Duplicatas)
		}
		grupo := d.Duplicatas[inicio:fim]
		wCel := lw / float64(len(grupo))

		x := margem
		setarBorda(pdf)
		for _, dup := range grupo {
			w3 := wCel / 3

			// Borda da célula
			pdf.Rect(x, y, wCel, altCelula, "D")

			// Labels (5pt, cinza)
			pdf.SetFont("Arial", "", 5)
			pdf.SetTextColor(80, 80, 80)
			pdf.SetXY(x+0.5, y+0.5)
			pdf.CellFormat(w3-1, 3, "Nº", "", 0, "L", false, 0, "")
			pdf.SetXY(x+w3+0.5, y+0.5)
			pdf.CellFormat(w3-1, 3, "VENCIMENTO", "", 0, "L", false, 0, "")
			pdf.SetXY(x+2*w3+0.5, y+0.5)
			pdf.CellFormat(w3-1, 3, "VALOR", "", 0, "L", false, 0, "")

			// Valores (6pt, preto)
			pdf.SetFont("Arial", "B", 6)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetXY(x+0.5, y+4)
			pdf.CellFormat(w3-1, 5, dup.Num, "", 0, "L", false, 0, "")
			pdf.SetXY(x+w3+0.5, y+4)
			pdf.CellFormat(w3-1, 5, dup.Vencimento, "", 0, "L", false, 0, "")
			pdf.SetXY(x+2*w3+0.5, y+4)
			pdf.CellFormat(w3-1, 5, formatarMoeda(dup.Valor), "", 0, "R", false, 0, "")

			x += wCel
		}
		y += altCelula
	}

	return y
}

// ── Pagamento ─────────────────────────────────────────────────────────────────

func renderPagamento(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DADOS DO PAGAMENTO", "1", 2, "C", true, 0, "")
	y += 4

	wForma := lw * 0.50
	wValor := lw - wForma
	for _, p := range d.Pagamentos {
		celulaCampo(pdf, margem, y, wForma, 8, "FORMA DE PAGAMENTO", p.Forma)
		celulaCampo(pdf, margem+wForma, y, wValor, 8, "VALOR", formatarMoeda(p.Valor))
		y += 8
	}

	return y
}

// ── Dados adicionais ──────────────────────────────────────────────────────────

func renderDadosAdicionais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	if d.InfCpl == "" && d.InfAdFisco == "" {
		return y
	}

	// No modelo de referência esse bloco (sempre o último antes do rodapé
	// "Impresso em") fica encostado perto do fim da página, não logo depois
	// da seção anterior -- é a POSIÇÃO que empurra pra baixo quando sobra
	// espaço, a altura da caixa continua fixa (esticar a caixa deixa uma
	// caixa enorme quase vazia, pior que o problema original).
	const altBloco = 20.0
	const pageBottom = 297.0 - margem
	const rodapeReservado = 8.0
	yMinimo := pageBottom - rodapeReservado - altBloco - 4 // 4 = altura do título
	if y < yMinimo {
		y = yMinimo
	}

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DADOS ADICIONAIS", "1", 2, "C", true, 0, "")
	y += 4

	wInteresse := lw * 0.65
	wFisco := lw - wInteresse

	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(margem+1, y+0.5)
	pdf.CellFormat(wInteresse-2, 3, "INFORMAÇÕES COMPLEMENTARES DE INTERESSE DO CONTRIBUINTE", "", 0, "L", false, 0, "")
	pdf.SetXY(margem+wInteresse+1, y+0.5)
	pdf.CellFormat(wFisco-2, 3, "RESERVADO AO FISCO", "", 0, "L", false, 0, "")

	setarBorda(pdf)
	pdf.Rect(margem, y, wInteresse, altBloco, "D")
	pdf.Rect(margem+wInteresse, y, wFisco, altBloco, "D")

	pdf.SetFont("Arial", "", 6)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(margem+1, y+4)
	pdf.MultiCell(wInteresse-2, 3.5, d.InfCpl, "", "L", false)

	if d.InfAdFisco != "" {
		pdf.SetXY(margem+wInteresse+1, y+4)
		pdf.MultiCell(wFisco-2, 3.5, d.InfAdFisco, "", "L", false)
	}

	return y + altBloco
}

// ── Rodapé ────────────────────────────────────────────────────────────────────

func renderRodape(pdf *Doc, y float64) {
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(margem, y+2)
	pdf.CellFormat(larguraUtil, 3, "Impresso em "+time.Now().Format("02/01/2006")+" as "+time.Now().Format("15:04:05"), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

// renderMarcaCancelada estampa "CANCELADA" em vermelho, diagonal, sobre toda
// a página — mesmo padrão visual usado por DANFEs de mercado pra distinguir
// nota cancelada de nota válida à primeira vista.
func renderMarcaCancelada(pdf *Doc) {
	pdf.SetFont("Arial", "B", 60)
	pdf.SetTextColor(200, 0, 0)
	pdf.SetAlpha(0.35, "Normal")

	centroX, centroY := larguraPage/2, 148.0
	pdf.TransformBegin()
	pdf.TransformRotate(45, centroX, centroY)
	pdf.SetXY(0, centroY-15)
	pdf.CellFormat(larguraPage, 30, "CANCELADA", "", 0, "C", false, 0, "")
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

	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(200, 0, 0)
	pdf.SetXY(margem+1, y+1)
	pdf.MultiCell(larguraUtil-2, 3.2,
		"DANFE IMPRESSO EM CONTINGENCIA - EPEC REGULARMENTE RECEBIDA PELA RECEITA FEDERAL DO BRASIL",
		"", "C", false)

	pdf.SetFont("Arial", "", 7)
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
	cupomLarg = 80.0               // largura do papel cupom em mm
	cupomMarg = 3.0                // margem lateral em mm
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
