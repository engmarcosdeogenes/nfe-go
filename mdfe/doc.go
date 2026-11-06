// Package mdfe monta o XML do MDF-e (modelo 58, leiaute 3.00, MOC MDF-e 3.00a)
// para o caso tpEmit=2 (empresa que transporta carga própria, vinculando as
// NF-e dela), modal rodoviário.
//
// Fluxo: Build devolve o XML sem assinatura e a chave de 44 dígitos; quem
// assina (pacote sign) insere <Signature> depois de <infMDFeSupl>, que é a
// ordem de TMDFe no XSD (infMDFe, infMDFeSupl, Signature).
//
// A fonte de verdade de ordem, obrigatoriedade e pattern é o XSD em
// testdata/schemas/mdfe; mdfe/xsd_test.go valida cada variante contra ele.
// Fora do escopo desta onda: tpEmit 1 e 3, contingência (tpEmis 2), CT-e,
// seguro (seg), idCSRT/hashCSRT, infSolicNFF e infPAA.
package mdfe
