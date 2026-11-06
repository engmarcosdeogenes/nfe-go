package mdfe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

func certEvTeste(t *testing.T) *cert.Certificado {
	t.Helper()
	pfx, err := cert.GerarCertificadoTeste("11222333000181", "x")
	if err != nil {
		t.Fatal(err)
	}
	c, err := cert.CarregarPFXBytes(pfx, "x")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Cada evento enviado é assinado e valida contra eventoMDFe_v3.00.xsd (detEvento é
// xs:any/skip, então o filho é validado também contra o XSD próprio).
func TestXSD_EventosMDFe(t *testing.T) {
	c := certEvTeste(t)
	cl, v := srv(t, 200, fixture(t, "ret_evento.xml"))
	cl.agora = func() time.Time { return time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	casos := []struct {
		nome, xsd, tp string
		f             func() (*RetornoEvento, error)
	}{
		{"enc", "evEncMDFe_v3.00.xsd", "110112", func() (*RetornoEvento, error) {
			return cl.Encerrar(ctx, c, chaveTeste, "926260000000001", time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), "52", "5208707")
		}},
		{"canc", "evCancMDFe_v3.00.xsd", "110111", func() (*RetornoEvento, error) {
			return cl.Cancelar(ctx, c, chaveTeste, "926260000000001", "Cancelamento por erro de digitacao & teste")
		}},
		{"cond", "evIncCondutorMDFe_v3.00.xsd", "110114", func() (*RetornoEvento, error) {
			return cl.IncluirCondutor(ctx, c, chaveTeste, "JOAO DA SILVA", "12345678909", 2)
		}},
	}
	for _, k := range casos {
		r, err := k.f()
		if err != nil {
			t.Fatalf("%s: %v", k.nome, err)
		}
		if v.path != "/ws/MDFeRecepcaoEvento/MDFeRecepcaoEvento.asmx" {
			t.Errorf("%s: path %s", k.nome, v.path)
		}
		ev := primeiro(v.corpo, "eventoMDFe")
		valSefazXSD(t, "eventoMDFe_v3.00.xsd", ev, false)
		valSefazXSD(t, k.xsd, ev, true)
		if !strings.Contains(string(ev), "<tpEvento>"+k.tp+"</tpEvento>") || !strings.Contains(string(ev), "<Signature ") {
			t.Errorf("%s: %s", k.nome, ev)
		}
		if err := sign.VerificarAssinaturaNS(ev, "<infEvento ", sign.NSMDFe, c); err != nil {
			t.Errorf("%s: assinatura: %v", k.nome, err)
		}
		if r.CStat != "135" || r.XMLProcEvento == nil || !strings.Contains(string(r.XMLProcEvento), "<retEventoMDFe") || r.NProt == "" {
			t.Errorf("%s: %+v", k.nome, r)
		}
	}
	// dhEvento no fuso da UF (GO = -03:00), Id com nSeq de 3 dígitos
	if _, err := cl.IncluirCondutor(ctx, c, chaveTeste, "JOAO DA SILVA", "12345678909", 2); err != nil {
		t.Fatal(err)
	}
	s := string(v.corpo)
	if !strings.Contains(s, `Id="ID1101145226101122233300018158001000000001112345678002"`[:0]+"ID110114"+chaveTeste+"002") ||
		!strings.Contains(s, "<dhEvento>2026-10-26T11:00:00-03:00</dhEvento>") ||
		!strings.Contains(s, "<CNPJ>11222333000181</CNPJ>") || !strings.Contains(s, "<cOrgao>52</cOrgao>") {
		t.Errorf("campos: %s", s)
	}
}

func TestXSD_EventoEmitentePF(t *testing.T) {
	// emitente PF: a chave leva 000+CPF; o autor só vira <CPF> com UsarAutorCPF
	chPF := "5226100001234567890958" + "001" + "000000001" + "1" + "12345678" + "0"
	cl, _ := srv(t, 200, fixture(t, "ret_evento.xml"))
	det := `<evEncMDFe><descEvento>Encerramento</descEvento><nProt>926260000000001</nProt><dtEnc>2026-10-26</dtEnc><cUF>52</cUF><cMun>5208707</cMun></evEncMDFe>`
	padrao, err := cl.montarEvento(certEvTeste(t), chPF, "110112", 1, det)
	if err != nil || !strings.Contains(string(padrao), "<CNPJ>00012345678909</CNPJ>") {
		t.Fatalf("autor padrão deve ser o CNPJ da chave: %v %s", err, padrao)
	}
	valSefazXSD(t, "eventoMDFe_v3.00.xsd", padrao, false)
	cl.UsarAutorCPF("123.456.789-09")
	ev, err := cl.montarEvento(certEvTeste(t), chPF, "110112", 1,
		`<evEncMDFe><descEvento>Encerramento</descEvento><nProt>926260000000001</nProt><dtEnc>2026-10-26</dtEnc><cUF>52</cUF><cMun>5208707</cMun></evEncMDFe>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ev), "<CPF>12345678909</CPF>") || strings.Contains(string(ev), "<CNPJ>") {
		t.Errorf("autor PF: %s", ev)
	}
	valSefazXSD(t, "eventoMDFe_v3.00.xsd", ev, false)
}

func TestEventos_EntradaInvalidaNaoChamaRede(t *testing.T) {
	c := certEvTeste(t)
	cl, v := srv(t, 200, fixture(t, "ret_evento.xml"))
	ctx := context.Background()
	if _, err := cl.Cancelar(ctx, c, chaveTeste, "926260000000001", "curta"); err == nil {
		t.Error("xJust curta")
	}
	if _, err := cl.Cancelar(ctx, c, chaveTeste, "926260000000001", strings.Repeat("a", 256)); err == nil {
		t.Error("xJust longa")
	}
	if _, err := cl.Cancelar(ctx, c, "123", "926260000000001", strings.Repeat("a", 20)); err == nil {
		t.Error("chave inválida")
	}
	if _, err := cl.Encerrar(ctx, c, chaveTeste, "1", time.Now(), "52", "5208707"); err == nil {
		t.Error("nProt")
	}
	if _, err := cl.IncluirCondutor(ctx, c, chaveTeste, "X", "12345678909", 1); err == nil {
		t.Error("nome curto")
	}
	if _, err := cl.IncluirCondutor(ctx, c, chaveTeste, "JOAO", "12345678909", 0); err == nil {
		t.Error("nSeq 0")
	}
	if v.path != "" {
		t.Errorf("houve chamada HTTP: %s", v.path)
	}
}

func TestEventos_RejeicaoEHTTP(t *testing.T) {
	c := certEvTeste(t)
	cl, _ := srv(t, 200, fixture(t, "ret_evento_rej.xml"))
	r, err := cl.Encerrar(context.Background(), c, chaveTeste, "926260000000001", time.Now(), "52", "5208707")
	if err != nil || r.CStat != "636" || r.XMLProcEvento != nil {
		t.Errorf("%+v %v", r, err)
	}
	cl, _ = srv(t, 500, []byte("x"))
	if _, err := cl.Encerrar(context.Background(), c, chaveTeste, "926260000000001", time.Now(), "52", "5208707"); err == nil {
		t.Error("HTTP 500")
	}
}

const nProtTeste = "926260000000001"

// Eventos adicionais (escopo ampliado): cada um valida contra eventoMDFe e contra o ev*.xsd próprio.
func TestXSD_EventosMDFe_Extras(t *testing.T) {
	c := certEvTeste(t)
	cl, v := srv(t, 200, fixture(t, "ret_evento.xml"))
	ctx := context.Background()
	apr := Pagamento{
		Nome: "TRANSPORTADOR AUTONOMO", CPF: "12345678909",
		Comp:      []CompPag{{Tipo: "01", Valor: 100}, {Tipo: "99", Valor: 50.5, XComp: "outros"}},
		VContrato: 150.5, IndPag: "1", VAdiant: 50, IndAntecipaAdiant: true,
		Prazos:    []Prazo{{Parcela: 1, Venc: "2026-11-30", Valor: 100.5}},
		TpAntecip: "1", PIX: "chave@pix.com",
	}
	bco := Pagamento{CNPJ: "11222333000181", Comp: []CompPag{{Tipo: "04", Valor: 10}}, VContrato: 10, IndPag: "0", CodBanco: "001", CodAgencia: "1234"}
	ipef := Pagamento{IDEstrangeiro: "ABCDE12345", Comp: []CompPag{{Tipo: "04", Valor: 0}}, VContrato: 0, IndPag: "0", CNPJIPEF: "11222333000181"}
	nfe := "52261011222333000181550010000000011000000019"
	casos := []struct {
		nome, xsd, tp string
		f             func() (*RetornoEvento, error)
	}{
		{"incdfe", "evInclusaoDFeMDFe_v3.00.xsd", "110115", func() (*RetornoEvento, error) {
			return cl.IncluirDFe(ctx, c, chaveTeste, nProtTeste, "5208707", "Goiania",
				[]DocInclusao{{"5300108", "Brasilia", nfe}, {"3550308", "Sao Paulo", nfe}}, 1)
		}},
		{"confirma", "evConfirmaServMDFe_v3.00.xsd", "110117", func() (*RetornoEvento, error) {
			return cl.ConfirmaServico(ctx, c, chaveTeste, nProtTeste, 1)
		}},
		{"pagto", "evPagtoOperMDFe_v3.00.xsd", "110116", func() (*RetornoEvento, error) {
			return cl.PagamentoOperacao(ctx, c, chaveTeste, nProtTeste, 3, 1, []Pagamento{apr, bco, ipef}, 1)
		}},
		{"altpagto", "evAlteracaoPagtoServMDFe_v3.00.xsd", "110118", func() (*RetornoEvento, error) {
			return cl.AlteracaoPagamento(ctx, c, chaveTeste, nProtTeste, []Pagamento{apr, bco}, 2)
		}},
		{"enc_terceiro", "evEncMDFe_v3.00.xsd", "110112", func() (*RetornoEvento, error) {
			return cl.EncerrarPorTerceiro(ctx, c, chaveTeste, nProtTeste, time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), "52", "5208707")
		}},
	}
	for _, k := range casos {
		if _, err := k.f(); err != nil {
			t.Fatalf("%s: %v", k.nome, err)
		}
		ev := primeiro(v.corpo, "eventoMDFe")
		valSefazXSD(t, "eventoMDFe_v3.00.xsd", ev, false)
		valSefazXSD(t, k.xsd, ev, true)
		if !strings.Contains(string(ev), "<tpEvento>"+k.tp+"</tpEvento>") {
			t.Errorf("%s: tpEvento", k.nome)
		}
		if err := sign.VerificarAssinaturaNS(ev, "<infEvento ", sign.NSMDFe, c); err != nil {
			t.Errorf("%s: %v", k.nome, err)
		}
		if k.nome == "enc_terceiro" && (!strings.Contains(string(ev), "<indEncPorTerceiro>1</indEncPorTerceiro>")) {
			t.Error("indEncPorTerceiro ausente")
		}
	}
}

func TestEventos_Extras_EntradaInvalida(t *testing.T) {
	c := certEvTeste(t)
	cl, v := srv(t, 200, fixture(t, "ret_evento.xml"))
	ctx := context.Background()
	if _, err := cl.IncluirDFe(ctx, c, chaveTeste, nProtTeste, "5208707", "Goiania", nil, 1); err == nil {
		t.Error("sem docs")
	}
	if _, err := cl.PagamentoOperacao(ctx, c, chaveTeste, nProtTeste, 1, 1, []Pagamento{{CPF: "12345678909", CNPJ: "11222333000181"}}, 1); err == nil {
		t.Error("CPF e CNPJ juntos")
	}
	if _, err := cl.AlteracaoPagamento(ctx, c, chaveTeste, nProtTeste, nil, 1); err == nil {
		t.Error("sem infPag")
	}
	if v.path != "" {
		t.Error("não deveria chamar HTTP")
	}
}

func TestAutorizar_ContingenciaTpEmis2(t *testing.T) {
	cl, _ := srv(t, 200, fixture(t, "ret_autorizado.xml"))
	x := []byte(`<MDFe xmlns="http://www.portalfiscal.inf.br/mdfe"><infMDFe Id="MDFe` + chaveTeste + `" versao="3.00"><ide><tpEmis>2</tpEmis></ide></infMDFe></MDFe>`)
	r, err := cl.Autorizar(context.Background(), x)
	if err != nil || r.CStat != "100" || !strings.Contains(string(r.XMLProc), "<tpEmis>2</tpEmis>") {
		t.Errorf("%+v %v", r, err)
	}
}
