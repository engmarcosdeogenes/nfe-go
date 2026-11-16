package sefaz_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

const chaveFixture = "52261134152609000106550010000001691000001690"

// procEventoCCeFixture segue o leiaute procEventoNFe_v1.00 (MOC 7.0): evento assinado + retEvento,
// cada um com o próprio xmlns, como a SEFAZ devolve no retConsSitNFe.
func procEventoCCeFixture(seq, nProt, texto string) string {
	return `<procEventoNFe versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<evento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<infEvento Id="ID110110` + chaveFixture + `0` + seq + `">` +
		`<cOrgao>52</cOrgao><tpAmb>1</tpAmb><CNPJ>34152609000106</CNPJ><chNFe>` + chaveFixture + `</chNFe>` +
		`<dhEvento>2026-11-16T10:15:00-03:00</dhEvento><tpEvento>110110</tpEvento><nSeqEvento>` + seq + `</nSeqEvento>` +
		`<verEvento>1.00</verEvento><detEvento versao="1.00"><descEvento>Carta de Correcao</descEvento>` +
		`<xCorrecao>` + texto + `</xCorrecao><xCondUso>` + sefaz.XCondUsoCartaCorrecao + `</xCondUso></detEvento></infEvento>` +
		`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignedInfo/><SignatureValue>AA==</SignatureValue></Signature>` +
		`</evento>` +
		`<retEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infEvento>` +
		`<tpAmb>1</tpAmb><verAplic>GO4.0</verAplic><cOrgao>52</cOrgao><cStat>135</cStat>` +
		`<xMotivo>Evento registrado e vinculado a NF-e</xMotivo><chNFe>` + chaveFixture + `</chNFe>` +
		`<tpEvento>110110</tpEvento><xEvento>Carta de Correcao</xEvento><nSeqEvento>` + seq + `</nSeqEvento>` +
		`<dhRegEvento>2026-11-16T10:15:02-03:00</dhRegEvento><nProt>` + nProt + `</nProt></infEvento></retEvento>` +
		`</procEventoNFe>`
}

// retConsSitNFe de uma NF-e com duas CC-e (formato do NFeConsultaProtocolo4, MOC 7.0 §5.5).
func retConsSitComCCe() string {
	return `<retConsSitNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe">` +
		`<tpAmb>1</tpAmb><verAplic>GO4.0</verAplic><cStat>100</cStat><xMotivo>Autorizado o uso da NF-e</xMotivo>` +
		`<cUF>52</cUF><dhRecbto>2026-11-16T10:20:00-03:00</dhRecbto><chNFe>` + chaveFixture + `</chNFe>` +
		`<protNFe versao="4.00"><infProt><tpAmb>1</tpAmb><verAplic>GO4.0</verAplic><chNFe>` + chaveFixture + `</chNFe>` +
		`<dhRecbto>2026-11-16T09:00:00-03:00</dhRecbto><nProt>152260000000001</nProt><cStat>100</cStat>` +
		`<xMotivo>Autorizado o uso da NF-e</xMotivo></infProt></protNFe>` +
		procEventoCCeFixture("1", "152260000000111", "Corrige o endereco de entrega para Rua A") +
		procEventoCCeFixture("2", "152260000000222", "Corrige o endereco de entrega para Rua B") +
		`</retConsSitNFe>`
}

func TestExtrairProcEventos_DoRetConsSitNFe(t *testing.T) {
	procs, err := sefaz.ExtrairProcEventos([]byte(retConsSitComCCe()))
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 {
		t.Fatalf("esperava 2 procEventoNFe, veio %d", len(procs))
	}
	for i, want := range []struct{ seq, prot, texto string }{
		{"1", "152260000000111", "Rua A"},
		{"2", "152260000000222", "Rua B"},
	} {
		p, err := sefaz.LerProcEvento(procs[i])
		if err != nil {
			t.Fatal(err)
		}
		if p.TpEvento != "110110" || p.NSeqEvento != want.seq || p.NProt != want.prot || p.CStat != "135" ||
			!strings.Contains(p.XCorrecao, want.texto) || p.XCondUso != sefaz.XCondUsoCartaCorrecao || p.ChNFe != chaveFixture {
			t.Errorf("proc %d lido errado: %+v", i, p)
		}
		// O pedaço solto tem que ser XML bem formado e raiz procEventoNFe.
		var raiz struct{ XMLName xml.Name }
		if err := xml.Unmarshal(procs[i], &raiz); err != nil || raiz.XMLName.Local != "procEventoNFe" ||
			raiz.XMLName.Space != "http://www.portalfiscal.inf.br/nfe" {
			t.Errorf("proc %d: raiz %v err %v", i, raiz.XMLName, err)
		}
	}
}

func TestConsultarProtocolo_DevolveProcEventos(t *testing.T) {
	mock := &cadastroTransport{resposta: envelopeConsCad(retConsSitComCCe())}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Producao, mock)
	ret, err := cl.ConsultarProtocolo(context.Background(), chaveFixture)
	if err != nil {
		t.Fatal(err)
	}
	if ret.CStat != "100" || ret.NProt != "152260000000001" {
		t.Errorf("consulta: %+v", ret.RetornoSEFAZ)
	}
	if len(ret.ProcEventos) != 2 {
		t.Fatalf("ProcEventos: %d", len(ret.ProcEventos))
	}
	if !bytes.Contains(ret.ProcEventos[1], []byte("<nProt>152260000000222</nProt>")) {
		t.Errorf("segundo procEvento errado: %s", ret.ProcEventos[1])
	}
}

func TestConsultarProtocolo_SemEventos(t *testing.T) {
	semEv := strings.Replace(retConsSitComCCe(), procEventoCCeFixture("1", "152260000000111", "Corrige o endereco de entrega para Rua A"), "", 1)
	semEv = strings.Replace(semEv, procEventoCCeFixture("2", "152260000000222", "Corrige o endereco de entrega para Rua B"), "", 1)
	mock := &cadastroTransport{resposta: envelopeConsCad(semEv)}
	ret, err := sefaz.NovoClienteTransporte("52", sefaz.Producao, mock).ConsultarProtocolo(context.Background(), chaveFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(ret.ProcEventos) != 0 {
		t.Errorf("esperava nenhum evento, veio %d", len(ret.ProcEventos))
	}
}

// O retEvento vem sem xmlns próprio dentro do retEnvEvento (herda do pai). No procEventoNFe montado
// ele tem que ganhar o xmlns, senão sai do namespace da NF-e.
func TestCancelar_DevolveProcEventoNFe(t *testing.T) {
	ret := `<retEnvEvento versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><idLote>1</idLote>` +
		`<tpAmb>2</tpAmb><verAplic>GO4.0</verAplic><cOrgao>52</cOrgao><cStat>128</cStat><xMotivo>Lote de Evento Processado</xMotivo>` +
		`<retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><verAplic>GO4.0</verAplic><cOrgao>52</cOrgao>` +
		`<cStat>135</cStat><xMotivo>Evento registrado e vinculado a NF-e</xMotivo><chNFe>` + chaveFixture + `</chNFe>` +
		`<tpEvento>110111</tpEvento><xEvento>Cancelamento</xEvento><nSeqEvento>1</nSeqEvento>` +
		`<dhRegEvento>2026-11-16T11:00:00-03:00</dhRegEvento><nProt>152260000000999</nProt></infEvento></retEvento>` +
		`</retEnvEvento>`
	mock := &cadastroTransport{resposta: envelopeConsCad(ret)}
	cl := sefaz.NovoClienteTransporte("52", sefaz.Homologacao, mock)
	r, err := cl.Cancelar(context.Background(), chaveFixture, "152260000000001", "Cancelamento por erro de digitacao", certTeste(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.CStatEvento != "135" || len(r.XMLProcEvento) == 0 {
		t.Fatalf("retorno: %+v", r)
	}
	x := string(r.XMLProcEvento)
	if !strings.HasPrefix(x, `<procEventoNFe versao="1.00" xmlns="http://www.portalfiscal.inf.br/nfe"><evento `) ||
		!strings.Contains(x, `<retEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00">`) ||
		!strings.Contains(x, "<Signature") {
		t.Errorf("procEventoNFe mal montado: %s", x)
	}
	p, err := sefaz.LerProcEvento(r.XMLProcEvento)
	if err != nil {
		t.Fatal(err)
	}
	if p.TpEvento != "110111" || p.NProt != "152260000000999" || p.XJust != "Cancelamento por erro de digitacao" {
		t.Errorf("procEvento lido: %+v", p)
	}
}
