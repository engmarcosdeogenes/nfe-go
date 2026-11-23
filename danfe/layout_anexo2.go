package danfe

// Geometria e tipografia do DANFE A-4 em modo retrato, transcritas do
// MOC 7.0 — Anexo II, "Manual de Especificações Técnicas do DANFE e Código
// de Barras" (ENCAT, outubro/2020). O manual trabalha em centímetros com a
// origem no canto superior esquerdo da folha; aqui está tudo em milímetros,
// que é a unidade usada no fpdf.
//
// Antes dessa transcrição o layout era calibrado de olho, comparando com
// DANFE de concorrente. Estes números não são preferência visual: são o
// leiaute sugerido do item 3.8.1, e as fontes são pisos normativos do
// item 3.7.

const (
	larguraPage = 210.0
	alturaPage  = 297.0

	// §3.6.2 — a margem entre o corpo impresso e o fim do formulário tem que
	// ficar entre 0,2cm e 0,8cm em cada lateral, topo e base inclusive. O
	// item 3.8.1 ancora o corpo em 0,25cm da esquerda, com 20,57cm de largura.
	margem      = 2.5
	larguraUtil = 205.7

	// Alturas do item 3.8.1: "0,42" para o descritivo de bloco (a tarja com o
	// nome do quadro) e "0,85" para linha de campo.
	altTitulo = 4.2
	altCampo  = 8.5

	// Âncoras verticais do item 3.8.1.
	yCanhoto = 4.2  // primeira linha do canhoto
	yCorpo   = 25.4 // topo do quadro "IDENTIFICAÇÃO DO EMITENTE"

	altCabecalho = 39.2 // quadros emitente / "DANFE" / código de barras
	altQuadroCB  = 14.8 // quadro do código de barras da chave
	altFatura    = 12.7 // quadro "Fatura/Duplicatas"
	altDadosAdic = 30.7 // "Informações Complementares" + "Reservado ao Fisco"
	altProdutos  = 67.7 // quadro "Dados dos Produtos/Serviços" (elástico)
)

// Tamanhos mínimos de fonte do item 3.7, em pontos. São piso, não sugestão —
// o layout anterior desenhava descritivo de campo em 5,5pt (mínimo 6) e
// conteúdo em 8,5pt (mínimo 10), o que além de não conformar era boa parte
// da sensação de "apertado" do documento.
const (
	fonteDescBloco = 6.0  // §3.7.1 — descritivo de bloco, negrito, caixa alta (mín. 5)
	fonteDescItens = 5.0  // §3.7.2 — descritivo das colunas de produtos (mín. 5)
	fonteDescCampo = 6.0  // §3.7.3 — descritivo dos demais campos (mín. 6)
	fonteDANFE     = 12.0 // §3.7.4 — a palavra "DANFE", negrito (mín. 12)
	fonteNumSerie  = 10.0 // §3.7.4 — número, série, folha e tipo de operação, negrito (mín. 10)
	fonteDocAux    = 8.0  // §3.7.4 — "DOCUMENTO AUXILIAR DA NOTA FISCAL ELETRÔNICA" (mín. 8)
	fonteEmitNome  = 12.0 // §3.7.6 — razão social / fantasia do emitente, negrito (mín. 12)
	fonteEmitDados = 8.0  // §3.7.6 — endereço, município, CEP, fone do emitente (mín. 8)
	fonteItens     = 6.0  // §3.7.7 — conteúdo das colunas de produtos (mín. 6)
	fonteInfCpl    = 6.0  // §3.7.8 — informações complementares (mín. 6)
	fonteCampo     = 10.0 // §3.7.9 — conteúdo dos demais campos (mín. 10)
)

// Larguras de campo por linha, na ordem da esquerda para a direita, exatamente
// como o item 3.8.1 as lista. Cada linha soma larguraUtil.
var (
	larguraEmitIE    = []float64{68.6, 68.6, 68.5}                   // IE | IE ST | CNPJ
	larguraDestL1    = []float64{123.2, 53.3, 29.2}                  // razão social | CNPJ | data emissão
	larguraDestL2    = []float64{101.6, 48.3, 26.7, 29.1}            // endereço | bairro | CEP | data entrada/saída
	larguraDestL3    = []float64{71.1, 40.6, 11.4, 53.3, 29.3}       // município | fone | UF | IE | hora entrada/saída
	larguraImpostoL1 = []float64{40.6, 40.6, 40.6, 40.6, 43.3}       // BC ICMS | ICMS | BC ST | ICMS ST | total produtos
	larguraImpostoL2 = []float64{33.0, 33.0, 33.0, 33.0, 33.0, 40.7} // frete | seguro | desconto | outras | IPI | total nota
	larguraTranspL1  = []float64{90.2, 27.9, 17.8, 22.9, 7.6, 39.3}  // razão social | frete por conta | ANTT | placa | UF | CNPJ
	larguraTranspL2  = []float64{90.2, 68.6, 7.6, 39.3}              // endereço | município | UF | IE
	larguraTranspL3  = []float64{29.2, 30.5, 30.5, 48.3, 34.3, 32.9} // qtde | espécie | marca | numeração | peso bruto | peso líquido
	larguraAdic      = []float64{129.5, 76.2}                        // informações complementares | reservado ao fisco
)

// Leiaute da NT 2026.010 (DANFE da reforma tributária), usado nas notas com
// dhEmi a partir de 01/12/2026. A NT não traz tabela de coordenadas como o
// Anexo II: as larguras são as do modelo retrato da pág. 8, lidas dos
// retângulos vetoriais do PDF oficial (o modelo tem 19,50cm de corpo) e
// escaladas para larguraUtil. Cada linha soma larguraUtil. As alturas e os
// pisos de fonte continuam os do Anexo II (§3.7), que a NT não altera — o
// modelo desenha os itens em 5pt, abaixo do piso de 6pt do §3.7.7.
var (
	larguraEmitCRT   = []float64{100.4, 105.3}                 // §4.2: CRT | tipo de regime de apuração do IBS/CBS
	larguraTotProd   = []float64{46.0, 40.6, 40.9, 37.4, 40.8} // total produtos | frete | seguro | desconto | outras despesas
	larguraTotICMS   = []float64{46.0, 40.6, 40.5, 37.9, 40.7} // BC ICMS | ICMS | BC ST | ICMS ST | IPI
	larguraTotICMSFc = []float64{51.3, 45.9, 51.5, 57.0}       // §4.4: FCP | FCP ST | DIFAL dest. | FCP dest. — e a linha do monofásico
	larguraRTCL1     = []float64{55.4, 50.1, 51.4, 48.8}       // §4.1: CBS | IBS UF | IBS município | IS
	larguraRTCMono   = []float64{51.3, 35.6, 67.2, 51.6}       // §4.1/§4.4: IBS mono | CBS mono | IBS mono retenção | CBS mono retenção
)

// colunasItensRTC: as oito colunas do modelo da pág. 8, na ordem dele. Não
// há coluna de código nem de NCM: o NCM e o cClassTrib vão na última linha
// da descrição, como no modelo.
var colunasItensRTC = []struct {
	label string
	w     float64
	align string
}{
	{"DESCRIÇÃO DO PRODUTO / SERVIÇO", 43.6, "L"},
	{"CST / CFOP", 12.8, "L"},
	{"QTD / UN", 15.5, "C"},
	{"VLR UNIT", 18.0, "R"},
	{"VLR TOTAL", 17.9, "R"},
	{"BASES DE CÁLCULO", 28.4, "L"},
	{"ALÍQUOTAS", 28.3, "L"},
	{"VALOR DOS TRIBUTOS", 41.2, "L"},
}

// Colunas do quadro "Dados dos Produtos/Serviços". A ordem é a do item 3.8.1
// (Obs 4: "Colunas apresentadas na ordem descrita"). O item 3.1.7 lista as
// colunas que NÃO podem ser suprimidas — código, descrição, NCM, CST, CFOP,
// unidade, quantidade, valor unitário, valor total, BC do ICMS próprio, valor
// do ICMS próprio e alíquota do ICMS — todas presentes aqui. Desconto, BC do
// ICMS ST e valor do ICMS ST são suprimíveis e ficaram de fora; IPI e alíquota
// de IPI também seriam, mas são úteis pro perfil industrial que consome a lib.
var colunasItens = []struct {
	label string
	w     float64
	align string
}{
	{"CÓDIGO", 17.0, "C"},
	{"DESCRIÇÃO DO PRODUTO / SERVIÇO", 54.7, "L"},
	{"NCM/SH", 13.0, "C"},
	{"CST", 10.0, "C"},
	{"CFOP", 9.0, "C"},
	{"UNID.", 7.0, "C"},
	{"QUANT.", 11.0, "R"},
	{"VALOR UNITÁRIO", 13.0, "R"},
	{"VALOR TOTAL", 13.0, "R"},
	{"B.CÁLC.ICMS", 13.0, "R"},
	{"VALOR ICMS", 13.0, "R"},
	{"VALOR IPI", 12.0, "R"},
	{"ALÍQ. ICMS", 10.0, "R"},
	{"ALÍQ. IPI", 10.0, "R"},
}
