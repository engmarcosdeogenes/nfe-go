# nfe-go

Biblioteca Go pura para documentos fiscais eletrônicos brasileiros — **sem
API de terceiro** (Focus NFe, eNotas, etc.). Constrói, assina, transmite à
SEFAZ e gera o DANFE.

- Módulo: `github.com/engmarcosdeogenes/nfe-go` · Go 1.25
- Cobre: NF-e 4.0 (modelo 55), NFC-e (modelo 65) e MDF-e 3.00 (modelo 58), CT-e 4.00 (modelo 57 em todos os modais e CT-e OS modelo 67) — **só mercadoria** (o CT-e OS é serviço de transporte, não NFS-e)
- Consumida por: [[notas-fiscais]] (SaaS fiscal) e o ERP [[MetalurgicaBase]]
- Desenhada como lib reutilizável — nenhum acoplamento a projeto consumidor

## Pacotes

| Pacote | O que faz |
|---|---|
| `builder/` | `Build(EntradaNFe) → (xmlBytes, ChaveAcesso, error)`. Converte a struct de alto nível `EntradaNFe` no XML do schema SEFAZ. Calcula totais, DV da chave, monta grupos de imposto. `CalcularTotaisEPEC` para contingência. |
| `sign/` | Assinatura digital xmldsig (Manual de Integração §4): canoniza `<infNFe>`, SHA-1 → DigestValue, RSA-SHA1 do SignedInfo → SignatureValue, insere `<Signature>` antes de `</NFe>`. |
| `cert/` | Carrega certificado A1 (`.pfx` / PKCS12) e monta o `tls.Config` para TLS mútuo com a SEFAZ. Raiz ICP-Brasil embutida (`//go:embed`). |
| `sefaz/` | Cliente SOAP: `autorizacao`, `cadastro` (consulta), `contingencia` (FS-DA), `distribuicao` (DistDFe), `epec`, `eventos` (CC-e, cancelamento, manifestação...), `gtin` (`ConsultarGTIN` no CCG do SVRS, NT 2022.001; envelope conferido no WSDL em 29/10/2026). `endpoints.go` mapeia URL por UF/ambiente; `client.go` mapeia SOAPAction por serviço. |
| `danfe/` | Gera o DANFE em PDF (retrato A4) e o cupom NFC-e a partir do XML autorizado. `danfe.Gerar(nfeXML) → []byte`. O cupom NFC-e (`GerarDANFENFCe`, `nfce.go`) segue o Manual DANFE NFC-e e QR Code v6.0 (divisões I a IX); o QR Code do builder tem versão 2.00 (padrão, com CSC) e 3.00 (`EntradaNFe.QRVersao="3"`, sem CSC). `parser.go` desserializa o XML de volta. **A geometria do DANFE A4 é normativa**: `layout_anexo2.go` transcreve a tabela de coordenadas e os tamanhos mínimos de fonte do MOC 7.0 Anexo II (§3.7 e §3.8.1), e `render_anexo2.go` desenha em cima dela — não mudar posição, largura ou tamanho de fonte sem conferir a seção citada no comentário. O PDF do manual está em `notas-fiscais/docs sefaz/ANEXO II -Manual EspecificaçõesTécnicas - Danfe-Código-Barras.pdf`. |
| `mdfe/` | MDF-e 58 (rodoviário, leiaute 3.00): `Build(Entrada)` → XML + chave; `Cliente` SVRS (`NovoCliente`: Autorizar, consultas, status, não encerrados); eventos (encerrar, cancelar, condutor, inclusão de DF-e, pagamento, confirmação); contingência (`tpEmis=2` + QR Code com `sign`). Assina com `sign.AssinarMDFe`/`AssinarEventoMDFe`. Autor de evento é o CNPJ da chave; emitente CPF usa `UsarAutorCPF`. **SOAPAction MDF-e não confirmada por WSDL (exige mTLS); confirmar no 1º teste com certificado em homologação.** |
| `mdfe/damdfe/` | DAMDFE em PDF A4 a partir do `mdfeProc`: `damdfe.Gerar(xml, damdfe.Opcoes{})`. |
| `cte/` | CT-e 57 4.00 em todos os modais: `Build(Entrada)` → XML + chave (normal, subcontratação, IBS/CBS, ICMSUFFim). CT-e OS 67 (pessoas, valores, excesso de bagagem; rodoviário, com `infTribFed`; modais 02 a 04 do OS não implementados, o XSD só define o `infModal` rodoviário): `BuildOS(EntradaOS)`, `ChaveOS`, `Cliente.AutorizarOS`; `Cliente` por autorizador da UF do emitente, com SVC (`NovoCliente`, `NovoClienteSVC`): Autorizar, Consultar, status; eventos (cancelar, carta de correção, EPEC, prestação em desacordo). Assina com `sign.AssinarCTe` e `sign.AssinarCTeOS`. Autor de evento é o CNPJ da chave. CNPJ alfanumérico (NT 2025.001) aceito. **Não confirmado: SOAPAction CT-e e SVC por UF (sem WSDL, exige mTLS); rótulos do DACTE, DACTE OS e DAMDFE (sem MOC); EPEC foi extinta pela NT 2026.004 (o método EPEC segue no código, não usar em produção sem conferir).** |
| `cte/dacte/` | DACTE em PDF A4 a partir do `cteProc`: `dacte.Gerar(xml, dacte.Opcoes{})`; DACTE OS a partir do `cteOSProc`: `dacte.GerarOS`. |

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

## Escopo: mercadoria, não serviço

Esta lib emite **NF-e 55, NFC-e 65, MDF-e 58 e CT-e 57 (todos os modais) e CT-e OS 67**. Nota de serviço é
**NFS-e**: documento municipal, leiaute próprio (DANFSE), webservice da
prefeitura ou do Emissor Nacional — não passa por aqui. Quem trata NFS-e é
o [[notas-fiscais]] (`internal/usecase/emitir_nfse.go`), e lá o
`gerar_danfe.go` recusa NFS-e de propósito: o comprovante dela é o
XML/número da prefeitura, não um DANFE.

A NF-e 55 ainda admite o grupo ISSQN no schema, para **nota conjugada**
(mercadoria + serviço no mesmo documento, onde o estado permite). É por isso
que o leiaute do DANFE reserva o quadro "Cálculo do ISSQN". A lib **não
emite conjugada** — não há campo de ISSQN no `builder` nem no `parser` do
`danfe`.

Consequências no DANFE, que são decisão e não esquecimento:

- O quadro "Cálculo do ISSQN" sai suprimido, pelo §3.3.3 do Anexo II, e a
  altura liberada vai para o quadro "Dados dos Produtos/Serviços" — que é a
  condição que o próprio item impõe.
- A **inscrição municipal do emitente não aparece** no documento: ela é o
  campo C19, daquele quadro, e não existe em nenhum outro lugar do leiaute
  (conferido em 3.8.1 retrato e 3.8.2 paisagem). Versões até a v1.6.0
  desenhavam a IM espremida na linha do emitente — isso foi copiado de
  DANFE de concorrente, não do manual, e saiu na v1.7.0.

Se um dia entrar nota conjugada no `builder`, o caminho é fazer o quadro
voltar inteiro (com a IM no lugar dele), não recolocar a IM na linha do
emitente. Se for só para imprimir a IM por outro motivo, o §3.10.5 manda
copiar para `infCpl` — ver `informacoesComplementares()` em
`danfe/render_anexo2.go`.

## Testes

`go test ./...` — cobertura pesada no `builder` (variações de CRT, CST de
ICMS, ST, IPI, DIFAL, IBS/CBS com e sem redução, contingência FS-DA,
pagamento com cartão). Rodar sempre depois de mexer em grupo de imposto.

Os testes XSD de `mdfe/` e `cte/` exigem python3 + lxml (ou `NFE_PYTHON` apontando para um python com lxml); sem isso **falham**, não pulam. MDF-e: o Id de `infMDFe` no XSD 3.00 ainda é numérico, então `mdfe.Build` recusa emitente com CNPJ alfanumérico.

## Não fazer

- Não adicionar dependência de HTTP client externo — `net/http` + `tls`
  puro é requisito (a SEFAZ exige TLS mútuo com curvas específicas).
- Não trocar SHA-1 / RSA-SHA1 na assinatura — é o que o Manual exige, não
  é escolha.
- Não mexer em posição, largura ou tamanho de fonte do DANFE A4 sem abrir a
  seção do Anexo II citada no comentário. Os números de `layout_anexo2.go`
  são o leiaute do manual e os pisos de fonte do §3.7 — não são gosto.
- Não tentar emitir NFS-e por aqui (ver "Escopo: mercadoria, não serviço").
