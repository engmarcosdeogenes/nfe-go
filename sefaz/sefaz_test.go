package sefaz_test

import (
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/sefaz"
)

func TestObterURL_GOHomologacao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para GO homologação")
	}
	if !strings.Contains(url, "homolog") && !strings.Contains(url, "homologacao") {
		t.Errorf("URL de homologação não parece correta: %s", url)
	}
	t.Logf("GO homologação autorizacao: %s", url)
}

func TestObterURL_GOProducao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoAutorizacao, sefaz.Producao)
	if url == "" {
		t.Fatal("URL vazia para GO produção")
	}
	if strings.Contains(url, "homolog") {
		t.Errorf("URL de produção contém 'homolog': %s", url)
	}
	t.Logf("GO produção autorizacao: %s", url)
}

func TestObterURL_SPHomologacao(t *testing.T) {
	url := sefaz.ObterURL("35", sefaz.ServicoConsultaProtocolo, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para SP homologação")
	}
	t.Logf("SP homologação consulta: %s", url)
}

func TestObterURL_FallbackSVRS(t *testing.T) {
	// AC (12) não tem entrada própria → deve usar SVRS
	url := sefaz.ObterURL("12", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para AC (SVRS fallback)")
	}
	if !strings.Contains(url, "svrs.rs.gov.br") {
		t.Errorf("AC deveria usar SVRS, mas URL é: %s", url)
	}
	t.Logf("AC (SVRS fallback) homologação: %s", url)
}

func TestObterURL_AMUsaSVAN(t *testing.T) {
	// AM (13) usa SVAN
	url := sefaz.ObterURL("13", sefaz.ServicoAutorizacao, sefaz.Homologacao)
	if url == "" {
		t.Fatal("URL vazia para AM")
	}
	if !strings.Contains(url, "sefazvirtual.fazenda.gov.br") {
		t.Errorf("AM deveria usar SVAN, mas URL é: %s", url)
	}
	t.Logf("AM (SVAN) homologação: %s", url)
}

func TestObterURL_TodosServicosGO(t *testing.T) {
	servicos := []sefaz.Servico{
		sefaz.ServicoAutorizacao,
		sefaz.ServicoRetAutorizacao,
		sefaz.ServicoConsultaProtocolo,
		sefaz.ServicoRecepcaoEvento,
		sefaz.ServicoInutilizacao,
		sefaz.ServicoStatusServico,
	}
	for _, srv := range servicos {
		url := sefaz.ObterURL("52", srv, sefaz.Homologacao)
		if url == "" {
			t.Errorf("URL vazia para GO / %s", srv)
		}
	}
}

func TestAmbienteString(t *testing.T) {
	if sefaz.Producao.String() != "Produção" {
		t.Errorf("Producao.String() = %q", sefaz.Producao.String())
	}
	if sefaz.Homologacao.String() != "Homologação" {
		t.Errorf("Homologacao.String() = %q", sefaz.Homologacao.String())
	}
}

// ── NFeDistribuicaoDFe ────────────────────────────────────────────────────────

func TestObterURL_DistribuicaoDFe_Nacional(t *testing.T) {
	// O serviço é nacional — cUF irrelevante, URL deve ser fazenda.gov.br
	for _, cuf := range []string{"52", "35", "12", "99"} {
		url := sefaz.ObterURL(cuf, sefaz.ServicoDistribuicaoDFe, sefaz.Homologacao)
		if url == "" {
			t.Fatalf("URL vazia para DistribuicaoDFe cuf=%s homologação", cuf)
		}
		if !strings.Contains(url, "nfe.fazenda.gov.br") {
			t.Errorf("cuf=%s: URL esperada conter 'nfe.fazenda.gov.br', got: %s", cuf, url)
		}
		if !strings.Contains(url, "hom") {
			t.Errorf("cuf=%s: URL de homologação deveria conter 'hom', got: %s", cuf, url)
		}
	}
}

func TestObterURL_DistribuicaoDFe_Producao(t *testing.T) {
	url := sefaz.ObterURL("52", sefaz.ServicoDistribuicaoDFe, sefaz.Producao)
	if url == "" {
		t.Fatal("URL vazia para DistribuicaoDFe produção")
	}
	if strings.Contains(url, "hom") {
		t.Errorf("URL de produção contém 'hom': %s", url)
	}
	if !strings.Contains(url, "nfe.fazenda.gov.br") {
		t.Errorf("URL de produção não contém 'nfe.fazenda.gov.br': %s", url)
	}
	t.Logf("DistribuicaoDFe produção: %s", url)
}

func TestRetornoDistribuicao_TemMais(t *testing.T) {
	casos := []struct {
		ultNSU string
		maxNSU string
		espera bool
	}{
		{"000000000000000", "000000000000050", true},
		{"000000000000050", "000000000000050", false},
		{"000000000000100", "000000000000050", false},
		{"", "000000000000050", false},
		{"000000000000010", "", false},
	}
	for _, c := range casos {
		r := sefaz.RetornoDistribuicao{UltNSU: c.ultNSU, MaxNSU: c.maxNSU}
		if got := r.TemMais(); got != c.espera {
			t.Errorf("TemMais() ultNSU=%q maxNSU=%q: got %v, want %v",
				c.ultNSU, c.maxNSU, got, c.espera)
		}
	}
}

func TestTipoDocDFeConstantes(t *testing.T) {
	if sefaz.DocProcNFe != "procNFe_v4.00.xsd" {
		t.Errorf("DocProcNFe = %q", sefaz.DocProcNFe)
	}
	if sefaz.DocResNFe != "resNFe_v1.01.xsd" {
		t.Errorf("DocResNFe = %q", sefaz.DocResNFe)
	}
}
