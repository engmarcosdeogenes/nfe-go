# nfe-go

Biblioteca Go pura para documentos fiscais eletrônicos brasileiros — **sem
API de terceiro** (Focus NFe, eNotas, etc.). Constrói, assina, transmite à
SEFAZ e gera o DANFE.

- Módulo: `github.com/engmarcosdeogenes/nfe-go` · Go 1.25
- Cobre: NF-e 4.0 e NFC-e (modelo 65)
- Consumida por: [[notas-fiscais]] (SaaS fiscal) e o ERP [[MetalurgicaBase]]
- Desenhada como lib reutilizável — nenhum acoplamento a projeto consumidor

## Pacotes

| Pacote | O que faz |
|---|---|
| `builder/` | `Build(EntradaNFe) → (xmlBytes, ChaveAcesso, error)`. Converte a struct de alto nível `EntradaNFe` no XML do schema SEFAZ. Calcula totais, DV da chave, monta grupos de imposto. `CalcularTotaisEPEC` para contingência. |
| `sign/` | Assinatura digital xmldsig (Manual de Integração §4): canoniza `<infNFe>`, SHA-1 → DigestValue, RSA-SHA1 do SignedInfo → SignatureValue, insere `<Signature>` antes de `</NFe>`. |
| `cert/` | Carrega certificado A1 (`.pfx` / PKCS12) e monta o `tls.Config` para TLS mútuo com a SEFAZ. Raiz ICP-Brasil embutida (`//go:embed`). |
| `sefaz/` | Cliente SOAP: `autorizacao`, `cadastro` (consulta), `contingencia` (FS-DA), `distribuicao` (DistDFe), `epec`, `eventos` (CC-e, cancelamento, manifestação...). `endpoints.go` mapeia URL por UF/ambiente; `client.go` mapeia SOAPAction por serviço. |
| `danfe/` | Gera o DANFE em PDF (retrato A4) e o cupom NFC-e a partir do XML autorizado. `danfe.Gerar(nfeXML) → []byte`. `parser.go` desserializa o XML de volta. |

Deps diretas: `go-pdf/fpdf` (PDF), `boombuler/barcode` + `skip2/go-qrcode`
(código de barras / QR NFC-e), `sslmate/go-pkcs12` (A1).

## Fluxo típico do consumidor

```
cert.Load(pfx, senha)                    → tlsConfig
builder.Build(entrada)                   → xml, chave
sign.Sign(xml, cert)                     → xmlAssinado
sefaz.Autorizar(xmlAssinado, tlsConfig)  → protocolo / rejeição
danfe.Gerar(xmlAutorizado)               → pdf
```

## Convenções

- Cada pacote tem doc comment no `.go` principal explicando o fluxo — **ler
  o doc comment antes de mexer**, ele é a fonte de verdade da mecânica.
- `EntradaNFe` (builder) é a única API que o caller preenche. Não expor
  structs de schema SEFAZ pra fora do pacote.
- Reforma tributária (IBS/CBS/IS) já está no builder — ver testes
  `TestCRT3_IBSCBS*` em `builder/builder_test.go`.

## Testes

`go test ./...` — cobertura pesada no `builder` (variações de CRT, CST de
ICMS, ST, IPI, DIFAL, IBS/CBS com e sem redução, contingência FS-DA,
pagamento com cartão). Rodar sempre depois de mexer em grupo de imposto.

## Não fazer

- Não adicionar dependência de HTTP client externo — `net/http` + `tls`
  puro é requisito (a SEFAZ exige TLS mútuo com curvas específicas).
- Não trocar SHA-1 / RSA-SHA1 na assinatura — é o que o Manual exige, não
  é escolha.
