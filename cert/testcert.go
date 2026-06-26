package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// GerarCertificadoTeste cria um par RSA autoassinado e devolve os bytes
// do .pfx com a senha fornecida. Usado exclusivamente em testes e homologação.
func GerarCertificadoTeste(cnpj, senha string) ([]byte, error) {
	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "NF-e TESTE - " + cnpj,
			SerialNumber: cnpj,
			Organization: []string{"EMPRESA DE TESTE LTDA"},
			Country:      []string{"BR"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &chave.PublicKey, chave)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	return pkcs12.Legacy.Encode(chave, cert, nil, senha)
}

// GerarPEMTeste retorna cert.pem e key.pem como strings (útil para debug).
func GerarPEMTeste(cnpj string) (certPEM, keyPEM string, err error) {
	pfx, err := GerarCertificadoTeste(cnpj, "teste")
	if err != nil {
		return "", "", err
	}

	c, err := CarregarPFXBytes(pfx, "teste")
	if err != nil {
		return "", "", err
	}

	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: c.CertDER}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(c.Chave)
	if err != nil {
		return "", "", err
	}
	keyBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}

	return string(pem.EncodeToMemory(certBlock)), string(pem.EncodeToMemory(keyBlock)), nil
}
