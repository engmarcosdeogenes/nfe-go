// Package cte monta o XML do CT-e (modelo 57, leiaute 4.00, MOC CT-e 4.00 +
// NT 2025.001 RTC / 2026.002 / 2026.004), pacote de schemas
// PL_CTe_400_NT2026.004 em testdata/schemas/cte.
//
// Fluxo: Build devolve o XML sem assinatura, sem <?xml?> e a chave de 44
// dígitos; quem assina insere <Signature> depois de <infCTeSupl> (ordem de
// TCTe: infCte, infCTeSupl, Signature).
//
// tpCTe: o XSD aceita 0 (normal), 1 (complemento) e 3 (substituição). O 2
// (anulação) foi extinto no leiaute 4.00 e não existe no XSD.
//
// Modal: só o rodoviário (01) é montado. Os tipos Aereo, Aquaviario,
// Ferroviario, Dutoviario e Multimodal existem no modelo de dados, mas Build
// devolve "modal X ainda não implementado" (Onda 4).
//
// infDoc: o XSD exige UM tipo de documento por CT-e (choice entre infNF,
// infNFe, infOutros e infDCe); misturar é recusado aqui antes da rejeição
// de schema (225).
//
// Chave: CNPJ alfanumérico está no pattern do XSD, mas o cálculo da chave
// com letras não é feito aqui; a chave é numérica.
package cte
