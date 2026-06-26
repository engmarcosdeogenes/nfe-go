package danfe_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/danfe"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

func nfeAssinadaParaTeste(t *testing.T) []byte {
	t.Helper()
	entrada := builder.EntradaNFe{
		Serie:    "1",
		NNF:      "42",
		DhEmi:    time.Date(2026, 6, 25, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		NatOp:    "VENDA DE MERCADORIA",
		TpAmb:    "2",
		FinNFe:   "1",
		IndFinal: "0",
		IndPres:  "1",
		Emitente: builder.EntradaEmitente{
			CNPJ: "11222333000181", Nome: "METALURGICA TESTE LTDA", Fantasia: "METALTESTE",
			IE: "123456789", CRT: "1",
			End: builder.EntradaEndereco{
				Logradouro: "Rua das Chapas", Numero: "100", Bairro: "Industrial",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74000000", Pais: "1058", NomePais: "Brasil", Fone: "6299999999",
			},
		},
		Dest: builder.EntradaDest{
			CNPJ: "99888777000155", Nome: "CLIENTE INDUSTRIA SA", IndIEDest: "1", IE: "987654321",
			End: builder.EntradaEndereco{
				Logradouro: "Av. do Aco", Numero: "500", Bairro: "Centro",
				CodigoMun: "5208707", Municipio: "Goiania", UF: "GO",
				CEP: "74100000", Pais: "1058", NomePais: "Brasil",
			},
		},
		Itens: []builder.EntradaItem{
			{
				CProd: "CHAPA-001", CEAN: "SEM GTIN", Nome: "CHAPA DE ACO INOX 304 2MM",
				NCM: "72193300", CFOP: "5102", Unidade: "KG",
				Quantidade: 100, VUnitario: 25.50,
				ICMS: builder.EntradaICMS{CSOSN: "400"},
			},
			{
				CProd: "PERF-002", CEAN: "SEM GTIN", Nome: "PERFIL ESTRUTURAL L 2X2",
				NCM: "72162100", CFOP: "5102", Unidade: "UN",
				Quantidade: 50, VUnitario: 18.00, VDesconto: 50.00,
				ICMS: builder.EntradaICMS{CSOSN: "400"},
			},
		},
		Frete:     builder.EntradaFrete{Modalidade: "1"},
		Pagamento: []builder.EntradaPagamento{{Forma: "15", Valor: 3400.00, APrazo: true}},
		InfCpl:    "Pedido interno: #42 - Condição de pagamento: 30/60/90 dias.",
	}

	xmlBytes, _, err := builder.Build(entrada)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	pfx, err := cert.GerarCertificadoTeste("11222333000181", "teste")
	if err != nil {
		t.Fatalf("GerarCertificado: %v", err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "teste")
	if err != nil {
		t.Fatalf("CarregarPFX: %v", err)
	}

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		t.Fatalf("AssinarNFe: %v", err)
	}
	return assinado
}

func TestGerarDANFE(t *testing.T) {
	nfeXML := nfeAssinadaParaTeste(t)

	pdfBytes, err := danfe.Gerar(nfeXML)
	if err != nil {
		t.Fatalf("Gerar: %v", err)
	}

	if len(pdfBytes) < 1000 {
		t.Errorf("PDF muito pequeno: %d bytes", len(pdfBytes))
	}

	// PDF começa com "%PDF"
	if !strings.HasPrefix(string(pdfBytes[:4]), "%PDF") {
		t.Error("output não é um PDF válido (não começa com %PDF)")
	}

	t.Logf("PDF gerado: %d bytes", len(pdfBytes))
}

func TestGerarDANFE_SalvaArquivo(t *testing.T) {
	if os.Getenv("DANFE_SALVAR") == "" {
		t.Skip("set DANFE_SALVAR=1 para salvar o PDF em disco")
	}

	nfeXML := nfeAssinadaParaTeste(t)
	pdfBytes, err := danfe.Gerar(nfeXML)
	if err != nil {
		t.Fatalf("Gerar: %v", err)
	}

	path := "testdata/danfe_teste.pdf"
	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pdfBytes, 0644); err != nil {
		t.Fatalf("escrever PDF: %v", err)
	}
	t.Logf("PDF salvo em %s (%d bytes)", path, len(pdfBytes))
}

func TestGerarComTransportadora(t *testing.T) {
	const xmlComTransp = `<?xml version="1.0" encoding="UTF-8"?>
<NFe xmlns="http://www.portalfiscal.inf.br/nfe">
  <infNFe Id="NFe35260625112223330001811001000000421965432101" versao="4.00">
    <ide>
      <cUF>35</cUF><cNF>96543210</cNF>
      <natOp>VENDA DE MERCADORIA</natOp>
      <serie>1</serie><nNF>42</nNF>
      <dhEmi>2026-06-25T10:00:00-03:00</dhEmi>
      <tpNF>1</tpNF><idDest>1</idDest><cMunFG>5208707</cMunFG>
      <tpImp>1</tpImp><tpEmis>1</tpEmis><cDV>1</cDV>
      <tpAmb>2</tpAmb><finNFe>1</finNFe><indFinal>0</indFinal>
      <indPres>1</indPres><procEmi>0</procEmi><verProc>1.0</verProc>
    </ide>
    <emit>
      <CNPJ>11222333000181</CNPJ>
      <xNome>METALURGICA TESTE LTDA</xNome>
      <xFant>METALTESTE</xFant>
      <enderEmit>
        <xLgr>Rua das Chapas</xLgr><nro>100</nro>
        <xBairro>Industrial</xBairro><cMun>5208707</cMun>
        <xMun>Goiania</xMun><UF>GO</UF><CEP>74000000</CEP>
        <cPais>1058</cPais><xPais>Brasil</xPais><fone>6299999999</fone>
      </enderEmit>
      <IE>123456789</IE><CRT>1</CRT>
    </emit>
    <dest>
      <CNPJ>99888777000155</CNPJ>
      <xNome>CLIENTE INDUSTRIA SA</xNome>
      <indIEDest>1</indIEDest><IE>987654321</IE>
      <enderDest>
        <xLgr>Av. do Aco</xLgr><nro>500</nro>
        <xBairro>Centro</xBairro><cMun>5208707</cMun>
        <xMun>Goiania</xMun><UF>GO</UF><CEP>74100000</CEP>
        <cPais>1058</cPais><xPais>Brasil</xPais>
      </enderDest>
    </dest>
    <det nItem="1">
      <prod>
        <cProd>CHAPA-001</cProd><cEAN>SEM GTIN</cEAN>
        <xProd>CHAPA DE ACO INOX 304 2MM</xProd>
        <NCM>72193300</NCM><CFOP>5102</CFOP>
        <uCom>KG</uCom><qCom>100.0000</qCom>
        <vUnCom>25.5000000000</vUnCom><vProd>2550.00</vProd>
        <cEANTrib>SEM GTIN</cEANTrib><uTrib>KG</uTrib>
        <qTrib>100.0000</qTrib><vUnTrib>25.5000000000</vUnTrib>
        <indTot>1</indTot>
      </prod>
      <imposto>
        <ICMS><ICMSSN102><orig>0</orig><CSOSN>400</CSOSN></ICMSSN102></ICMS>
      </imposto>
    </det>
    <total>
      <ICMSTot>
        <vBC>0.00</vBC><vICMS>0.00</vICMS><vICMSDeson>0.00</vICMSDeson>
        <vFCP>0.00</vFCP><vBCST>0.00</vBCST><vST>0.00</vST>
        <vFCPST>0.00</vFCPST><vFCPSTRet>0.00</vFCPSTRet>
        <vProd>2550.00</vProd><vFrete>0.00</vFrete><vSeg>0.00</vSeg>
        <vDesc>0.00</vDesc><vII>0.00</vII><vIPI>0.00</vIPI>
        <vIPIDevol>0.00</vIPIDevol><vPIS>0.00</vPIS><vCOFINS>0.00</vCOFINS>
        <vOutro>850.00</vOutro><vNF>3400.00</vNF><vTotTrib>0.00</vTotTrib>
      </ICMSTot>
    </total>
    <transp>
      <modFrete>0</modFrete>
      <transporta>
        <CNPJ>33333222000144</CNPJ>
        <xNome>TRANSPORTADORA VELOZ LTDA</xNome>
        <IE>111222333</IE>
        <xEnder>Rua das Carretas, 500</xEnder>
        <xMun>Goiania</xMun>
        <UF>GO</UF>
      </transporta>
      <vol>
        <qVol>10</qVol>
        <esp>CAIXAS</esp>
        <marca>METALTESTE</marca>
        <nVol>001-010</nVol>
        <pesoB>255.000</pesoB>
        <pesoL>250.000</pesoL>
      </vol>
    </transp>
    <pag>
      <detPag><tPag>01</tPag><vPag>3400.00</vPag></detPag>
    </pag>
    <infAdic><infCpl>Pedido 42.</infCpl></infAdic>
  </infNFe>
</NFe>`

	// 1. Verificar parsing da transportadora
	dados, err := danfe.ParseNFeXML([]byte(xmlComTransp))
	if err != nil {
		t.Fatalf("ParseNFeXML: %v", err)
	}

	if dados.TranspNome != "TRANSPORTADORA VELOZ LTDA" {
		t.Errorf("TranspNome: %q", dados.TranspNome)
	}
	if dados.TranspCNPJ != "33.333.222/0001-44" {
		t.Errorf("TranspCNPJ: %q, esperava \"33.333.222/0001-44\"", dados.TranspCNPJ)
	}
	if dados.TranspIE != "111222333" {
		t.Errorf("TranspIE: %q", dados.TranspIE)
	}
	if dados.TranspEnd != "Rua das Carretas, 500" {
		t.Errorf("TranspEnd: %q", dados.TranspEnd)
	}
	if dados.TranspMun != "Goiania" {
		t.Errorf("TranspMun: %q", dados.TranspMun)
	}
	if dados.TranspUF != "GO" {
		t.Errorf("TranspUF: %q", dados.TranspUF)
	}

	// 2. Verificar volumes
	if len(dados.Volumes) != 1 {
		t.Fatalf("Volumes: esperava 1, got %d", len(dados.Volumes))
	}
	vol := dados.Volumes[0]
	if vol.Quantidade != 10 {
		t.Errorf("vol.Quantidade: %g, esperava 10", vol.Quantidade)
	}
	if vol.Especie != "CAIXAS" {
		t.Errorf("vol.Especie: %q", vol.Especie)
	}
	if vol.Marca != "METALTESTE" {
		t.Errorf("vol.Marca: %q", vol.Marca)
	}
	if vol.Numeracao != "001-010" {
		t.Errorf("vol.Numeracao: %q", vol.Numeracao)
	}
	if vol.PesoBruto != 255.0 {
		t.Errorf("vol.PesoBruto: %g, esperava 255", vol.PesoBruto)
	}
	if vol.PesoLiq != 250.0 {
		t.Errorf("vol.PesoLiq: %g, esperava 250", vol.PesoLiq)
	}
	t.Logf("Transp: %s | CNPJ: %s | Vol: %g cx | PesoB: %.3f kg",
		dados.TranspNome, dados.TranspCNPJ, vol.Quantidade, vol.PesoBruto)

	// 3. Gerar PDF com bloco de transporte preenchido
	pdfBytes, err := danfe.Gerar([]byte(xmlComTransp))
	if err != nil {
		t.Fatalf("Gerar: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Errorf("PDF muito pequeno: %d bytes", len(pdfBytes))
	}
	if !strings.HasPrefix(string(pdfBytes[:4]), "%PDF") {
		t.Error("output não é um PDF válido (não começa com %PDF)")
	}
	t.Logf("PDF com transportadora gerado: %d bytes", len(pdfBytes))
}

func TestGerarComDuplicatas(t *testing.T) {
	const xmlComDuplicatas = `<?xml version="1.0" encoding="UTF-8"?>
<NFe xmlns="http://www.portalfiscal.inf.br/nfe">
  <infNFe Id="NFe35260625112223330001811001000000421965432101" versao="4.00">
    <ide>
      <cUF>35</cUF><cNF>96543210</cNF>
      <natOp>VENDA DE MERCADORIA</natOp>
      <serie>1</serie><nNF>42</nNF>
      <dhEmi>2026-06-25T10:00:00-03:00</dhEmi>
      <tpNF>1</tpNF><idDest>1</idDest><cMunFG>5208707</cMunFG>
      <tpImp>1</tpImp><tpEmis>1</tpEmis><cDV>1</cDV>
      <tpAmb>2</tpAmb><finNFe>1</finNFe><indFinal>0</indFinal>
      <indPres>1</indPres><procEmi>0</procEmi><verProc>1.0</verProc>
    </ide>
    <emit>
      <CNPJ>11222333000181</CNPJ>
      <xNome>METALURGICA TESTE LTDA</xNome>
      <xFant>METALTESTE</xFant>
      <enderEmit>
        <xLgr>Rua das Chapas</xLgr><nro>100</nro>
        <xBairro>Industrial</xBairro><cMun>5208707</cMun>
        <xMun>Goiania</xMun><UF>GO</UF><CEP>74000000</CEP>
        <cPais>1058</cPais><xPais>Brasil</xPais><fone>6299999999</fone>
      </enderEmit>
      <IE>123456789</IE><CRT>1</CRT>
    </emit>
    <dest>
      <CNPJ>99888777000155</CNPJ>
      <xNome>CLIENTE INDUSTRIA SA</xNome>
      <indIEDest>1</indIEDest><IE>987654321</IE>
      <enderDest>
        <xLgr>Av. do Aco</xLgr><nro>500</nro>
        <xBairro>Centro</xBairro><cMun>5208707</cMun>
        <xMun>Goiania</xMun><UF>GO</UF><CEP>74100000</CEP>
        <cPais>1058</cPais><xPais>Brasil</xPais>
      </enderDest>
    </dest>
    <det nItem="1">
      <prod>
        <cProd>CHAPA-001</cProd><cEAN>SEM GTIN</cEAN>
        <xProd>CHAPA DE ACO INOX 304 2MM</xProd>
        <NCM>72193300</NCM><CFOP>5102</CFOP>
        <uCom>KG</uCom><qCom>100.0000</qCom>
        <vUnCom>25.5000000000</vUnCom><vProd>2550.00</vProd>
        <cEANTrib>SEM GTIN</cEANTrib><uTrib>KG</uTrib>
        <qTrib>100.0000</qTrib><vUnTrib>25.5000000000</vUnTrib>
        <indTot>1</indTot>
      </prod>
      <imposto>
        <ICMS><ICMSSN102><orig>0</orig><CSOSN>400</CSOSN></ICMSSN102></ICMS>
      </imposto>
    </det>
    <total>
      <ICMSTot>
        <vBC>0.00</vBC><vICMS>0.00</vICMS><vICMSDeson>0.00</vICMSDeson>
        <vFCP>0.00</vFCP><vBCST>0.00</vBCST><vST>0.00</vST>
        <vFCPST>0.00</vFCPST><vFCPSTRet>0.00</vFCPSTRet>
        <vProd>2550.00</vProd><vFrete>0.00</vFrete><vSeg>0.00</vSeg>
        <vDesc>0.00</vDesc><vII>0.00</vII><vIPI>0.00</vIPI>
        <vIPIDevol>0.00</vIPIDevol><vPIS>0.00</vPIS><vCOFINS>0.00</vCOFINS>
        <vOutro>850.00</vOutro><vNF>3400.00</vNF><vTotTrib>0.00</vTotTrib>
      </ICMSTot>
    </total>
    <transp><modFrete>1</modFrete></transp>
    <cobr>
      <dup><nDup>001</nDup><dVenc>2026-07-25</dVenc><vDup>1133.33</vDup></dup>
      <dup><nDup>002</nDup><dVenc>2026-08-25</dVenc><vDup>1133.33</vDup></dup>
      <dup><nDup>003</nDup><dVenc>2026-09-25</dVenc><vDup>1133.34</vDup></dup>
    </cobr>
    <pag>
      <detPag><tPag>15</tPag><vPag>3400.00</vPag></detPag>
    </pag>
    <infAdic><infCpl>Pedido 42 - 30/60/90 dias.</infCpl></infAdic>
  </infNFe>
</NFe>`

	// 1. Verificar parsing das 3 duplicatas
	dados, err := danfe.ParseNFeXML([]byte(xmlComDuplicatas))
	if err != nil {
		t.Fatalf("ParseNFeXML: %v", err)
	}
	if len(dados.Duplicatas) != 3 {
		t.Errorf("esperava 3 duplicatas, got %d", len(dados.Duplicatas))
	}
	if len(dados.Duplicatas) == 3 {
		if dados.Duplicatas[0].Num != "001" {
			t.Errorf("dup[0].Num: %q, esperava \"001\"", dados.Duplicatas[0].Num)
		}
		if dados.Duplicatas[1].Vencimento != "25/08/2026" {
			t.Errorf("dup[1].Vencimento: %q, esperava \"25/08/2026\"", dados.Duplicatas[1].Vencimento)
		}
		if dados.Duplicatas[2].Valor != 1133.34 {
			t.Errorf("dup[2].Valor: %.2f, esperava 1133.34", dados.Duplicatas[2].Valor)
		}
	}
	t.Logf("Duplicatas parseadas: %d", len(dados.Duplicatas))

	// 2. Gerar PDF — deve conter o bloco DUPLICATAS renderizado
	pdfBytes, err := danfe.Gerar([]byte(xmlComDuplicatas))
	if err != nil {
		t.Fatalf("Gerar: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Errorf("PDF muito pequeno: %d bytes", len(pdfBytes))
	}
	if !strings.HasPrefix(string(pdfBytes[:4]), "%PDF") {
		t.Error("output não é um PDF válido (não começa com %PDF)")
	}
	t.Logf("PDF com 3 duplicatas gerado: %d bytes", len(pdfBytes))
}

func TestParseNFeXML(t *testing.T) {
	nfeXML := nfeAssinadaParaTeste(t)

	dados, err := danfe.ParseNFeXML(nfeXML)
	if err != nil {
		t.Fatalf("ParseNFeXML: %v", err)
	}

	if dados.EmitNome != "METALURGICA TESTE LTDA" {
		t.Errorf("EmitNome: %q", dados.EmitNome)
	}
	if dados.DestNome != "CLIENTE INDUSTRIA SA" {
		t.Errorf("DestNome: %q", dados.DestNome)
	}
	if len(dados.Itens) != 2 {
		t.Errorf("esperava 2 itens, got %d", len(dados.Itens))
	}
	if dados.VNF != 3400.00 {
		t.Errorf("VNF: %.2f (esperava 3400.00)", dados.VNF)
	}
	if dados.ChaveAcesso == "" {
		t.Error("ChaveAcesso vazia")
	}
	if len(dados.ChaveAcesso) != 44 {
		t.Errorf("ChaveAcesso com %d dígitos (esperava 44): %s", len(dados.ChaveAcesso), dados.ChaveAcesso)
	}
	t.Logf("Chave: %s", dados.ChaveAcesso)
	t.Logf("Emitente: %s | Destinatário: %s", dados.EmitNome, dados.DestNome)
	t.Logf("Total NF: R$ %.2f | Itens: %d", dados.VNF, len(dados.Itens))
}
