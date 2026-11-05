package sefaz

import (
	"errors"
	"strings"
	"testing"
)

// Regressão 05/11/2026: CNPJ que vinha de outro sistema levava 656 eterno porque o ultNSU da resposta
// (onde o Ambiente Nacional parou para o CNPJ) se perdia no texto do erro.
func TestLerRetornoDistribuicao_656TrazUltNSU(t *testing.T) {
	env := `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body>
<nfeDistDFeInteresseResponse xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"><nfeDistDFeInteresseResult>
<retDistDFeInt versao="1.01" xmlns="http://www.portalfiscal.inf.br/nfe"><tpAmb>1</tpAmb><verAplic>1.7.6</verAplic>
<cStat>656</cStat><xMotivo>Rejeicao: Consumo Indevido (Deve ser utilizado o ultNSU nas solicitacoes subsequentes. Tente apos 1 hora)</xMotivo>
<dhResp>2026-11-05T20:05:00-03:00</dhResp><ultNSU>000000000030686</ultNSU><maxNSU>000000000000000</maxNSU>
</retDistDFeInt></nfeDistDFeInteresseResult></nfeDistDFeInteresseResponse></soap:Body></soap:Envelope>`
	_, err := lerRetornoDistribuicao([]byte(env))
	var e *ErroDistribuicao
	if !errors.As(err, &e) {
		t.Fatalf("erro não é *ErroDistribuicao: %v", err)
	}
	if e.CStat != "656" || e.UltNSU != "000000000030686" {
		t.Fatalf("erro = %+v", e)
	}
	if !strings.Contains(err.Error(), "cStat=656") {
		t.Fatalf("texto mudou: %s", err)
	}
}
