package sefaz

import (
	"fmt"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// AutorizarContingencia constrói e assina uma NF-e no modo FS-DA (tpEmis=5)
// sem transmitir à SEFAZ. O XML assinado retornado deve ser armazenado pelo
// emissor e retransmitido via Cliente.Autorizar quando a SEFAZ normalizar.
//
// A entrada deve ter TpEmis="5", DhCont e XJust preenchidos; erros de
// validação ou assinatura são propagados diretamente.
func AutorizarContingencia(e builder.EntradaNFe, c *cert.Certificado) ([]byte, error) {
	if e.TpEmis != "5" {
		return nil, fmt.Errorf("sefaz: AutorizarContingencia exige tpEmis=\"5\" (FS-DA), recebido %q", e.TpEmis)
	}

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		return nil, fmt.Errorf("sefaz: contingência build: %w", err)
	}

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: contingência assinar: %w", err)
	}

	return assinado, nil
}
