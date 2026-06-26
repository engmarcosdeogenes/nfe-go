// Package cert carrega certificados digitais A1 (.pfx / PKCS12)
// e prepara o tls.Config para comunicação mútua com a SEFAZ.
package cert

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Certificado reúne a chave privada e o certificado X.509 do emitente.
type Certificado struct {
	Chave   *rsa.PrivateKey
	Cert    *x509.Certificate
	CertDER []byte // bytes DER do certificado (usado na assinatura XML)
}

// CarregarPFX lê um arquivo .pfx e extrai a chave + certificado.
// O caminho é fornecido pelo operador do sistema — sem risco de traversal em uso normal.
func CarregarPFX(caminho, senha string) (*Certificado, error) {
	dados, err := os.ReadFile(caminho) // #nosec G304 — path fornecido pelo operador do sistema
	if err != nil {
		return nil, fmt.Errorf("cert: leitura do arquivo: %w", err)
	}
	return CarregarPFXBytes(dados, senha)
}

// CarregarPFXBytes faz o mesmo a partir de []byte (útil quando o .pfx
// vem de banco de dados ou variável de ambiente).
func CarregarPFXBytes(dados []byte, senha string) (*Certificado, error) {
	chavePriv, cert, err := pkcs12.Decode(dados, senha)
	if err != nil {
		return nil, fmt.Errorf("cert: decodificação PKCS12: %w", err)
	}

	rsakey, ok := chavePriv.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("cert: a chave privada não é RSA")
	}

	return &Certificado{
		Chave:   rsakey,
		Cert:    cert,
		CertDER: cert.Raw,
	}, nil
}

// TLSConfig retorna um *tls.Config com autenticação mútua pronto para
// as chamadas SOAP à SEFAZ.
func (c *Certificado) TLSConfig() *tls.Config {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{c.CertDER},
		PrivateKey:  c.Chave,
		Leaf:        c.Cert,
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
}

// CNPJ extrai o CNPJ do Subject do certificado (campo SerialNumber).
// Retorna string vazia se não encontrado.
func (c *Certificado) CNPJ() string {
	return c.Cert.Subject.SerialNumber
}

// Valido retorna true se o certificado está dentro do prazo de validade.
func (c *Certificado) Valido() bool {
	agora := time.Now()
	return agora.After(c.Cert.NotBefore) && agora.Before(c.Cert.NotAfter)
}
