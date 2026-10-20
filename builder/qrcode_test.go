package builder_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Valores de exemplo do Manual DANFE NFC-e e QR Code v6.0 (mar/2025),
// seções 4.3.6.1 e 4.3.6.2 (QR Code 2.00) e 4.4.1 (3.00 online).
const (
	manualURL  = "http://www.sefazexemplo.gov.br/nfce/qrcode"
	manualCSC  = "SEU-CODIGO-CSC-CONTRIBUINTE-36-CARACTERES"
	manualChOn = "28170800156225000131650110000151341562040824"
	manualChOf = "28170800156225000131650110000151349562040824"
)

func TestQRCode_ExemplosDoManual(t *testing.T) {
	tests := []struct {
		nome string
		got  func() string
		want string
	}{
		{
			"v2 online (4.3.6.1)",
			func() string { return builder.QRCodeOnlineV2(manualURL, manualChOn, "1", "000001", manualCSC) },
			manualURL + "?p=" + manualChOn + "|2|1|1|DC6AE2C2B9A992BE59679AC365E29922DE6B7511",
		},
		{
			"v2 offline (4.3.6.2)",
			func() string {
				return builder.QRCodeOfflineV2(manualURL, manualChOf, "1", "02", "60.90", "yzGYhUx1/XYYzksWB+fPR3Qc50c=", "1", manualCSC)
			},
			manualURL + "?p=" + manualChOf + "|2|1|02|60.90|797a4759685578312f5859597a6b7357422b6650523351633530633d|1|4615A93BB0D7C4E780F8D30EE77EDD5BA55C7D66",
		},
		{
			"v3 online (4.4.1, produção)",
			func() string { return builder.QRCodeOnlineV3(manualURL, manualChOn, "1") },
			manualURL + "?p=" + manualChOn + "|3|1",
		},
		{
			"v3 online (4.4.1, homologação)",
			func() string { return builder.QRCodeOnlineV3(manualURL, manualChOn, "2") },
			manualURL + "?p=" + manualChOn + "|3|2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			if got := tc.got(); got != tc.want {
				t.Errorf("QR = %s\nesperado %s", got, tc.want)
			}
		})
	}
}

// O manual não publica a chave privada do exemplo da 4.4.2, então não dá pra
// reproduzir os bytes da assinatura: confere os 8 parâmetros e verifica a
// assinatura RSA-SHA1 (Base64) com a chave pública.
func TestQRCodeOfflineV3_ParametrosEAssinatura(t *testing.T) {
	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		nome, tpID, id, params string
	}{
		{"CPF", "2", "12345678909", manualChOf + "|3|1|02|60.90|2|12345678909"},
		{"CNPJ", "1", "11222333000181", manualChOf + "|3|1|02|60.90|1|11222333000181"},
		{"estrangeiro", "3", "", manualChOf + "|3|1|02|60.90|3|"},
		{"não identificado", "", "", manualChOf + "|3|1|02|60.90||"},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			qr, err := builder.QRCodeOfflineV3(manualURL, manualChOf, "1", "02", "60.90", tc.tpID, tc.id, chave)
			if err != nil {
				t.Fatal(err)
			}
			prefixo := manualURL + "?p=" + tc.params + "|"
			if !strings.HasPrefix(qr, prefixo) {
				t.Fatalf("QR = %s\nprefixo esperado %s", qr, prefixo)
			}
			sig, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(qr, prefixo))
			if err != nil {
				t.Fatalf("assinatura não é Base64: %v", err)
			}
			h := sha1.Sum([]byte(tc.params))
			if err := rsa.VerifyPKCS1v15(&chave.PublicKey, crypto.SHA1, h[:], sig); err != nil {
				t.Errorf("assinatura RSA-SHA1 não confere: %v", err)
			}
		})
	}
}

func TestBuild_NFCe_QRVersao3_SemCSC(t *testing.T) {
	tests := []struct {
		nome    string
		versao  string
		csc     string
		erro    string
		qrTermo string
	}{
		{"v3 sem CSC passa e não usa hash", "3", "", "", "|3|2"},
		{"v3 com CSC ignora o CSC", "3", "QUALQUER", "", "|3|2"},
		{"v2 explícito com CSC", "2", "CE154B7B6FB48B77", "", "|2|2|1|"},
		{"padrão (vazio) é v2", "", "CE154B7B6FB48B77", "", "|2|2|1|"},
		{"v2 sem CSC é recusado", "2", "", "CSC", ""},
		{"versão inválida", "4", "X", "QRVersao", ""},
	}
	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			e := entradaNFCe()
			e.QRVersao, e.CSC = tc.versao, tc.csc
			xmlBytes, chave, err := builder.Build(e)
			if tc.erro != "" {
				if err == nil || !strings.Contains(err.Error(), tc.erro) {
					t.Fatalf("esperava erro com %q, veio %v", tc.erro, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			qr := extrairTag(string(xmlBytes), "qrCode")
			if !strings.Contains(qr, chave.String()+tc.qrTermo) {
				t.Errorf("qrCode = %s, esperava conter %s%s", qr, chave.String(), tc.qrTermo)
			}
			if tc.versao == "3" && strings.Count(qr, "|") != 2 {
				t.Errorf("QR 3.00 online deve ter só 3 parâmetros: %s", qr)
			}
		})
	}
}

func extrairTag(s, tag string) string {
	i := strings.Index(s, "<"+tag+">")
	j := strings.Index(s, "</"+tag+">")
	if i < 0 || j < 0 {
		return ""
	}
	return s[i+len(tag)+2 : j]
}
