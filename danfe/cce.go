package danfe

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

// GerarCCe desenha a Carta de Correção Eletrônica (evento 110110) em A4 simples, a partir do
// procEventoNFe da CC-e e, se tiver, do XML da nota (nfeProc) para emitente, destinatário e
// número/série. Sem nfeXML sai só o que o evento traz (CNPJ do autor, chave). Não existe leiaute
// oficial de impressão da CC-e: o documento fiscal é o XML; este PDF é comprovante de leitura.
func GerarCCe(procEventoXML, nfeXML []byte) ([]byte, error) {
	ev, err := sefaz.LerProcEvento(procEventoXML)
	if err != nil {
		return nil, fmt.Errorf("danfe: CC-e: %w", err)
	}
	if ev.TpEvento != "110110" {
		return nil, fmt.Errorf("danfe: CC-e: evento %s não é carta de correção", ev.TpEvento)
	}
	var nota *DadosDANFE
	if len(nfeXML) > 0 {
		if nota, err = ParseNFeXML(nfeXML); err != nil {
			return nil, fmt.Errorf("danfe: CC-e: ler nota: %w", err)
		}
	}

	f := fpdf.New("P", "mm", "A4", "")
	f.SetCompression(false) // documento pequeno; texto legível no PDF facilita conferir e testar
	f.SetMargins(15, 15, 15)
	f.SetAutoPageBreak(true, 15)
	f.AddPage()
	pdf := novoDoc(f)
	larg := 180.0

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(larg, 9, "CARTA DE CORREÇÃO ELETRÔNICA", "", 1, "C", false, 0, "")
	if ev.TpAmb == "2" {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(larg, 5, "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL", "", 1, "C", false, 0, "")
	}
	pdf.Ln(3)

	campo := func(rotulo, valor string, w float64, ln int) {
		x, y := pdf.GetX(), pdf.GetY()
		pdf.SetFont("Helvetica", "", 7)
		pdf.CellFormat(w, 4, rotulo, "LTR", 2, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(w, 6, valor, "LBR", 0, "L", false, 0, "")
		if ln == 1 {
			pdf.SetXY(15, y+10)
		} else {
			pdf.SetXY(x+w, y)
		}
	}
	secao := func(t string) {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(larg, 6, t, "", 1, "L", false, 0, "")
	}

	emitNome, emitDoc := "", formatarCNPJ(ev.CNPJ)
	destNome, destDoc, numero, serie := "", "", "", ""
	if nota != nil {
		emitNome = nota.EmitNome
		if nota.EmitCNPJ != "" {
			emitDoc = nota.EmitCNPJ
		} else if nota.EmitCPF != "" {
			emitDoc = nota.EmitCPF
		}
		destNome = nota.DestNome
		destDoc = nota.DestCNPJ
		if destDoc == "" {
			destDoc = nota.DestCPF
		}
		if destDoc == "" {
			destDoc = nota.DestIdEstrangeiro
		}
		numero, serie = nota.NumeroNota, nota.Serie
	}

	secao("EMITENTE")
	campo("RAZÃO SOCIAL", emitNome, 130, 0)
	campo("CNPJ/CPF", emitDoc, 50, 1)
	secao("DESTINATÁRIO")
	campo("NOME / RAZÃO SOCIAL", destNome, 130, 0)
	campo("CNPJ/CPF", destDoc, 50, 1)

	secao("NOTA FISCAL ELETRÔNICA")
	campo("CHAVE DE ACESSO", formatarChave(ev.ChNFe), 130, 0)
	campo("NÚMERO", numero, 25, 0)
	campo("SÉRIE", serie, 25, 1)

	secao("EVENTO")
	campo("SEQUÊNCIA DO EVENTO", ev.NSeqEvento, 40, 0)
	campo("DATA/HORA DO EVENTO", formatarDataHoraSeg(ev.DhEvento), 45, 0)
	campo("PROTOCOLO", ev.NProt, 50, 0)
	campo("DATA/HORA DO REGISTRO", formatarDataHoraSeg(ev.DhRegEvento), 45, 1)

	secao("CORREÇÃO")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(larg, 5, ev.XCorrecao, "1", "L", false)

	secao("CONDIÇÕES DE USO")
	pdf.SetFont("Helvetica", "", 8)
	pdf.MultiCell(larg, 4, ev.XCondUso, "1", "J", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("danfe: CC-e: gerar PDF: %w", err)
	}
	return out.Bytes(), nil
}
