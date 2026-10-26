package cte

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

const nProtSF = "926260000000001"

func certSF(t *testing.T) *cert.Certificado {
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

func TestXSD_SefazEventos(t *testing.T) {
	c := certSF(t)
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	cl.agora = func() time.Time { return time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	just := "Cancelamento por erro de digitacao"
	casos := []struct {
		nome, xsd, tp, autor string
		f                    func() (*RetornoEvento, error)
	}{
		{"cancelar", "evCancCTe_v4.00.xsd", "110111", "11222333000181", func() (*RetornoEvento, error) {
			return cl.Cancelar(ctx, c, chaveSF, nProtSF, just)
		}},
		{"cce", "evCCeCTe_v4.00.xsd", "110110", "11222333000181", func() (*RetornoEvento, error) {
			return cl.CartaCorrecao(ctx, c, chaveSF, []Correcao{
				{Grupo: "toma3", Campo: "toma", Valor: "3"},
				{Grupo: "infCarga", Campo: "xOutCat", Valor: "A & B <x>", NroItem: "2"},
			}, 3)
		}},
		{"epec", "evEPECCTe_v4.00.xsd", "110113", "11222333000181", func() (*RetornoEvento, error) {
			return cl.EPEC(ctx, c, chaveSF, EPEC{
				XJust: "Contingencia por indisponibilidade da SEFAZ", VICMS: 12.5, VICMSST: 1, VTPrest: 100, VCarga: 5000,
				Toma: "3", TomaUF: "GO", TomaCNPJ: "11222333000181", TomaIE: "123456789", Modal: "01", UFIni: "GO", UFFim: "SP",
				DhEmi: time.Date(2026, 10, 26, 10, 0, 0, 0, time.FixedZone("x", -3*3600)),
			})
		}},
		{"desacordo", "evPrestDesacordo_v4.00.xsd", "610110", "11222333000181", func() (*RetornoEvento, error) {
			return cl.PrestacaoDesacordo(ctx, c, chaveSF, "Servico nao foi prestado conforme contratado")
		}},
		{"cancdesacordo", "evCancPrestDesacordo_v4.00.xsd", "610111", "11222333000181", func() (*RetornoEvento, error) {
			return cl.CancelarPrestacaoDesacordo(ctx, c, chaveSF, nProtSF)
		}},
	}
	for _, k := range casos {
		r, err := k.f()
		if err != nil {
			t.Fatalf("%s: %v", k.nome, err)
		}
		if r.CStat != "135" || r.XMLProcEvento == nil {
			t.Errorf("%s: %+v", k.nome, r)
		}
		if v.path != "/ws/CTeRecepcaoEventoV4/CTeRecepcaoEventoV4.asmx" {
			t.Errorf("path %s", v.path)
		}
		ev := sfPrimeiro(v.corpo, "eventoCTe")
		valSF(t, "eventoCTe_v4.00.xsd", ev, false)
		valSF(t, k.xsd, ev, true)
		s := string(ev)
		for _, w := range []string{"<tpEvento>" + k.tp + "</tpEvento>", `Id="ID` + k.tp + chaveSF + seqSF(k.nome) + `"`, "<cOrgao>52</cOrgao>", "<CNPJ>" + k.autor + "</CNPJ>", "-03:00</dhEvento>", "<dhEvento>2026-10-26T11:00:00"} {
			if !strings.Contains(s, w) {
				t.Errorf("%s: falta %s em %s", k.nome, w, s)
			}
		}
		if err := sign.VerificarAssinaturaNS(ev, "<infEvento ", sign.NSCTe, c); err != nil {
			t.Errorf("%s: assinatura: %v", k.nome, err)
		}
	}
	if !strings.Contains(string(v.corpo), "<nProtEvPrestDes>"+nProtSF) {
		t.Errorf("último corpo: %s", v.corpo)
	}
}

func TestXSD_SefazEventoAutorTomadorEPF(t *testing.T) {
	c := certSF(t)
	// chave de outro emitente (CNPJ 99888777000166): 610110 usa o CNPJ do certificado (tomador)
	ch := "52261099888777000166570010000000011123456780"
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	if _, err := cl.PrestacaoDesacordo(context.Background(), c, ch, "Servico nao foi prestado conforme contratado"); err != nil {
		t.Fatal(err)
	}
	if ev := string(sfPrimeiro(v.corpo, "eventoCTe")); !strings.Contains(ev, "<CNPJ>11222333000181</CNPJ>") || strings.Contains(ev, "99888777000166</CNPJ>") {
		t.Errorf("autor tomador: %s", ev)
	}
	// emitente: CNPJ da chave; PF: UsarAutorCPF
	if _, err := cl.Cancelar(context.Background(), c, ch, nProtSF, "Cancelamento por erro de digitacao"); err != nil {
		t.Fatal(err)
	}
	if ev := string(sfPrimeiro(v.corpo, "eventoCTe")); !strings.Contains(ev, "<CNPJ>99888777000166</CNPJ>") {
		t.Errorf("autor emitente: %s", ev)
	}
	cl.UsarAutorCPF("123.456.789-09")
	if _, err := cl.Cancelar(context.Background(), c, ch, nProtSF, "Cancelamento por erro de digitacao"); err != nil {
		t.Fatal(err)
	}
	ev := sfPrimeiro(v.corpo, "eventoCTe")
	if !strings.Contains(string(ev), "<CPF>12345678909</CPF>") || strings.Contains(string(ev), "<CNPJ>") {
		t.Errorf("autor PF: %s", ev)
	}
	valSF(t, "eventoCTe_v4.00.xsd", ev, false)
}

func TestSefazEventos_EntradaInvalidaNaoChamaRede(t *testing.T) {
	c := certSF(t)
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	ctx := context.Background()
	ok := EPEC{XJust: "Contingencia por indisponibilidade", Toma: "3", TomaUF: "GO", TomaCNPJ: "11222333000181", Modal: "01", UFIni: "GO", UFFim: "SP", DhEmi: time.Now()}
	mal := ok
	mal.TomaCPF = "12345678909" // CNPJ e CPF juntos
	erros := map[string]func() error{
		"xJust curta": func() error { _, e := cl.Cancelar(ctx, c, chaveSF, nProtSF, "curta"); return e },
		"xJust longa": func() error { _, e := cl.Cancelar(ctx, c, chaveSF, nProtSF, strings.Repeat("a", 256)); return e },
		"nProt":       func() error { _, e := cl.Cancelar(ctx, c, chaveSF, "1", strings.Repeat("a", 20)); return e },
		"chave":       func() error { _, e := cl.Cancelar(ctx, c, "123", nProtSF, strings.Repeat("a", 20)); return e },
		"cce vazia":   func() error { _, e := cl.CartaCorrecao(ctx, c, chaveSF, nil, 1); return e },
		"cce nSeq 21": func() error { _, e := cl.CartaCorrecao(ctx, c, chaveSF, []Correcao{{"a", "b", "c", ""}}, 21); return e },
		"cce campo 21": func() error {
			_, e := cl.CartaCorrecao(ctx, c, chaveSF, []Correcao{{"a", strings.Repeat("b", 21), "c", ""}}, 1)
			return e
		},
		"cce item 100": func() error {
			_, e := cl.CartaCorrecao(ctx, c, chaveSF, []Correcao{{"a", "b", "c", "100"}}, 1)
			return e
		},
		"epec ambíguo":  func() error { _, e := cl.EPEC(ctx, c, chaveSF, mal); return e },
		"desacordo obs": func() error { _, e := cl.PrestacaoDesacordo(ctx, c, chaveSF, "curto"); return e },
		"canc desac":    func() error { _, e := cl.CancelarPrestacaoDesacordo(ctx, c, chaveSF, "12"); return e },
		"cert nulo":     func() error { _, e := cl.Cancelar(ctx, nil, chaveSF, nProtSF, strings.Repeat("a", 20)); return e },
	}
	for nome, f := range erros {
		if f() == nil {
			t.Errorf("%s: esperava erro", nome)
		}
	}
	if _, err := cl.EPEC(ctx, c, chaveSF, ok); err != nil {
		t.Errorf("EPEC válida: %v", err)
	}
	if v.path == "" {
		t.Error("EPEC válida deveria chamar")
	}
}

func TestSefazEventos_NoRedeSoNoCasoValido(t *testing.T) {
	c := certSF(t)
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	_, _ = cl.Cancelar(context.Background(), c, chaveSF, "1", "xxxxxxxxxxxxxxxxxxxx")
	if v.path != "" {
		t.Errorf("houve chamada HTTP: %s", v.path)
	}
}

func TestSefazEventos_RejeicaoEHTTP(t *testing.T) {
	c := certSF(t)
	cl, _ := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento_rej.xml"))
	r, err := cl.Cancelar(context.Background(), c, chaveSF, nProtSF, "Cancelamento por erro de digitacao")
	if err != nil || r.CStat != "494" || r.XMLProcEvento != nil {
		t.Errorf("%+v %v", r, err)
	}
	cl, _ = srvSF(t, "GO", 500, []byte("x"))
	if _, err := cl.Cancelar(context.Background(), c, chaveSF, nProtSF, "Cancelamento por erro de digitacao"); err == nil {
		t.Error("HTTP 500")
	}
	cl, _ = srvSF(t, "GO", 200, []byte("lixo"))
	if _, err := cl.Cancelar(context.Background(), c, chaveSF, nProtSF, "Cancelamento por erro de digitacao"); err == nil {
		t.Error("lixo")
	}
}

func seqSF(nome string) string {
	if nome == "cce" {
		return "003"
	}
	return "001"
}
