package cert_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
)

const cnpjTeste = "11222333000181"

func TestGerarECarregarCertificadoTeste(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "senha123")
	if err != nil {
		t.Fatalf("GerarCertificadoTeste: %v", err)
	}
	if len(pfx) == 0 {
		t.Fatal("PFX gerado vazio")
	}

	c, err := cert.CarregarPFXBytes(pfx, "senha123")
	if err != nil {
		t.Fatalf("CarregarPFXBytes: %v", err)
	}

	if c.Chave == nil {
		t.Fatal("chave privada nil")
	}
	if c.Cert == nil {
		t.Fatal("certificado nil")
	}
	if c.Cert.NotAfter.Before(time.Now()) {
		t.Fatal("certificado já expirado")
	}

	tlsCfg := c.TLSConfig()
	if len(tlsCfg.Certificates) != 1 {
		t.Fatal("TLSConfig sem certificado")
	}

	t.Logf("Certificado OK — CN: %s, válido até: %s", c.Cert.Subject.CommonName, c.Cert.NotAfter.Format("2006-01-02"))
}

func TestSenhaErrada(t *testing.T) {
	pfx, _ := cert.GerarCertificadoTeste(cnpjTeste, "correta")
	_, err := cert.CarregarPFXBytes(pfx, "errada")
	if err == nil {
		t.Fatal("esperava erro com senha errada")
	}
}

func TestCNPJ(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "teste")
	if err != nil {
		t.Fatal(err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatal(err)
	}
	cnpj := c.CNPJ()
	if cnpj != cnpjTeste {
		t.Errorf("CNPJ() = %q, esperava %q", cnpj, cnpjTeste)
	}
}

// TestCNPJ_FallbackParaCN cobre o caso real da AC Solução Digital Múltipla:
// SerialNumber vazio, CNPJ só embutido no CN como "RAZAO SOCIAL:CNPJ".
func TestCNPJ_FallbackParaCN(t *testing.T) {
	c := &cert.Certificado{
		Cert: &x509.Certificate{
			Subject: pkix.Name{
				CommonName: "ATUAR SISTEMAS E EQUIPAMENTOS CONTRA INCENDIO LTD:20666560000197",
			},
		},
	}
	if got := c.CNPJ(); got != "20666560000197" {
		t.Errorf("CNPJ() = %q, esperava %q", got, "20666560000197")
	}
}

func TestCNPJ_SemSerialNumberNemCNPJNoCN(t *testing.T) {
	c := &cert.Certificado{
		Cert: &x509.Certificate{
			Subject: pkix.Name{CommonName: "SEM CNPJ NENHUM"},
		},
	}
	if got := c.CNPJ(); got != "" {
		t.Errorf("CNPJ() = %q, esperava vazio", got)
	}
}

func TestValido_CertFresco(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "teste")
	if err != nil {
		t.Fatal(err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Valido() {
		t.Error("certificado recém-gerado deveria ser válido")
	}
	t.Logf("válido de %s até %s", c.Cert.NotBefore.Format("2006-01-02"), c.Cert.NotAfter.Format("2006-01-02"))
}

func TestCarregarPFX_Arquivo(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "arquivo123")
	if err != nil {
		t.Fatal(err)
	}

	// Salva em arquivo temporário e carrega pelo path
	f, err := os.CreateTemp("", "cert_teste_*.pfx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(pfx); err != nil {
		t.Fatal(err)
	}
	f.Close()

	c, err := cert.CarregarPFX(f.Name(), "arquivo123")
	if err != nil {
		t.Fatalf("CarregarPFX: %v", err)
	}
	if c.Chave == nil {
		t.Fatal("chave nil após CarregarPFX")
	}
}

func TestCarregarPFX_ArquivoInexistente(t *testing.T) {
	_, err := cert.CarregarPFX("/nao/existe/cert.pfx", "senha")
	if err == nil {
		t.Fatal("esperava erro para arquivo inexistente")
	}
}

func TestGerarPEMTeste(t *testing.T) {
	certPEM, keyPEM, err := cert.GerarPEMTeste(cnpjTeste)
	if err != nil {
		t.Fatalf("GerarPEMTeste: %v", err)
	}
	if !strings.Contains(certPEM, "BEGIN CERTIFICATE") {
		t.Error("certPEM não contém BEGIN CERTIFICATE")
	}
	if !strings.Contains(keyPEM, "BEGIN PRIVATE KEY") {
		t.Error("keyPEM não contém BEGIN PRIVATE KEY")
	}
}

// TestTLSConfigTemCifraRSA cobre bloqueio real de produção: nfe.sefaz.go.gov.br
// recusa todas as ECDHE e só fecha handshake com DHE (que o Go não implementa)
// ou RSA (que o Go desabilita por padrão). Sem essas cifras na lista, emitir em
// produção é impossível — e homologação não pega, porque negocia ECDHE.
func TestTLSConfigTemCifraRSA(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "senha123")
	if err != nil {
		t.Fatalf("gerar cert de teste: %v", err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "senha123")
	if err != nil {
		t.Fatalf("carregar cert de teste: %v", err)
	}
	cfg := c.TLSConfig()
	if len(cfg.CipherSuites) == 0 {
		t.Fatal("CipherSuites vazio: volta pro default do Go, que exclui as RSA")
	}

	var temRSA, temECDHE bool
	for _, cs := range cfg.CipherSuites {
		switch cs {
		case tls.TLS_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_RSA_WITH_AES_256_GCM_SHA384:
			temRSA = true
		case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
			temECDHE = true
		}
	}
	if !temRSA {
		t.Error("sem cifra RSA: SEFAZ-GO produção recusa o handshake")
	}
	if !temECDHE {
		t.Error("sem cifra ECDHE: perderia forward secrecy onde o servidor suporta")
	}
	if cfg.CipherSuites[0] != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
		t.Error("ECDHE tem que vir primeiro — RSA é fallback, não preferência")
	}
}

// TestTLSConfigConfiaICPBrasil cobre outro bloqueio real de produção:
// nfe.sefaz.go.gov.br manda só o cert folha (emitido por AC SOLUTI SSL EV
// G4, sob raiz ICP-Brasil v10), sem a cadeia intermediária — e nenhuma das
// duas está no pool padrão do Go (Mozilla/OS). Sem RootCAs explícito aqui,
// o handshake falha com "certificate signed by unknown authority" mesmo com
// mTLS do cliente correto.
func TestTLSConfigConfiaICPBrasil(t *testing.T) {
	pfx, err := cert.GerarCertificadoTeste(cnpjTeste, "senha123")
	if err != nil {
		t.Fatalf("gerar cert de teste: %v", err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "senha123")
	if err != nil {
		t.Fatalf("carregar cert de teste: %v", err)
	}
	cfg := c.TLSConfig()
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs vazio: volta pro pool padrão do Go, sem ICP-Brasil")
	}
	// Assunto exato da intermediária AC SOLUTI SSL EV G4 — subject fixo do
	// cert embutido em icpbrasil_root.pem, não muda entre execuções.
	alvo := "AC SOLUTI SSL EV G4"
	achou := false
	for _, sub := range cfg.RootCAs.Subjects() { //nolint:staticcheck // Subjects é deprecated mas suficiente pra esse assert
		if strings.Contains(string(sub), alvo) {
			achou = true
			break
		}
	}
	if !achou {
		t.Errorf("pool não contém %q — intermediária ICP-Brasil não foi embutida", alvo)
	}
}
