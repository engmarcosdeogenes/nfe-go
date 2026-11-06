package cte

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
)

// AutorizarOS envia o CT-e OS (modelo 67) assinado ao serviço CTeRecepcaoOSV4,
// em GZip+Base64 como o CT-e 57. A resposta é retCTeOS (retCTeOS_v4.00.xsd:
// tpAmb, cUF, verAplic, cStat, xMotivo, protCTe opcional). Com cStat 100,
// XMLProc é o cteOSProc (procCTeOS_v4.00.xsd: CTeOS + protCTe). Rejeições
// voltam em RetornoAutorizacao sem erro Go.
//
// Não confirmado: SOAPAction e namespace do WSDL do CTeRecepcaoOSV4 (mesma
// convenção do 57, sem WSDL por exigir mTLS) e se a recepção do OS é síncrona
// em todos os autorizadores (o XSD de retorno traz o protocolo, então assumido).
func (cl *Cliente) AutorizarOS(ctx context.Context, cteOSAssinado []byte) (*RetornoAutorizacao, error) {
	x := sfSemDeclaracao(cteOSAssinado)
	var zb bytes.Buffer
	zw := gzip.NewWriter(&zb)
	if _, err := zw.Write(x); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	inner, err := cl.chamar(ctx, svcRecepcaoOS, base64.StdEncoding.EncodeToString(zb.Bytes()))
	if err != nil {
		return nil, err
	}
	ret, err := sfExigir(inner, "retCTeOS")
	if err != nil {
		return nil, err
	}
	r := &RetornoAutorizacao{CStat: sfTexto(ret, "cStat"), XMotivo: sfTexto(ret, "xMotivo")}
	prot := sfPrimeiro(ret, "protCTe")
	if prot != nil {
		inf := sfPrimeiro(prot, "infProt")
		r.NProt, r.DhRecbto, r.ChCTe = sfTexto(inf, "nProt"), sfTexto(inf, "dhRecbto"), sfTexto(inf, "chCTe")
		if c := sfTexto(inf, "cStat"); c != "" {
			r.CStat, r.XMotivo = c, sfTexto(inf, "xMotivo")
		}
	}
	if r.CStat == "" {
		return nil, fmt.Errorf("cte: retCTeOS sem cStat: %s", sfTruncar(ret, 256))
	}
	if r.CStat == "100" && prot != nil {
		r.XMLProc = []byte(`<cteOSProc versao="4.00" xmlns="` + nsCTe + `">` + string(x) + string(prot) + `</cteOSProc>`)
	}
	return r, nil
}
