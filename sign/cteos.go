package sign

import "github.com/engmarcosdeogenes/nfe-go/cert"

// AssinarCTeOS assina <infCte Id="CTe..."> do CT-e OS (modelo 67) e insere a
// Signature antes de </CTeOS> (depois de infCTeSupl). AssinarCTe não serve:
// procura </CTe> e, sem achar, anexaria a assinatura fora da raiz.
func AssinarCTeOS(x []byte, c *cert.Certificado) ([]byte, error) {
	return assinarGenerico(x, []byte("<infCte "), []byte("</CTeOS>"), NSCTe, c)
}
