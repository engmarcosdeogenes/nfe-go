package danfe

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

// DANFE NFC-e (cupom) conforme o Manual de Padrões DANFE NFC-e e QR Code
// v6.0 (mar/2025). A ordem do documento segue as figuras 5 (QR centralizado) e
// 6 (contingência) do manual:
//
//	I    cabeçalho (CNPJ/CPF, razão social, endereço, título fixo)
//	     [contingência: "EMITIDA EM CONTINGÊNCIA / Pendente de autorização"]
//	II   itens (código, descrição, qtde, un, valor unit., valor total)
//	III  totais, formas de pagamento, troco
//	IV   "Consulte pela Chave de Acesso em" + URL + chave em 11 blocos de 4
//	VI   consumidor (CNPJ/CPF/Id. Estrangeiro ou "CONSUMIDOR NÃO IDENTIFICADO")
//	VII  número, série, data/hora de emissão, protocolo de autorização
//	VIII mensagem fiscal (infAdFisco; contingência; homologação)
//	V    QR Code (mín. 25 mm, nível M, UTF-8)
//	IX   informações complementares do contribuinte (infCpl)
const (
	cupomLarg = 80.0                    // largura do papel em mm (mínimo do manual: 56 mm)
	cupomMarg = 3.0                     // margem lateral em mm (mínimo do manual: 2 mm)
	cupomLW   = cupomLarg - 2*cupomMarg // largura útil = 74 mm
	cupomQR   = 38.0                    // lado do QR Code com a margem segura, em mm (mínimo do manual: 25 mm)
)

// Textos fixos do manual v6.0.
const (
	textoTituloNFCe         = "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica" // 3.1.1
	textoConsultaChave      = "Consulte pela Chave de Acesso em"                           // 3.1.4
	textoNaoIdentificado    = "CONSUMIDOR NÃO IDENTIFICADO"                                // 3.1.6
	textoProtocolo          = "Protocolo de autorização:"                                  // 3.1.7
	textoContingencia1      = "EMITIDA EM CONTINGÊNCIA"                                    // 3.1.8
	textoContingencia2      = "Pendente de autorização"                                    // 3.1.8
	textoHomologacao        = "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO – SEM VALOR FISCAL"      // 3.1.8
	textoViaEstabelecimento = "Via do Estabelecimento"                                     // 3.1.8
	textoViaConsumidor      = "Via consumidor"                                             // figura 6
)

// OpcoesNFCe ajusta a impressão do cupom.
type OpcoesNFCe struct {
	// ViaEstabelecimento imprime a 2ª via da NFC-e em contingência
	// ("Via do Estabelecimento" ao lado da data de emissão, manual 3.1.8).
	// Sem efeito fora de contingência.
	ViaEstabelecimento bool
	// Logo (opcional) vai no alto da divisão I.
	Logo *Logo
}

// GerarDANFENFCe gera o cupom fiscal digital 80mm para NFC-e (mod=65).
// O XML de entrada pode ser <NFe> simples ou <nfeProc> com protocolo.
func GerarDANFENFCe(xmlNFeProc []byte) ([]byte, error) {
	return GerarDANFENFCeComOpcoes(xmlNFeProc, OpcoesNFCe{})
}

// GerarDANFENFCeComOpcoes é GerarDANFENFCe com 2ª via/logo.
func GerarDANFENFCeComOpcoes(xmlNFeProc []byte, op OpcoesNFCe) ([]byte, error) {
	dados, err := ParseNFeXML(xmlNFeProc)
	if err != nil {
		return nil, fmt.Errorf("danfe: nfce: %w", err)
	}
	return renderizarCupom(dados, op)
}

func renderizarCupom(d *DadosDANFE, op OpcoesNFCe) ([]byte, error) {
	// Primeira passada numa folha alta só para medir; a segunda já sai com a
	// altura exata (rolo de bobina: sem folha em branco no fim).
	_, alturaUsada, err := desenharCupom(d, op, 3000)
	if err != nil {
		return nil, err
	}
	out, _, err := desenharCupom(d, op, alturaUsada+cupomMarg)
	return out, err
}

func desenharCupom(d *DadosDANFE, op OpcoesNFCe, altura float64) ([]byte, float64, error) {
	pdf := novoDoc(fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: cupomLarg, Ht: altura},
	}))
	pdf.SetMargins(cupomMarg, cupomMarg, cupomMarg)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	contingencia := d.TpEmis == "9"
	y := cupomMarg
	y = cupomCabecalho(pdf, d, op, y)
	if contingencia {
		y = cupomBlocoContingencia(pdf, y)
	}
	y = cupomSeparador(pdf, y)
	y = cupomItens(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomTotais(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomConsultaChave(pdf, d, y)
	y = cupomSeparador(pdf, y)
	y = cupomConsumidor(pdf, d, y)
	y = cupomIdentificacao(pdf, d, op, contingencia, y)
	y = cupomMensagemFiscal(pdf, d, contingencia, y)
	y = cupomQRCode(pdf, d, y)
	y = cupomInfCpl(pdf, d, y)

	if pdf.Err() {
		return nil, 0, fmt.Errorf("danfe: nfce: fpdf: %s", pdf.Error())
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, 0, fmt.Errorf("danfe: nfce: output: %w", err)
	}
	return buf.Bytes(), y, nil
}

// cupomTexto escreve um parágrafo (quebra de linha automática) na largura útil
// e devolve o y logo abaixo.
func cupomTexto(pdf *Doc, y float64, estilo string, tam float64, alinh, txt string) float64 {
	pdf.SetFont("Arial", estilo, tam)
	pdf.SetXY(cupomMarg, y)
	pdf.MultiCell(cupomLW, tam*0.42, txt, "", alinh, false)
	return pdf.GetY()
}

// cupomPar escreve rótulo à esquerda e valor à direita na mesma linha.
func cupomPar(pdf *Doc, y float64, estilo string, tam float64, rotulo, valor string) float64 {
	h := tam * 0.45
	pdf.SetFont("Arial", estilo, tam)
	pdf.SetXY(cupomMarg, y)
	wV := cupomLW * 0.40
	pdf.CellFormat(cupomLW-wV, h, rotulo, "", 0, "L", false, 0, "")
	pdf.CellFormat(wV, h, valor, "", 0, "R", false, 0, "")
	return y + h
}

func cupomSeparador(pdf *Doc, y float64) float64 {
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.2)
	pdf.Line(cupomMarg, y+0.8, cupomMarg+cupomLW, y+0.8)
	return y + 1.8
}

// ── Divisão I — cabeçalho ─────────────────────────────────────────────────────

func cupomCabecalho(pdf *Doc, d *DadosDANFE, op OpcoesNFCe, y float64) float64 {
	if op.Logo != nil {
		if w, h := ajustarNaCaixa(op.Logo, 22, 12); w > 0 {
			nome := "nfce_logo"
			pdf.RegisterImageOptionsReader(nome, fpdf.ImageOptions{ImageType: op.Logo.Tipo}, bytes.NewReader(op.Logo.Dados))
			pdf.Image(nome, cupomMarg+(cupomLW-w)/2, y, w, h, false, "", 0, "")
			y += h + 1
		}
	}

	y = cupomTexto(pdf, y, "B", 8, "C", d.EmitNome)

	switch {
	case d.EmitCNPJ != "":
		y = cupomTexto(pdf, y, "", 7, "C", "CNPJ: "+d.EmitCNPJ)
	case d.EmitCPF != "":
		y = cupomTexto(pdf, y, "", 7, "C", "CPF: "+d.EmitCPF)
	}
	if end := enderecoCupom(d.EmitEnd); end != "" {
		y = cupomTexto(pdf, y, "", 7, "C", end)
	}
	return cupomTexto(pdf, y+0.5, "B", 7, "C", textoTituloNFCe)
}

// enderecoCupom monta o endereço completo, sem país (manual 3.1.1).
func enderecoCupom(e enderecoDANFE) string {
	var partes []string
	for _, p := range []string{e.Logradouro, e.Numero, e.Complemento, e.Bairro} {
		if p != "" {
			partes = append(partes, p)
		}
	}
	mun := e.Municipio
	if e.UF != "" {
		if mun != "" {
			mun += " - "
		}
		mun += e.UF
	}
	if mun != "" {
		partes = append(partes, mun)
	}
	if e.CEP != "" {
		partes = append(partes, "CEP "+e.CEP)
	}
	return strings.Join(partes, ", ")
}

// cupomBlocoContingencia: "EMITIDA EM CONTINGÊNCIA / Pendente de autorização",
// centralizado em duas linhas, em destaque (manual 3.1.8).
func cupomBlocoContingencia(pdf *Doc, y float64) float64 {
	y = cupomSeparador(pdf, y)
	y = cupomTexto(pdf, y+0.5, "B", 10, "C", textoContingencia1)
	y = cupomTexto(pdf, y, "B", 7, "C", textoContingencia2)
	return y + 0.5
}

// ── Divisão II — itens ────────────────────────────────────────────────────────

func cupomItens(pdf *Doc, d *DadosDANFE, y float64) float64 {
	const wQtd, wUn = 26.0, 22.0
	cabecalho := func(y float64) float64 {
		pdf.SetFont("Arial", "B", 6.5)
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(cupomLW, 3, "CÓDIGO  DESCRIÇÃO", "", 2, "L", false, 0, "")
		pdf.SetX(cupomMarg)
		pdf.CellFormat(wQtd, 3, "QTDE UN", "", 0, "L", false, 0, "")
		pdf.CellFormat(wUn, 3, "VL. UNIT.", "", 0, "R", false, 0, "")
		pdf.CellFormat(cupomLW-wQtd-wUn, 3, "VL. TOTAL", "", 1, "R", false, 0, "")
		return pdf.GetY() + 0.3
	}
	y = cabecalho(y)

	for _, it := range d.Itens {
		y = cupomTexto(pdf, y, "", 7, "L", strings.TrimSpace(it.CProd+"  "+it.XProd))
		pdf.SetFont("Arial", "", 7)
		pdf.SetXY(cupomMarg, y)
		pdf.CellFormat(wQtd, 3.1, strings.TrimSpace(formatarDec(it.Qtd, 0, 4)+" "+it.Unidade), "", 0, "L", false, 0, "")
		pdf.CellFormat(wUn, 3.1, formatarDec(it.VUnit, 2, 4), "", 0, "R", false, 0, "")
		pdf.CellFormat(cupomLW-wQtd-wUn, 3.1, formatarMoeda(it.VProd), "", 1, "R", false, 0, "")
		y = pdf.GetY() + 0.4
	}
	return y
}

// ── Divisão III — totais e pagamento ──────────────────────────────────────────

func cupomTotais(pdf *Doc, d *DadosDANFE, y float64) float64 {
	acrescimos := d.VFrete + d.VSeg + d.VOutro
	y = cupomPar(pdf, y, "", 7.5, "Qtde. total de itens", strconv.Itoa(len(d.Itens)))
	y = cupomPar(pdf, y, "", 7.5, "Valor total R$", formatarMoeda(d.VProd))
	if acrescimos > 0 {
		y = cupomPar(pdf, y, "", 7.5, "Acréscimos R$", formatarMoeda(acrescimos))
	}
	if d.VDesc > 0 {
		y = cupomPar(pdf, y, "", 7.5, "Descontos R$", formatarMoeda(d.VDesc))
	}
	// "Valor a Pagar" só quando há acréscimo ou desconto (manual 3.1.3).
	if acrescimos > 0 || d.VDesc > 0 {
		y = cupomPar(pdf, y, "B", 8.5, "Valor a Pagar R$", formatarMoeda(d.VNF))
	}

	if len(d.Pagamentos) > 0 {
		y = cupomPar(pdf, y+0.8, "B", 6.5, "FORMA DE PAGAMENTO", "VALOR PAGO R$")
		for _, p := range d.Pagamentos {
			y = cupomPar(pdf, y, "", 7.5, p.Forma, formatarMoeda(p.Valor))
		}
	}
	if d.VTroco > 0 {
		y = cupomPar(pdf, y, "", 7.5, "Troco R$", formatarMoeda(d.VTroco))
	}
	return y
}

// ── Divisão IV — consulta pela chave ──────────────────────────────────────────

func cupomConsultaChave(pdf *Doc, d *DadosDANFE, y float64) float64 {
	y = cupomTexto(pdf, y+0.3, "B", 7, "C", textoConsultaChave)
	if d.UrlChave != "" {
		y = cupomTexto(pdf, y, "", 6.5, "C", d.UrlChave)
	}
	if d.ChaveAcesso != "" {
		y = cupomTexto(pdf, y+0.3, "", 7, "C", formatarChave(d.ChaveAcesso))
	}
	return y
}

// ── Divisão VI — consumidor ───────────────────────────────────────────────────

func cupomConsumidor(pdf *Doc, d *DadosDANFE, y float64) float64 {
	var id string
	switch {
	case d.DestCNPJ != "":
		id = "CONSUMIDOR CNPJ: " + d.DestCNPJ
	case d.DestCPF != "":
		id = "CONSUMIDOR CPF: " + d.DestCPF
	case d.DestIdEstrangeiro != "":
		id = "CONSUMIDOR Id. Estrangeiro: " + d.DestIdEstrangeiro
	}
	if id == "" {
		return cupomTexto(pdf, y+0.3, "B", 7.5, "C", textoNaoIdentificado) + 0.5
	}
	y = cupomTexto(pdf, y+0.3, "B", 7.5, "C", id)
	if d.DestNome != "" {
		y = cupomTexto(pdf, y, "", 7, "C", d.DestNome)
	}
	if end := enderecoCupom(d.DestEnd); end != "" {
		y = cupomTexto(pdf, y, "", 7, "C", end)
	}
	return y + 0.5
}

// ── Divisão VII — identificação e protocolo ───────────────────────────────────

func cupomIdentificacao(pdf *Doc, d *DadosDANFE, op OpcoesNFCe, contingencia bool, y float64) float64 {
	linha := fmt.Sprintf("NFC-e nº %s  Série %s  %s", d.NumeroNota, d.Serie, d.DhEmiCompleta)
	y = cupomTexto(pdf, y, "B", 7, "C", linha)
	if contingencia {
		via := textoViaConsumidor
		if op.ViaEstabelecimento {
			via = textoViaEstabelecimento
		}
		y = cupomTexto(pdf, y, "B", 7, "C", via)
	}
	// Em contingência o protocolo é suprimido (manual 3.1.7).
	if !contingencia && d.NumProtocolo != "" {
		y = cupomTexto(pdf, y, "B", 7, "C", textoProtocolo+" "+d.NumProtocolo)
		if d.DhRecbtoCompleta != "" {
			y = cupomTexto(pdf, y, "", 7, "C", "Data de autorização: "+d.DhRecbtoCompleta)
		}
	}
	return y + 0.3
}

// ── Divisão VIII — mensagem fiscal ────────────────────────────────────────────

func cupomMensagemFiscal(pdf *Doc, d *DadosDANFE, contingencia bool, y float64) float64 {
	if d.InfAdFisco != "" {
		y = cupomTexto(pdf, y+0.3, "", 6.5, "C", d.InfAdFisco)
	}
	if contingencia {
		y = cupomTexto(pdf, y+0.5, "B", 10, "C", textoContingencia1)
		y = cupomTexto(pdf, y, "B", 7, "C", textoContingencia2)
	}
	if d.TpAmb == "2" {
		y = cupomTexto(pdf, y+0.5, "B", 7.5, "C", textoHomologacao)
	}
	return y + 0.3
}

// ── Divisão V — QR Code ───────────────────────────────────────────────────────

func cupomQRCode(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if d.QrCode == "" {
		return y
	}
	// Nível M de correção de erros (manual 4.5.2); o conteúdo vai em bytes
	// UTF-8 (4.5.3). A margem segura (quiet zone) vem do próprio encoder.
	qr, err := qrcode.New(d.QrCode, qrcode.Medium)
	if err != nil {
		return y
	}
	png, err := qr.PNG(512)
	if err != nil {
		return y
	}
	xQR := cupomMarg + (cupomLW-cupomQR)/2
	pdf.RegisterImageOptionsReader("nfce_qr", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))
	pdf.Image("nfce_qr", xQR, y+0.5, cupomQR, cupomQR, false, "", 0, "")
	return y + cupomQR + 1.5
}

// ── Divisão IX — informações complementares do contribuinte ───────────────────

func cupomInfCpl(pdf *Doc, d *DadosDANFE, y float64) float64 {
	if d.InfCpl == "" {
		return y
	}
	y = cupomSeparador(pdf, y)
	return cupomTexto(pdf, y, "", 6.5, "L", d.InfCpl) + 0.5
}

// formatarDec formata v com vírgula decimal e ponto de milhar, usando entre
// min e max casas (corta zeros à direita até min): 5.4 -> "5,40", 0.482 ->
// "0,482", 6 (min 0) -> "6".
func formatarDec(v float64, min, max int) string {
	s := strconv.FormatFloat(v, 'f', max, 64)
	int_, frac, _ := strings.Cut(s, ".")
	for len(frac) > min && strings.HasSuffix(frac, "0") {
		frac = frac[:len(frac)-1]
	}
	out := inserirPontos(strings.TrimPrefix(int_, "-"))
	if strings.HasPrefix(int_, "-") {
		out = "-" + out
	}
	if frac != "" {
		out += "," + frac
	}
	return out
}
