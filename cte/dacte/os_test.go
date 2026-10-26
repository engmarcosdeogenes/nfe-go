package dacte

import (
	"bytes"
	"os"
	"testing"
)

func gerarOS(t *testing.T, arq string, op Opcoes) []byte {
	t.Helper()
	compressao = false
	t.Cleanup(func() { compressao = true })
	pdf, err := GerarOS(ler(t, arq), op)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("não é PDF")
	}
	return pdf
}

func TestOS_PassageirosFretamento(t *testing.T) {
	pdf := gerarOS(t, "cteos_passageiros_proc.xml", Opcoes{})
	if n := len(rePag.FindAll(pdf, -1)); n != 1 {
		t.Errorf("páginas = %d, quer 1", n)
	}
	tem(t, pdf, "DACTE OS", "Modelo 67", "6 - Transporte de pessoas", `Quantidade \(passageiros\)`, "40,0000",
		"Empresa Tomadora", "44.555.666/0001-77", "2.500,00", "90 - Simples Nacional",
		"123456789012", "1 - Eventual", "05/11/2026 08:00:00", "ABC1D23", "Seguradora X", "AP-1",
		"926260000000002", "Fretamento eventual conforme contrato 77/2026", "Folha 1/1")
	if bytes.Contains(pdf, []byte("CANCELADO")) || bytes.Contains(pdf, []byte("HOMOLOG")) {
		t.Error("marca indevida")
	}
	if p := os.Getenv("DACTEOS_PDF"); p != "" { // amostra para conferência visual
		compressao = true
		b, err := GerarOS(ler(t, "cteos_passageiros_proc.xml"), Opcoes{})
		if err != nil || os.WriteFile(p, b, 0o644) != nil {
			t.Fatal(err)
		}
	}
}

func TestOS_ValoresPreviaHomologacao(t *testing.T) {
	pdf := gerarOS(t, "cteos_valores_previa.xml", Opcoes{})
	tem(t, pdf, "7 - Transporte de valores", `Quantidade \(volumes/malotes\)`, "SEM VALOR FISCAL - PR", "AMBIENTE DE HOMOLOG",
		"PIS retido", "16,25", "COFINS retido", "CSLL retido", "GTV-e relacionadas", "Cust\xf3dia", "Outros", "Cobran\xe7a", "F-1", "IBS/CBS - CST", "00 ")
}

func TestOS_BagagemSemTomadorECancelado(t *testing.T) {
	pdf := gerarOS(t, "cteos_bagagem_proc.xml", Opcoes{Cancelado: true})
	tem(t, pdf, "8 - Excesso de bagagem", "CANCELADO", "Excesso de bagagem")
	if bytes.Contains(pdf, []byte("TOMADOR DO SERVI")) {
		t.Error("quadro do tomador sem toma")
	}
}

func TestOS_Rejeicoes(t *testing.T) {
	for nome, x := range map[string]string{
		"lixo":        "<<<",
		"raiz errada": "<CTe xmlns=\"http://www.portalfiscal.inf.br/cte\"/>",
		"sem chave":   "<CTeOS xmlns=\"http://www.portalfiscal.inf.br/cte\"><infCte Id=\"x\"/></CTeOS>",
		"modelo 57":   "<CTeOS xmlns=\"http://www.portalfiscal.inf.br/cte\"><infCte Id=\"CTe52261011222333000181570010000000011123456780\"><ide><mod>57</mod></ide></infCte></CTeOS>",
	} {
		if _, err := GerarOS([]byte(x), Opcoes{}); err == nil {
			t.Errorf("%s: deveria dar erro", nome)
		}
	}
	proc := bytes.Replace(ler(t, "cteos_passageiros_proc.xml"), []byte("<cStat>100</cStat>"), []byte("<cStat>110</cStat>"), 1)
	if _, err := GerarOS(proc, Opcoes{}); err == nil {
		t.Error("cStat 110 deveria dar erro")
	}
	if _, err := Gerar(ler(t, "cteos_bagagem_proc.xml"), Opcoes{}); err == nil {
		t.Error("Gerar (57) deve recusar CT-e OS")
	}
}
