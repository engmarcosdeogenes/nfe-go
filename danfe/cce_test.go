package danfe_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/danfe"
	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

func procEventoCCeTeste(chave string) []byte {
	return []byte(`<procEventoNFe versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infEvento Id="ID110110` + chave + `02">` +
		`<cOrgao>52</cOrgao><tpAmb>2</tpAmb><CNPJ>` + chave[6:20] + `</CNPJ><chNFe>` + chave + `</chNFe>` +
		`<dhEvento>2026-11-16T10:15:00-03:00</dhEvento><tpEvento>110110</tpEvento><nSeqEvento>2</nSeqEvento>` +
		`<verEvento>1.00</verEvento><detEvento versao="1.00"><descEvento>Carta de Correcao</descEvento>` +
		`<xCorrecao>Corrige o endereco de entrega para Rua das Palmeiras 100</xCorrecao>` +
		`<xCondUso>` + sefaz.XCondUsoCartaCorrecao + `</xCondUso></detEvento></infEvento></evento>` +
		`<retEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infEvento><tpAmb>2</tpAmb>` +
		`<cStat>135</cStat><xMotivo>Evento registrado e vinculado a NF-e</xMotivo><chNFe>` + chave + `</chNFe>` +
		`<tpEvento>110110</tpEvento><nSeqEvento>2</nSeqEvento><dhRegEvento>2026-11-16T10:15:02-03:00</dhRegEvento>` +
		`<nProt>152260000000222</nProt></infEvento></retEvento></procEventoNFe>`)
}

func TestGerarCCe(t *testing.T) {
	nfe := nfeAssinadaParaTeste(t)
	dados, err := danfe.ParseNFeXML(nfe)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := danfe.GerarCCe(procEventoCCeTeste(dados.ChaveAcesso), nfe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) || len(pdf) < 1000 {
		t.Fatalf("PDF inválido (%d bytes)", len(pdf))
	}
	s := string(pdf)
	for _, want := range []string{
		"CARTA DE CORRE", "152260000000222", "Rua das Palmeiras 100", "Convenio S/N",
		dados.ChaveAcesso[:4], dados.EmitNome, "HOMOLOGA",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF sem %q", want)
		}
	}
	if os.Getenv("CCE_SALVAR") != "" {
		if err := os.WriteFile(os.Getenv("CCE_SALVAR"), pdf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGerarCCe_SemNota(t *testing.T) {
	pdf, err := danfe.GerarCCe(procEventoCCeTeste("52261134152609000106550010000001691000001690"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pdf), "34.152.609/0001-06") {
		t.Error("sem a nota, o CNPJ do autor do evento tem que aparecer")
	}
}

func TestGerarCCe_RecusaOutroEvento(t *testing.T) {
	cancel := bytes.ReplaceAll(procEventoCCeTeste("52261134152609000106550010000001691000001690"), []byte("110110"), []byte("110111"))
	if _, err := danfe.GerarCCe(cancel, nil); err == nil {
		t.Error("cancelamento não pode virar PDF de CC-e")
	}
}
