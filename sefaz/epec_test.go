package sefaz_test

import (
	"context"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

// TestObterURL_AN_RecepcaoEventoEPEC cobre o achado de que o EPEC é sempre
// recebido pelo Ambiente Nacional (cOrgao=91), não pela SEFAZ da UF nem por
// SVC-AN/SVC-RS (esses são pro modo de contingência tpEmis=6/7, diferente).
func TestObterURL_AN_RecepcaoEventoEPEC(t *testing.T) {
	prod := sefaz.ObterURL("91", sefaz.ServicoRecepcaoEventoAN, sefaz.Producao)
	if prod != "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx" {
		t.Errorf("URL produção = %q", prod)
	}
	homol := sefaz.ObterURL("91", sefaz.ServicoRecepcaoEventoAN, sefaz.Homologacao)
	if homol != "https://hom1.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx" {
		t.Errorf("URL homologação = %q", homol)
	}
}

func TestRegistrarEPEC_ChaveInvalida_Erro(t *testing.T) {
	_, err := sefaz.RegistrarEPEC(context.Background(), sefaz.EntradaEPEC{ChNFe: "123"}, sefaz.Homologacao, certTeste(t))
	if err == nil || !strings.Contains(err.Error(), "44 dígitos") {
		t.Fatalf("esperava erro de chNFe, veio: %v", err)
	}
}

func TestRegistrarEPEC_SemDestinatario_Erro(t *testing.T) {
	in := sefaz.EntradaEPEC{ChNFe: strings.Repeat("5", 44)}
	_, err := sefaz.RegistrarEPEC(context.Background(), in, sefaz.Homologacao, certTeste(t))
	if err == nil || !strings.Contains(err.Error(), "destinatário") {
		t.Fatalf("esperava erro de destinatário, veio: %v", err)
	}
}
