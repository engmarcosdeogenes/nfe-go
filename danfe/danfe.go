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
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
)

// Gerar recebe o XML de uma NF-e (assinada ou nfeProc com protocolo)
// e retorna os bytes do PDF do DANFE.
func Gerar(nfeXML []byte) ([]byte, error) {
	dados, err := ParseNFeXML(nfeXML)
	if err != nil {
		return nil, fmt.Errorf("danfe: %w", err)
	}
	return renderizar(dados)
}

// ── Constantes de layout ──────────────────────────────────────────────────────

const (
	margem      = 5.0  // margem lateral em mm
	larguraPage = 210.0
	larguraUtil = larguraPage - 2*margem
	corBorda    = "cinza"
)

// ── Renderização ──────────────────────────────────────────────────────────────

func renderizar(d *DadosDANFE) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margem, margem, margem)
	pdf.SetAutoPageBreak(true, 5)
	pdf.AddPage()

	// Fonte padrão
	pdf.SetFont("Arial", "", 7)

	// ── Bloco 1: Cabeçalho ────────────────────────────────────────────────────
	y := renderCabecalho(pdf, d)

	// ── Bloco 2: Chave de acesso + barcode ────────────────────────────────────
	y = renderChave(pdf, d, y)

	// ── Bloco 3: Destinatário ─────────────────────────────────────────────────
	y = renderDestinatario(pdf, d, y)

	// ── Bloco 4: Itens ────────────────────────────────────────────────────────
	y = renderItens(pdf, d, y)

	// ── Bloco 5: Cálculo do imposto ───────────────────────────────────────────
	y = renderTotais(pdf, d, y)

	// ── Bloco 6: Transporte ───────────────────────────────────────────────────
	y = renderTransporte(pdf, d, y)

	// ── Bloco 7: Duplicatas (cobrança parcelada) ──────────────────────────────
	if len(d.Duplicatas) > 0 {
		y = renderDuplicatas(pdf, d, y)
	}

	// ── Bloco 8: Dados de pagamento ───────────────────────────────────────────
	if len(d.Pagamentos) > 0 {
		y = renderPagamento(pdf, d, y)
	}

	// ── Bloco 9: Dados adicionais ─────────────────────────────────────────────
	renderDadosAdicionais(pdf, d, y)

	if pdf.Err() {
		return nil, fmt.Errorf("danfe: fpdf: %s", pdf.Error())
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("danfe: output: %w", err)
	}
	return buf.Bytes(), nil
}

// ── Funções de helpers de desenho ─────────────────────────────────────────────

func setarBorda(pdf *fpdf.Fpdf) {
	pdf.SetDrawColor(150, 150, 150)
	pdf.SetLineWidth(0.2)
}

func celulaCampo(pdf *fpdf.Fpdf, x, y, w, h float64, label, valor string) {
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

func renderCabecalho(pdf *fpdf.Fpdf, d *DadosDANFE) float64 {
	y := margem
	lw := larguraUtil

	altCab := 30.0

	// Bloco emitente (esq 40%)
	wEmit := lw * 0.40
	setarBorda(pdf)
	pdf.Rect(margem, y, wEmit, altCab, "D")
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(margem+1, y+4)
	nome := d.EmitNome
	if d.EmitFantasia != "" {
		nome = d.EmitFantasia
	}
	pdf.MultiCell(wEmit-2, 5, nome, "", "C", false)
	pdf.SetFont("Arial", "", 6)
	end := d.EmitEnd
	pdf.SetX(margem + 1)
	pdf.CellFormat(wEmit-2, 3.5, end.Logradouro+", "+end.Numero, "", 2, "C", false, 0, "")
	pdf.SetX(margem + 1)
	pdf.CellFormat(wEmit-2, 3.5, end.Bairro+" - "+end.Municipio+"/"+end.UF, "", 2, "C", false, 0, "")
	pdf.SetX(margem + 1)
	pdf.CellFormat(wEmit-2, 3.5, "CEP: "+end.CEP+"  Fone: "+end.Fone, "", 2, "C", false, 0, "")
	pdf.SetX(margem + 1)
	pdf.CellFormat(wEmit-2, 3.5, "CNPJ: "+d.EmitCNPJ+"  IE: "+d.EmitIE, "", 2, "C", false, 0, "")

	// Bloco central DANFE (centro 20%)
	wCentro := lw * 0.22
	xCentro := margem + wEmit
	setarBorda(pdf)
	pdf.Rect(xCentro, y, wCentro, altCab, "D")
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(xCentro, y+3)
	pdf.CellFormat(wCentro, 5, "DANFE", "", 2, "C", false, 0, "")
	pdf.SetFont("Arial", "", 5.5)
	pdf.SetX(xCentro)
	pdf.MultiCell(wCentro, 3.5, "Documento Auxiliar da\nNota Fiscal Eletrônica", "", "C", false)
	pdf.SetFont("Arial", "", 6)
	tipoDesc := "1 - SAÍDA"
	if d.TipoNF == "0" {
		tipoDesc = "0 - ENTRADA"
	}
	pdf.SetXY(xCentro+1, y+19)
	pdf.CellFormat(wCentro-2, 4, "ENTRADA / SAÍDA", "", 2, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.SetX(xCentro)
	pdf.CellFormat(wCentro, 5, tipoDesc[:1], "", 2, "C", false, 0, "")

	// Bloco NF-e (dir 38%)
	wDir := lw - wEmit - wCentro
	xDir := xCentro + wCentro
	setarBorda(pdf)
	pdf.Rect(xDir, y, wDir, altCab, "D")

	// Número e série
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(xDir+1, y+1)
	pdf.CellFormat(wDir-2, 3, "NF-e  N°.", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(xDir+1, y+4)
	nfNum := fmt.Sprintf("%09s", d.NumeroNota)
	// Formatar: 000.000.001
	if len(nfNum) == 9 {
		nfNum = nfNum[0:3] + "." + nfNum[3:6] + "." + nfNum[6:9]
	}
	pdf.CellFormat(wDir-2, 5, nfNum, "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(xDir + 1)
	pdf.CellFormat(wDir-2, 3, "SÉRIE", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(xDir + 1)
	pdf.CellFormat(wDir-2, 4, d.Serie, "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(xDir + 1)
	pdf.CellFormat(wDir-2, 3, "DATA E HORA DE EMISSÃO", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 7)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(xDir + 1)
	pdf.CellFormat(wDir-2, 4, d.DataEmissao, "", 2, "L", false, 0, "")

	// Protocolo de autorização
	if d.NumProtocolo != "" {
		pdf.SetFont("Arial", "", 5)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetX(xDir + 1)
		pdf.CellFormat(wDir-2, 3, "PROTOCOLO DE AUTORIZAÇÃO DE USO", "", 2, "L", false, 0, "")
		pdf.SetFont("Arial", "", 6)
		pdf.SetTextColor(0, 80, 0)
		pdf.SetX(xDir + 1)
		pdf.CellFormat(wDir-2, 4, d.NumProtocolo, "", 2, "L", false, 0, "")
	}

	// Homologação watermark
	if d.TpAmb == "2" {
		pdf.SetFont("Arial", "B", 48)
		pdf.SetTextColor(220, 220, 220)
		pdf.SetXY(margem, y+60)
		pdf.CellFormat(lw, 20, "SEM VALOR FISCAL", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	return y + altCab
}

// ── Chave de acesso ───────────────────────────────────────────────────────────

func renderChave(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
	altBloco := 18.0
	lw := larguraUtil

	setarBorda(pdf)
	pdf.Rect(margem, y, lw, altBloco, "D")

	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(margem+1, y+1)
	pdf.CellFormat(lw-2, 3, "CHAVE DE ACESSO", "", 2, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(margem + 1)
	pdf.CellFormat(lw-2, 4, formatarChave(d.ChaveAcesso), "", 2, "C", false, 0, "")

	// Barcode Code 128
	if d.ChaveAcesso != "" {
		barcodeImg, err := gerarBarcodeCode128(d.ChaveAcesso)
		if err == nil {
			imgName := "barcode_chave"
			pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(barcodeImg))
			xBarcode := margem + (lw-80)/2
			pdf.Image(imgName, xBarcode, y+8.5, 80, 7, false, "", 0, "")
		}
	}

	pdf.SetFont("Arial", "", 5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(margem+1, y+altBloco-4)
	consult := "Consulta de autenticidade no portal nacional da NF-e: www.nfe.fazenda.gov.br/portal"
	pdf.CellFormat(lw-2, 3.5, consult, "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	return y + altBloco
}

// ── Destinatário ──────────────────────────────────────────────────────────────

func renderDestinatario(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
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

	// Linha 2: endereço, bairro, CEP, municipio, UF, fone, IE
	end := d.DestEnd
	endStr := end.Logradouro
	if end.Numero != "" {
		endStr += ", " + end.Numero
	}
	if end.Complemento != "" {
		endStr += " - " + end.Complemento
	}
	wEnd := lw * 0.38
	wBairro := lw * 0.22
	wCEP := lw * 0.12
	wMun := lw * 0.18
	wUF := lw * 0.05
	wFone := lw - wEnd - wBairro - wCEP - wMun - wUF
	celulaCampo(pdf, margem, y, wEnd, 9, "ENDEREÇO", endStr)
	celulaCampo(pdf, margem+wEnd, y, wBairro, 9, "BAIRRO", end.Bairro)
	celulaCampo(pdf, margem+wEnd+wBairro, y, wCEP, 9, "CEP", end.CEP)
	celulaCampo(pdf, margem+wEnd+wBairro+wCEP, y, wMun, 9, "MUNICÍPIO", end.Municipio)
	celulaCampo(pdf, margem+wEnd+wBairro+wCEP+wMun, y, wUF, 9, "UF", end.UF)
	celulaCampo(pdf, margem+wEnd+wBairro+wCEP+wMun+wUF, y, wFone, 9, "FONE/FAX", end.Fone)
	y += 9

	_ = alt
	return y
}

// ── Itens ─────────────────────────────────────────────────────────────────────

func renderItens(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	// Cabeçalho da tabela
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DADOS DOS PRODUTOS / SERVIÇOS", "1", 2, "C", true, 0, "")
	y += 4

	// Colunas: num, código, descrição, NCM, CST, CFOP, un, qtd, v.unit, v.desc, v.prod, v.BC, v.ICMS, al%, IPI
	cols := []struct {
		label string
		w     float64
		align string
	}{
		{"#", 5, "C"},
		{"CÓD.", 12, "C"},
		{"DESCRIÇÃO DO PRODUTO", 50, "L"},
		{"NCM", 13, "C"},
		{"CST", 8, "C"},
		{"CFOP", 9, "C"},
		{"UN", 7, "C"},
		{"QTDE", 12, "R"},
		{"VL UNIT.", 14, "R"},
		{"VL DESC.", 14, "R"},
		{"VL TOTAL", 14, "R"},
		{"VL IPI", 12, "R"},
	}

	// Linha de cabeçalho das colunas
	pdf.SetFont("Arial", "B", 5)
	pdf.SetFillColor(245, 245, 245)
	x := margem
	for _, c := range cols {
		pdf.SetXY(x, y)
		pdf.CellFormat(c.w, 5, c.label, "1", 0, "C", true, 0, "")
		x += c.w
	}
	y += 5

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
			fmt.Sprintf("%d", item.Num),
			item.CProd,
			item.XProd,
			item.NCM,
			item.CST,
			item.CFOP,
			item.Unidade,
			formatarQtd(item.Qtd),
			formatarMoeda(item.VUnit),
			formatarMoeda(item.VDesc),
			formatarMoeda(item.VProd),
			formatarMoeda(item.IPI),
		}

		// Calcular altura necessária para o nome do produto
		altLinha := 5.0
		descW := cols[2].w - 2
		pdf.SetFont("Arial", "", 6)
		linhasDesc := pdf.SplitLines([]byte(item.XProd), descW)
		if len(linhasDesc) > 1 {
			altLinha = float64(len(linhasDesc)) * 3.5
		}

		for i, c := range cols {
			pdf.SetXY(x, y)
			if i == 2 {
				// Descrição com wrap
				pdf.SetFont("Arial", "", 6)
				pdf.MultiCell(c.w, 3.5, vals[i], "1", "L", false)
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

func renderTotais(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "CÁLCULO DO IMPOSTO", "1", 2, "C", true, 0, "")
	y += 4

	// Linha 1
	w := lw / 7
	campos1 := [][2]string{
		{"BASE CÁLC. ICMS", formatarMoeda(d.VBC)},
		{"VALOR DO ICMS", formatarMoeda(d.VICMS)},
		{"BASE CÁLC. ICMS ST", formatarMoeda(d.VBCST)},
		{"VALOR ICMS ST", formatarMoeda(d.VST)},
		{"VL APROX. TRIB.", "0,00"},
		{"VALOR DO IPI", formatarMoeda(d.VIPI)},
		{"VALOR DO PIS", formatarMoeda(d.VPIS)},
	}
	for i, c := range campos1 {
		celulaCampo(pdf, margem+float64(i)*w, y, w, 9, c[0], c[1])
	}
	y += 9

	// Linha 2
	campos2 := [][2]string{
		{"VL. COFINS", formatarMoeda(d.VCOFINS)},
		{"VL. FRETE", formatarMoeda(d.VFrete)},
		{"VL. SEGURO", formatarMoeda(d.VSeg)},
		{"DESCONTO", formatarMoeda(d.VDesc)},
		{"OUTRAS DESPESAS", formatarMoeda(d.VOutro)},
		{"VL. TOTAL PRODUTOS", formatarMoeda(d.VProd)},
		{"VALOR TOTAL DA NF", formatarMoeda(d.VNF)},
	}
	for i, c := range campos2 {
		celulaCampo(pdf, margem+float64(i)*w, y, w, 9, c[0], c[1])
	}
	y += 9

	return y
}

// ── Transporte ────────────────────────────────────────────────────────────────

func renderTransporte(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
	lw := larguraUtil

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "TRANSPORTADOR / VOLUMES TRANSPORTADOS", "1", 2, "C", true, 0, "")
	y += 4

	// Linha 1: Nome/RS | Frete | CNPJ/CPF | IE
	wNome := lw * 0.40
	wFrete := lw * 0.15
	wCNPJ := lw * 0.27
	wIE := lw - wNome - wFrete - wCNPJ
	celulaCampo(pdf, margem, y, wNome, 9, "RAZÃO SOCIAL DO TRANSPORTADOR", d.TranspNome)
	celulaCampo(pdf, margem+wNome, y, wFrete, 9, "MODALIDADE DO FRETE", descricaoModFrete(d.ModFrete))
	celulaCampo(pdf, margem+wNome+wFrete, y, wCNPJ, 9, "CNPJ / CPF", d.TranspCNPJ)
	celulaCampo(pdf, margem+wNome+wFrete+wCNPJ, y, wIE, 9, "IE", d.TranspIE)
	y += 9

	// Linha 2: Endereço | Município | UF
	wEnd := lw * 0.55
	wMun := lw * 0.35
	wUF := lw - wEnd - wMun
	celulaCampo(pdf, margem, y, wEnd, 9, "ENDEREÇO", d.TranspEnd)
	celulaCampo(pdf, margem+wEnd, y, wMun, 9, "MUNICÍPIO", d.TranspMun)
	celulaCampo(pdf, margem+wEnd+wMun, y, wUF, 9, "UF", d.TranspUF)
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

func renderDuplicatas(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
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

func renderPagamento(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) float64 {
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

func renderDadosAdicionais(pdf *fpdf.Fpdf, d *DadosDANFE, y float64) {
	lw := larguraUtil

	if d.InfCpl == "" && d.InfAdFisco == "" {
		return
	}

	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetXY(margem, y)
	pdf.CellFormat(lw, 4, "DADOS ADICIONAIS", "1", 2, "C", true, 0, "")
	y += 4

	altBloco := 20.0
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
