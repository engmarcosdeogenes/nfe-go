package cert_test

import (
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
