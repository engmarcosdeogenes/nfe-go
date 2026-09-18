package danfe

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Renderização do DANFE A-4 retrato em cima da geometria normativa de
// layout_anexo2.go (MOC 7.0 — Anexo II, item 3.8.1). Cada bloco desenha na
// posição e largura que o manual define; o que o manual não fixa está
// marcado com a seção que autoriza a escolha.

// renderizarPrevia é o mesmo desenho do DANFE, com tarja de pré-visualização
// no lugar da de cancelamento.
func renderizarPrevia(d *DadosDANFE, logo *Logo) ([]byte, error) {
	return renderizarComMarca(d, false, nil, logo, "PRE-VISUALIZACAO - SEM VALOR FISCAL", 28)
}

func renderizar(d *DadosDANFE, cancelada bool, epec *InfoEPEC, logo *Logo) ([]byte, error) {
	return renderizarComMarca(d, cancelada, epec, logo, "", 0)
}

func renderizarComMarca(d *DadosDANFE, cancelada bool, epec *InfoEPEC, logo *Logo, marca string, tamanhoMarca float64) ([]byte, error) {
	pdf := novoDoc(fpdf.New("P", "mm", "A4", ""))
	pdf.SetMargins(margem, margem, margem)
	pdf.SetAutoPageBreak(true, margem)
	// §3.10.2 — o número de ordem e o total de folhas têm que sair na parte
	// superior de todas as folhas, inclusive quando só existe uma. {nb} é o
	// alias que o fpdf troca pelo total real no fim da geração.
	pdf.AliasNbPages("")
	pdf.AddPage()

	// §3.3.1 — o canhoto só existe na primeira folha.
	renderCanhoto(pdf, d)

	// Cabeçalho normativo: emitente, "DANFE", número/série/folha, códigos de
	// barras, chave de acesso, natureza da operação e IE/IEST/CNPJ. É esse
	// conjunto que o §3.5 manda repetir em toda folha adicional.
	y := renderCabecalho(pdf, d, yCorpo, logo)
	y = renderNatureza(pdf, d, y)

	// Blocos exclusivos da primeira folha.
	y = renderDestinatario(pdf, d, y)
	y = renderDuplicatas(pdf, d, y)
	y = renderTotais(pdf, d, y)
	y = renderTransporte(pdf, d, y)

	// O quadro "Cálculo do ISSQN" fica suprimido: a lib não emite serviço com
	// ISSQN (não existe a tag correspondente no builder nem no parser), e o
	// §3.3.3 permite suprimi-lo nesse caso desde que a altura liberada vá para
	// o quadro "Dados dos Produtos/Serviços" — que é o que limiteItens faz.
	y = renderItens(pdf, d, y, logo, limiteItens(d, epec))

	y = renderPagamento(pdf, d, y)
	y = renderDadosAdicionais(pdf, d, y)
	if epec != nil {
		y = renderContingenciaEPEC(pdf, epec, y)
	}
	renderRodape(pdf, y)

	if cancelada {
		renderMarcaCancelada(pdf)
	}
	if marca != "" {
		renderMarcaDiagonal(pdf, marca, tamanhoMarca)
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

// limiteItens devolve o y máximo do quadro "Dados dos Produtos/Serviços": o
// fim da área útil da folha menos o que os blocos seguintes ainda consomem.
// É esse quadro que absorve toda folga do leiaute — o manual diz isso em três
// lugares (§3.3.2 fatura suprimida, §3.3.3 ISSQN suprimido, §3.10.3 margem
// maior por limitação de impressora): a altura liberada vai para os produtos,
// nunca vira espaço morto.
//
// Vale para toda página, não só a última. Reservar o espaço em todas custa
// algumas linhas de item e evita ter que saber de antemão qual é a última.
func limiteItens(d *DadosDANFE, epec *InfoEPEC) float64 {
	const altRodape = 4.0
	restante := altRodape + altTitulo + altDadosAdic
	if len(d.Pagamentos) > 0 {
		restante += altTitulo + altCampo*float64(len(d.Pagamentos))
	}
	if epec != nil {
		restante += 16
	}
	return alturaPage - margem - restante
}

// ── Helpers de desenho ────────────────────────────────────────────────────────

func setarBorda(pdf *Doc) {
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.1)
}

// textoNaCaixa prepara a fonte para escrever texto numa largura w: tenta
// tamMax, encolhe até tamMin e só então corta o que sobrar. CellFormat não
// corta nem quebra sozinho — sem isso um rótulo longo em célula estreita
// ("VALOR TOTAL DOS PRODUTOS" num quinto da largura) escreve por cima da
// borda e invade a margem da folha. tamMin é sempre o piso do item 3.7, e o
// corte só entra quando nem no piso cabe: perder o fim de um texto é menos
// grave que borrar o campo vizinho, e o item 3.7 exige legibilidade.
func textoNaCaixa(pdf *Doc, texto string, w, tamMax, tamMin float64) string {
	tam := tamMax
	pdf.SetFont("Times", "", tam)
	for tam > tamMin && pdf.GetStringWidth(pdf.tr(texto)) > w {
		tam -= 0.25
		pdf.SetFont("Times", "", tam)
	}
	for len(texto) > 1 && pdf.GetStringWidth(pdf.tr(texto)) > w {
		texto = texto[:len(texto)-1]
	}
	return texto
}

// quebrarRotulo divide o descritivo em até duas linhas quando ele não cabe em
// uma. O item 3.7.3 fixa 6pt como piso do descritivo de campo, então rótulo
// comprido em campo estreito ("OUTRAS DESPESAS ACESSÓRIAS" em 3,3cm, "DATA DA
// ENTRADA/SAÍDA" em 2,9cm) não se resolve encolhendo a fonte: ou quebra em
// duas linhas, ou perde o fim.
func quebrarRotulo(pdf *Doc, texto string, w float64) []string {
	pdf.SetFont("Times", "", fonteDescCampo)
	if pdf.GetStringWidth(pdf.tr(texto)) <= w {
		return []string{texto}
	}
	palavras := strings.Fields(texto)
	linha, i := "", 0
	for ; i < len(palavras); i++ {
		tentativa := palavras[i]
		if linha != "" {
			tentativa = linha + " " + palavras[i]
		}
		if linha != "" && pdf.GetStringWidth(pdf.tr(tentativa)) > w {
			break
		}
		linha = tentativa
	}
	return []string{linha, strings.Join(palavras[i:], " ")}
}

// celulaCampo desenha um campo do leiaute: borda, descritivo em cima
// (§3.7.3, mínimo 6pt, caixa alta) e conteúdo embaixo (§3.7.9, mínimo 10pt).
func celulaCampo(pdf *Doc, x, y, w, h float64, label, valor string) {
	setarBorda(pdf)
	pdf.Rect(x, y, w, h, "D")

	const altRotulo = 2.2
	pdf.SetTextColor(70, 70, 70)
	yRotulo := y + 0.3
	for _, linha := range quebrarRotulo(pdf, label, w-2) {
		linha = textoNaCaixa(pdf, linha, w-2, fonteDescCampo, fonteDescCampo)
		pdf.SetXY(x+1, yRotulo)
		pdf.CellFormat(w-2, altRotulo, linha, "", 0, "L", false, 0, "")
		yRotulo += altRotulo
	}

	pdf.SetTextColor(0, 0, 0)
	valor = textoNaCaixa(pdf, valor, w-2, fonteCampo, fonteCampo-2)
	pdf.SetXY(x+1, yRotulo)
	pdf.CellFormat(w-2, y+h-yRotulo-0.3, valor, "", 0, "L", false, 0, "")
}

// tituloBloco desenha a tarja com o nome do quadro (§3.7.1: negrito, caixa
// alta, mínimo 5pt) e devolve o y logo abaixo dela.
func tituloBloco(pdf *Doc, y float64, texto string) float64 {
	pdf.SetFont("Times", "B", fonteDescBloco)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(margem, y)
	pdf.CellFormat(larguraUtil, altTitulo, texto, "", 0, "L", false, 0, "")
	return y + altTitulo
}

// linhaCampos desenha uma linha de campos com as larguras do item 3.8.1.
func linhaCampos(pdf *Doc, y float64, larguras []float64, campos [][2]string) float64 {
	x := margem
	for i, l := range larguras {
		var label, valor string
		if i < len(campos) {
			label, valor = campos[i][0], campos[i][1]
		}
		celulaCampo(pdf, x, y, l, altCampo, label, valor)
		x += l
	}
	return y + altCampo
}

// ── Canhoto de recebimento ───────────────────────────────────────────────────

// renderCanhoto desenha a faixa destacável do topo, onde o recebedor assina
// confirmando o recebimento, com a linha de picote abaixo. Geometria do
// item 3.8.1: duas linhas de 8,5mm à esquerda (recibo / data + assinatura) e
// um bloco de 17mm à direita com NF-e, número e série.
func renderCanhoto(pdf *Doc, d *DadosDANFE) {
	const (
		wRecibo = 161.0
		wData   = 41.0
		xAssin  = margem + wData
		wAssin  = wRecibo - wData
		xNFe    = margem + wRecibo
		wNFe    = larguraUtil - wRecibo
	)
	y := yCanhoto

	setarBorda(pdf)
	pdf.Rect(margem, y, wRecibo, altCampo, "D")
	pdf.Rect(xNFe, y, wNFe, altCampo*2, "D")

	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}
	recebemos := fmt.Sprintf(
		"RECEBEMOS DE %s OS PRODUTOS/SERVIÇOS CONSTANTES NA NOTA FISCAL ELETRÔNICA INDICADA AO LADO - EMISSÃO: %s - VALOR TOTAL: R$ %s - DESTINATÁRIO: %s - %s",
		nome, d.DataEmissao, formatarMoeda(d.VNF), d.DestNome, enderecoLinha(d.DestEnd),
	)
	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(margem+1, y+0.6)
	pdf.MultiCell(wRecibo-2, 2.6, recebemos, "", "L", false)

	y += altCampo
	celulaCampo(pdf, margem, y, wData, altCampo, "DATA DE RECEBIMENTO", "")
	celulaCampo(pdf, xAssin, y, wAssin, altCampo, "IDENTIFICAÇÃO E ASSINATURA DO RECEBEDOR", "")

	pdf.SetFont("Times", "B", fonteNumSerie)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(xNFe+1, yCanhoto+1)
	pdf.MultiCell(wNFe-2, 4.2,
		"NF-e\nNº "+numeroFormatado(d.NumeroNota)+"\nSÉRIE "+fmt.Sprintf("%03s", d.Serie),
		"", "L", false)

	// Picote entre o canhoto e o corpo do DANFE.
	y += altCampo
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.1)
	pdf.SetDashPattern([]float64{1, 1}, 0)
	pdf.Line(margem, y+2, margem+larguraUtil, y+2)
	pdf.SetDashPattern([]float64{}, 0)
}

// ── Cabeçalho (repetido em toda folha, §3.5) ─────────────────────────────────

// altBarcode é a altura da barra do Code128. Mínimo oficial 0,8cm (Anexo II,
// item 2 "Código de Barras") — a versão antiga usava 7mm, abaixo do mínimo.
const altBarcode = 10.0

// renderCabecalho desenha os três quadros do topo do corpo: identificação do
// emitente (100mm), descrição "DANFE" com número/série/folha (25,4mm) e
// código de barras + chave de acesso (80,3mm), todos com 39,2mm de altura,
// conforme o item 3.8.1. O §3.5 obriga a repetir esse conjunto, na mesma
// disposição e tamanho, em toda folha adicional — por isso a função é chamada
// uma vez por página e não guarda estado.
func renderCabecalho(pdf *Doc, d *DadosDANFE, y float64, logo *Logo) float64 {
	const (
		wEmit  = 100.0
		xDanfe = margem + wEmit
		wDanfe = 25.4
		xChave = xDanfe + wDanfe
		wChave = larguraUtil - wEmit - wDanfe
	)

	setarBorda(pdf)
	pdf.Rect(margem, y, wEmit, altCabecalho, "D")
	pdf.Rect(xDanfe, y, wDanfe, altCabecalho, "D")

	// ── Quadro identificação do emitente ──
	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetTextColor(70, 70, 70)
	pdf.SetXY(margem+1, y+0.4)
	pdf.CellFormat(wEmit-2, 2.8, "IDENTIFICAÇÃO DO EMITENTE", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	// Com logo, reserva uma faixa fixa à esquerda (slot, não o tamanho real da
	// imagem) pra manter o texto no mesmo lugar independente da proporção do
	// arquivo enviado.
	xTexto, wTexto := margem+1, wEmit-2
	if logo != nil {
		const wSlot, altSlot = 20.0, 20.0
		if largura, altura := ajustarNaCaixa(logo, wSlot, altSlot); largura > 0 {
			imgName := fmt.Sprintf("logo_emit_%p", logo)
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: logo.Tipo}, bytes.NewReader(logo.Dados))
			pdf.Image(imgName, margem+1+(wSlot-largura)/2, y+5+(altSlot-altura)/2, largura, altura, false, "", 0, "")
			xTexto, wTexto = margem+1+wSlot+2, wEmit-2-wSlot-2
		}
	}

	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}
	pdf.SetXY(xTexto, y+4)
	pdf.SetFont("Times", "B", fonteEmitNome)
	pdf.MultiCell(wTexto, 5, nome, "", "C", false)

	end := d.EmitEnd
	pdf.SetFont("Times", "", fonteEmitDados)
	for _, linha := range []string{
		end.Logradouro + ", " + end.Numero,
		end.Bairro + " - CEP " + end.CEP,
		end.Municipio + " - " + end.UF,
		"Fone/Fax: " + end.Fone,
	} {
		pdf.SetX(xTexto)
		pdf.CellFormat(wTexto, 3.4, linha, "", 2, "C", false, 0, "")
	}

	// ── Quadro da descrição "DANFE" ──
	pdf.SetFont("Times", "B", fonteDANFE)
	pdf.SetXY(xDanfe, y+0.8)
	pdf.CellFormat(wDanfe, 5.5, "DANFE", "", 0, "C", false, 0, "")

	pdf.SetFont("Times", "", fonteDocAux)
	pdf.SetXY(xDanfe+1, y+6.3)
	pdf.MultiCell(wDanfe-2, 2.9, "DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA", "", "C", false)

	// §3.7.4 — tipo de operação em caixa alta e negrito (mín. 8pt), com o
	// dígito de tpNF destacado em caixa própria, como o leiaute do anexo
	// mostra. O quadro tem só 2,54cm de largura, então as duas linhas usam a
	// largura inteira e a caixa do dígito fica à direita da segunda linha
	// ("1 - SAÍDA" é curta e não encosta nela). Dividir a largura com a caixa
	// nas duas linhas fazia "0 - ENTRADA" quebrar e empurrava o número da nota
	// por cima da borda de baixo do quadro.
	digito := "1"
	if d.TipoNF == "0" {
		digito = "0"
	}
	pdf.SetFont("Times", "B", fonteDocAux)
	pdf.SetXY(xDanfe+1, y+17.9)
	pdf.MultiCell(wDanfe-2, 3.4, "0 - ENTRADA\n1 - SAÍDA", "", "L", false)
	pdf.SetFont("Times", "B", fonteNumSerie)
	pdf.SetXY(xDanfe+wDanfe-6.5, y+18.4)
	pdf.CellFormat(5.5, 5.8, digito, "1", 0, "C", false, 0, "")

	// Uma CellFormat por linha, não MultiCell: o total de folhas só é conhecido
	// no fim da geração, então aqui sai o alias "{nb}" literal -- mais largo que
	// o número que vai substituí-lo. MultiCell mediria o alias e quebraria a
	// linha antes da troca, jogando "FOLHA" e "1/1" em linhas separadas e pra
	// fora do quadro. CellFormat não quebra, e o texto final cabe.
	yNum := y + 24.9
	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetXY(xDanfe+1, yNum)
	pdf.CellFormat(wDanfe-2, 2.4, "Nº", "", 0, "L", false, 0, "")
	yNum += 2.4
	pdf.SetFont("Times", "B", fonteNumSerie)
	for _, linha := range []string{
		numeroFormatado(d.NumeroNota),
		fmt.Sprintf("SÉRIE %03s", d.Serie),
		fmt.Sprintf("FOLHA %d/{nb}", pdf.PageNo()),
	} {
		pdf.SetXY(xDanfe+1, yNum)
		pdf.CellFormat(wDanfe-2, 3.6, linha, "", 0, "L", false, 0, "")
		yNum += 3.6
	}

	// ── Quadro do código de barras e da chave de acesso ──
	setarBorda(pdf)
	pdf.Rect(xChave, y, wChave, altQuadroCB, "D")
	if d.ChaveAcesso != "" {
		if barcodeImg, err := gerarBarcodeCode128(d.ChaveAcesso); err == nil {
			imgName := "barcode_chave"
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(barcodeImg))
			pdf.Image(imgName, xChave+4, y+(altQuadroCB-altBarcode)/2, wChave-8, altBarcode, false, "", 0, "")
		}
	}

	yChave := y + altQuadroCB
	setarBorda(pdf)
	pdf.Rect(xChave, yChave, wChave, altCampo, "D")
	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetTextColor(70, 70, 70)
	pdf.SetXY(xChave+1, yChave+0.4)
	pdf.CellFormat(wChave-2, 2.8, "CHAVE DE ACESSO", "", 0, "L", false, 0, "")

	// §3.7.5 — a chave sai em negrito; o manual não fixa tamanho mínimo pra
	// ela, então pode encolher pra caber nos 80mm do quadro.
	pdf.SetTextColor(0, 0, 0)
	chave := formatarChave(d.ChaveAcesso)
	pdf.SetXY(xChave+1, yChave+3.4)
	chave = textoNaCaixa(pdf, chave, wChave-2, fonteCampo, 6.0)
	pdf.SetFontStyle("B")
	pdf.CellFormat(wChave-2, 4.4, chave, "", 0, "C", false, 0, "")

	yConsulta := yChave + altCampo
	altConsulta := altCabecalho - altQuadroCB - altCampo
	setarBorda(pdf)
	pdf.Rect(xChave, yConsulta, wChave, altConsulta, "D")
	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(xChave+1, yConsulta+2)
	pdf.MultiCell(wChave-2, 3.2,
		"Consulta de autenticidade no portal nacional da NF-e\nwww.nfe.fazenda.gov.br ou no site da Sefaz Autorizadora",
		"", "C", false)

	// §3.10.1 — marca d'água é permitida desde que não prejudique a leitura
	// dos dados impressos. Em homologação a NF-e não tem valor fiscal e o
	// aviso sai em toda folha.
	if d.TpAmb == "2" {
		pdf.SetFont("Times", "B", 46)
		pdf.SetTextColor(225, 225, 225)
		pdf.SetXY(margem, alturaPage/2)
		pdf.CellFormat(larguraUtil, 20, "SEM VALOR FISCAL", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	return y + altCabecalho
}

// ── Natureza da operação + identificação fiscal do emitente ──────────────────

// renderNatureza desenha as duas linhas que fecham o cabeçalho normativo:
// natureza da operação + protocolo de autorização, e IE / IE do substituto
// tributário / CNPJ do emitente. Também faz parte do que o §3.5 manda repetir
// em folha adicional.
func renderNatureza(pdf *Doc, d *DadosDANFE, y float64) float64 {
	const wNat = 125.4 // 0,25cm → 12,79cm, o mesmo corte vertical do cabeçalho

	protocolo := d.NumProtocolo
	if protocolo != "" && d.DataAutorizacao != "" {
		protocolo += " - " + d.DataAutorizacao
	}
	celulaCampo(pdf, margem, y, wNat, altCampo, "NATUREZA DA OPERAÇÃO", d.NatOp)
	celulaCampo(pdf, margem+wNat, y, larguraUtil-wNat, altCampo, "PROTOCOLO DE AUTORIZAÇÃO DE USO", protocolo)
	y += altCampo

	// O item 3.8.1 fixa três campos aqui: IE, IE do substituto tributário e
	// CNPJ. A inscrição municipal do emitente não mora nesta linha — ela é o
	// campo C19, do quadro "Cálculo do ISSQN", que está suprimido (§3.3.3).
	return linhaCampos(pdf, y, larguraEmitIE, [][2]string{
		{"INSCRIÇÃO ESTADUAL", d.EmitIE},
		{"INSCRIÇÃO ESTADUAL DO SUBST. TRIBUTÁRIO", d.EmitIEST},
		{"CNPJ", d.EmitCNPJ},
	})
}

// ── Destinatário / Remetente ─────────────────────────────────────────────────

func renderDestinatario(pdf *Doc, d *DadosDANFE, y float64) float64 {
	y = tituloBloco(pdf, y, "DESTINATÁRIO / REMETENTE")

	doc, labelDoc := d.DestCNPJ, "CNPJ"
	if doc == "" {
		doc, labelDoc = d.DestCPF, "CPF"
	}
	y = linhaCampos(pdf, y, larguraDestL1, [][2]string{
		{"NOME / RAZÃO SOCIAL", d.DestNome},
		{labelDoc, doc},
		{"DATA DA EMISSÃO", d.DataEmissao},
	})

	end := d.DestEnd
	endStr := end.Logradouro
	if end.Numero != "" {
		endStr += ", " + end.Numero
	}
	if end.Complemento != "" {
		endStr += " - " + end.Complemento
	}
	y = linhaCampos(pdf, y, larguraDestL2, [][2]string{
		{"ENDEREÇO", endStr},
		{"BAIRRO / DISTRITO", end.Bairro},
		{"CEP", end.CEP},
		{"DATA DA ENTRADA / SAÍDA", d.DataSaida},
	})

	return linhaCampos(pdf, y, larguraDestL3, [][2]string{
		{"MUNICÍPIO", end.Municipio},
		{"FONE / FAX", end.Fone},
		{"UF", end.UF},
		{"INSCRIÇÃO ESTADUAL", d.DestIE},
		{"HORA DA ENTRADA / SAÍDA", d.HoraSaida},
	})
}

// ── Fatura / Duplicatas ──────────────────────────────────────────────────────

// renderDuplicatas desenha o quadro "Fatura/Duplicatas". §3.3.2 permite
// suprimi-lo quando o contribuinte não usa esses documentos, desde que a
// altura liberada vá para o quadro "Dados dos Produtos/Serviços" — que é o
// que acontece, já que o limite dos itens é calculado a partir do fim da
// folha e não da posição nominal deste quadro.
func renderDuplicatas(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if len(d.Duplicatas) == 0 {
		return y
	}
	y = tituloBloco(pdf, y, "FATURA / DUPLICATAS")

	setarBorda(pdf)
	pdf.Rect(margem, y, larguraUtil, altFatura, "D")

	const porLinha = 5
	wCel := larguraUtil / porLinha
	pdf.SetTextColor(0, 0, 0)
	for i, dup := range d.Duplicatas {
		if i >= porLinha*2 {
			break
		}
		x := margem + float64(i%porLinha)*wCel
		yy := y + float64(i/porLinha)*(altFatura/2)
		texto := fmt.Sprintf("%s  %s  %s", dup.Num, dup.Vencimento, formatarMoeda(dup.Valor))
		texto = textoNaCaixa(pdf, texto, wCel-2, fonteCampo, fonteCampo-2)
		pdf.SetXY(x+1, yy+0.8)
		pdf.CellFormat(wCel-2, 4.4, texto, "", 0, "L", false, 0, "")
	}

	return y + altFatura
}

// ── Cálculo do imposto ───────────────────────────────────────────────────────

// renderTotais desenha o quadro "Cálculo do Imposto" com os campos e as
// larguras do item 3.8.1: cinco na primeira linha e seis na segunda. A versão
// anterior desenhava 7+7, incluindo PIS e COFINS, que não são campos do
// leiaute — e deslocava tudo que vinha depois.
//
// DIFAL, FCP e total de tributos não têm campo próprio no leiaute; o §3.1.8
// manda levá-los para "Informações Complementares", que é o que
// informacoesComplementares faz.
func renderTotais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	y = tituloBloco(pdf, y, "CÁLCULO DO IMPOSTO")

	y = linhaCampos(pdf, y, larguraImpostoL1, [][2]string{
		{"BASE DE CÁLCULO DO ICMS", formatarMoeda(d.VBC)},
		{"VALOR DO ICMS", formatarMoeda(d.VICMS)},
		{"BASE DE CÁLCULO DO ICMS ST", formatarMoeda(d.VBCST)},
		{"VALOR DO ICMS ST", formatarMoeda(d.VST)},
		{"VALOR TOTAL DOS PRODUTOS", formatarMoeda(d.VProd)},
	})

	return linhaCampos(pdf, y, larguraImpostoL2, [][2]string{
		{"VALOR DO FRETE", formatarMoeda(d.VFrete)},
		{"VALOR DO SEGURO", formatarMoeda(d.VSeg)},
		{"DESCONTO", formatarMoeda(d.VDesc)},
		{"OUTRAS DESPESAS ACESSÓRIAS", formatarMoeda(d.VOutro)},
		{"VALOR DO IPI", formatarMoeda(d.VIPI)},
		{"VALOR TOTAL DA NOTA", formatarMoeda(d.VNF)},
	})
}

// ── Transportador / Volumes transportados ────────────────────────────────────

// renderTransporte desenha as três linhas do quadro, sempre. A versão
// anterior colapsava para uma linha quando não havia transportador nem
// volume; o item 3.8.1 fixa as três, e colapsar desloca verticalmente todos
// os quadros seguintes.
func renderTransporte(pdf *Doc, d *DadosDANFE, y float64) float64 {
	y = tituloBloco(pdf, y, "TRANSPORTADOR / VOLUMES TRANSPORTADOS")

	y = linhaCampos(pdf, y, larguraTranspL1, [][2]string{
		{"RAZÃO SOCIAL", d.TranspNome},
		{"FRETE POR CONTA", descricaoModFrete(d.ModFrete)},
		{"CÓDIGO ANTT", d.TranspANTT},
		{"PLACA DO VEÍCULO", d.TranspPlaca},
		{"UF", d.TranspPlacaUF},
		{"CNPJ / CPF", d.TranspCNPJ},
	})

	y = linhaCampos(pdf, y, larguraTranspL2, [][2]string{
		{"ENDEREÇO", d.TranspEnd},
		{"MUNICÍPIO", d.TranspMun},
		{"UF", d.TranspUF},
		{"INSCRIÇÃO ESTADUAL", d.TranspIE},
	})

	var qtd, especie, marca, numeracao, pesoB, pesoL string
	if len(d.Volumes) > 0 {
		v := d.Volumes[0]
		qtd = fmt.Sprintf("%g", v.Quantidade)
		especie, marca, numeracao = v.Especie, v.Marca, v.Numeracao
		pesoB, pesoL = fmt.Sprintf("%.3f", v.PesoBruto), fmt.Sprintf("%.3f", v.PesoLiq)
	}
	return linhaCampos(pdf, y, larguraTranspL3, [][2]string{
		{"QUANTIDADE", qtd},
		{"ESPÉCIE", especie},
		{"MARCA", marca},
		{"NUMERAÇÃO", numeracao},
		{"PESO BRUTO", pesoB},
		{"PESO LÍQUIDO", pesoL},
	})
}

// ── Dados dos produtos / serviços ────────────────────────────────────────────

func renderItens(pdf *Doc, d *DadosDANFE, y float64, logo *Logo, limite float64) float64 {
	cols := colunasItens
	const idxDescricao = 1

	// Cabeçalho das colunas. §3.7.2: descritivo em caixa alta, mínimo 5pt.
	// Vira closure porque cada folha adicional redesenha o mesmo cabeçalho.
	// Os rótulos quebram em duas linhas em vez de serem cortados: as colunas
	// de imposto são estreitas e 5pt já é o piso do §3.7.2, então não sobra
	// folga pra encolher ("VALOR TOTAL" e "ALÍQ. ICMS" saíam truncados).
	cabecalhoColunas := func(y float64) float64 {
		const alt = 6.0
		setarBorda(pdf)
		pdf.SetTextColor(0, 0, 0)
		x := margem
		for _, c := range cols {
			pdf.Rect(x, y, c.w, alt, "D")
			linhas := quebrarRotulo(pdf, c.label, c.w-1)
			yy := y + (alt-float64(len(linhas))*2.4)/2
			for _, linha := range linhas {
				linha = textoNaCaixa(pdf, linha, c.w-1, fonteDescItens, fonteDescItens)
				pdf.SetFontStyle("B")
				pdf.SetXY(x, yy)
				pdf.CellFormat(c.w, 2.4, linha, "", 0, "C", false, 0, "")
				yy += 2.4
			}
			x += c.w
		}
		return y + alt
	}

	// §3.5 — toda folha adicional repete, na mesma disposição e tamanho da
	// primeira, os dados do emitente, a descrição "DANFE", número/série/folha,
	// os códigos de barras, natureza da operação, chave de acesso e o
	// IE/IEST/CNPJ do emitente. Só depois disso o quadro de itens continua.
	novaFolha := func() float64 {
		pdf.AddPage()
		// A folha adicional não tem canhoto (§3.3.1), então o cabeçalho sobe
		// pro topo da área útil em vez de ficar na âncora da primeira folha --
		// senão sobravam 2cm de papel branco no alto de cada folha extra, que
		// o §3.5 permite usar pros itens que não couberam.
		yy := renderCabecalho(pdf, d, margem, logo)
		yy = renderNatureza(pdf, d, yy)
		yy = tituloBloco(pdf, yy, "DADOS DOS PRODUTOS / SERVIÇOS")
		return cabecalhoColunas(yy)
	}

	y = tituloBloco(pdf, y, "DADOS DOS PRODUTOS / SERVIÇOS")
	y = cabecalhoColunas(y)

	for _, item := range d.Itens {
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

		// A descrição é a única coluna que pode usar mais linhas (§3.2, último
		// parágrafo); a altura da linha inteira acompanha ela.
		pdf.SetFont("Times", "", fonteItens)
		linhasDesc := pdf.SplitLines([]byte(pdf.tr(item.XProd)), cols[idxDescricao].w-2)
		altLinha := 4.0
		if len(linhasDesc) > 1 {
			altLinha = float64(len(linhasDesc)) * 3.2
		}

		if y+altLinha > limite {
			y = novaFolha()
		}

		x := margem
		for i, c := range cols {
			if i == idxDescricao {
				// MultiCell desenha a borda por linha de texto, com altura
				// fixa, que não bate com a altura real da linha da tabela —
				// sobrava um vão entre as duas bordas. Retângulo à parte, na
				// altura certa, e o texto por cima sem borda própria.
				setarBorda(pdf)
				pdf.Rect(x, y, c.w, altLinha, "D")
				pdf.SetFont("Times", "", fonteItens)
				pdf.SetXY(x+1, y+0.4)
				pdf.MultiCell(c.w-2, 3.2, vals[i], "", "L", false)
			} else {
				texto := textoNaCaixa(pdf, vals[i], c.w-1.5, fonteItens, fonteItens)
				pdf.SetXY(x, y)
				pdf.CellFormat(c.w, altLinha, texto, "1", 0, c.align, false, 0, "")
			}
			x += c.w
		}
		y += altLinha
	}

	// O quadro vai até o limite mesmo depois do último item: §3.3.2, §3.3.3 e
	// §3.10.3 dizem que a altura liberada por outros quadros é somada aqui, e
	// o leiaute do anexo mostra o quadro fechado, com as verticais das colunas
	// descendo até a base. Sem isso a folha terminava no último item e sobrava
	// um terço de A4 em branco.
	if y < limite {
		setarBorda(pdf)
		pdf.Rect(margem, y, larguraUtil, limite-y, "D")
		x := margem
		for _, c := range cols[:len(cols)-1] {
			x += c.w
			pdf.Line(x, y, x, limite)
		}
		y = limite
	}

	return y
}

// ── Dados do pagamento ───────────────────────────────────────────────────────

// renderPagamento desenha as formas de pagamento. O quadro não existe no
// item 3.8.1 (foi criado pela NT 2016.002, posterior ao desenho do leiaute);
// fica entre os produtos e os dados adicionais, que é onde o mercado o
// posicionou.
func renderPagamento(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if len(d.Pagamentos) == 0 {
		return y
	}
	y = tituloBloco(pdf, y, "DADOS DO PAGAMENTO")

	const wForma = 129.5
	for _, p := range d.Pagamentos {
		celulaCampo(pdf, margem, y, wForma, altCampo, "FORMA DE PAGAMENTO", p.Forma)
		celulaCampo(pdf, margem+wForma, y, larguraUtil-wForma, altCampo, "VALOR", formatarMoeda(p.Valor))
		y += altCampo
	}
	return y
}

// ── Dados adicionais ─────────────────────────────────────────────────────────

// renderDadosAdicionais desenha "Informações Complementares" (12,95cm) e
// "Reservado ao Fisco" (7,62cm), ambos com 3,07cm de altura, conforme o
// item 3.8.1. §3.1.9: o contribuinte não preenche o quadro do fisco.
func renderDadosAdicionais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	y = tituloBloco(pdf, y, "DADOS ADICIONAIS")

	wCpl, wFisco := larguraAdic[0], larguraAdic[1]

	setarBorda(pdf)
	pdf.Rect(margem, y, wCpl, altDadosAdic, "D")
	pdf.Rect(margem+wCpl, y, wFisco, altDadosAdic, "D")

	pdf.SetFont("Times", "", fonteDescCampo)
	pdf.SetTextColor(70, 70, 70)
	pdf.SetXY(margem+1, y+0.4)
	pdf.CellFormat(wCpl-2, 2.8, "INFORMAÇÕES COMPLEMENTARES", "", 0, "L", false, 0, "")
	pdf.SetXY(margem+wCpl+1, y+0.4)
	pdf.CellFormat(wFisco-2, 2.8, "RESERVADO AO FISCO", "", 0, "L", false, 0, "")

	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Times", "", fonteInfCpl)
	if texto := informacoesComplementares(d); texto != "" {
		pdf.SetXY(margem+1, y+3.6)
		pdf.MultiCell(wCpl-2, 2.8, texto, "", "L", false)
	}
	if d.InfAdFisco != "" {
		pdf.SetXY(margem+wCpl+1, y+3.6)
		pdf.MultiCell(wFisco-2, 2.8, d.InfAdFisco, "", "L", false)
	}

	return y + altDadosAdic
}

// informacoesComplementares monta o conteúdo do campo. Além do infCpl da
// própria NF-e, o §3.1.8 manda incluir aqui os valores do ICMS interestadual
// (DIFAL e FCP), e o §3.10.5 manda copiar para cá qualquer valor que não tenha
// campo próprio no leiaute — é o caso do total de tributos. Não existe campo
// de DIFAL no quadro "Cálculo do Imposto", então a linha extra que a versão
// anterior desenhava lá não era conforme.
func informacoesComplementares(d *DadosDANFE) string {
	texto := d.InfCpl
	if d.VICMSUFDest != 0 || d.VFCPUFDest != 0 || d.VICMSUFRemet != 0 {
		texto = juntarLinha(texto, fmt.Sprintf(
			"Valores totais do ICMS Interestadual: DIFAL da UF destino R$%s + FCP R$%s; DIFAL da UF origem R$%s.",
			formatarMoeda(d.VICMSUFDest), formatarMoeda(d.VFCPUFDest), formatarMoeda(d.VICMSUFRemet)))
	}
	if d.VTotTrib != 0 {
		texto = juntarLinha(texto, "Valor aproximado dos tributos: R$"+formatarMoeda(d.VTotTrib))
	}
	return texto
}

func juntarLinha(texto, linha string) string {
	if texto == "" {
		return linha
	}
	return texto + "\n" + linha
}

// ── Rodapé ───────────────────────────────────────────────────────────────────

func renderRodape(pdf *Doc, y float64) {
	pdf.SetFont("Times", "", 5)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(margem, y+0.6)
	pdf.CellFormat(larguraUtil, 3, "Impresso em "+time.Now().Format("02/01/2006 15:04:05"), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

// numeroFormatado devolve o número da NF-e no formato 000.000.000 usado no
// leiaute ("NF-e / Nº 000.000.000 / SÉRIE 000", item 3.8.1).
func numeroFormatado(numero string) string {
	n := fmt.Sprintf("%09s", numero)
	if len(n) != 9 {
		return numero
	}
	return n[0:3] + "." + n[3:6] + "." + n[6:9]
}

// enderecoLinha monta o endereço em uma linha só, pro texto do canhoto.
func enderecoLinha(e enderecoDANFE) string {
	s := e.Logradouro
	if e.Numero != "" {
		s += ", " + e.Numero
	}
	if e.Bairro != "" {
		s += " - " + e.Bairro
	}
	if e.Municipio != "" {
		s += " - " + e.Municipio
	}
	if e.UF != "" {
		s += "/" + e.UF
	}
	return s
}
