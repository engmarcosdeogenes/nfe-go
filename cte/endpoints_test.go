package cte

import "testing"

var ufs27 = []string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO"}

func TestEndpoint_TodaUFResolveServicoEAmbiente(t *testing.T) {
	if len(ufs27) != 27 || len(autorizadorUF) != 27 {
		t.Fatalf("esperava 27 UFs: %d/%d", len(ufs27), len(autorizadorUF))
	}
	for _, uf := range ufs27 {
		for _, svc := range []bool{false, true} {
			a, ok := autorizadorUF[uf]
			if svc {
				a, ok = svcUF(uf)
			}
			if !ok {
				t.Errorf("%s svc=%v sem autorizador", uf, svc)
				continue
			}
			for _, amb := range []string{"1", "2"} {
				for _, s := range todosServicos {
					u := a.url(s, amb, "")
					if len(u) < 20 || u[:8] != "https://" {
						t.Errorf("%s %s amb %s: url %q", uf, s.nome, amb, u)
					}
				}
			}
		}
	}
}

func TestEndpoint_AutorizadorPorUF(t *testing.T) {
	casos := map[string]string{"GO": "SVRS", "SE": "SVRS", "RS": "SVRS", "SP": "SVSP", "PE": "SVSP", "AP": "SVSP", "RR": "SVSP", "MG": "MG", "MS": "MS", "MT": "MT", "PR": "PR"}
	for uf, want := range casos {
		cl := NovoClienteHTTP(uf, "2", "http://x", nil)
		if cl.Autorizador() != want {
			t.Errorf("%s: %s != %s", uf, cl.Autorizador(), want)
		}
	}
	if _, err := novo("XX", "1", false); err == nil {
		t.Error("UF inválida")
	}
	if _, err := novo("SP", "3", false); err == nil {
		t.Error("tpAmb inválido")
	}
	if _, err := NovoCliente("SP", "1", nil); err == nil {
		t.Error("cert nulo")
	}
}

func TestEndpoint_URLsExatas(t *testing.T) {
	casos := []struct {
		uf, amb string
		s       servicoCTe
		want    string
	}{
		{"GO", "1", svcRecepcaoSinc, "https://cte.svrs.rs.gov.br/ws/CTeRecepcaoSincV4/CTeRecepcaoSincV4.asmx"},
		{"GO", "2", svcConsulta, "https://cte-homologacao.svrs.rs.gov.br/ws/CTeConsultaV4/CTeConsultaV4.asmx"},
		{"SP", "1", svcRecepcaoEvento, "https://nfe.fazenda.sp.gov.br/CTeWS/WS/CTeRecepcaoEventoV4.asmx"},
		{"PE", "2", svcStatusServico, "https://homologacao.nfe.fazenda.sp.gov.br/CTeWS/WS/CTeStatusServicoV4.asmx"},
		{"MG", "1", svcConsulta, "https://cte.fazenda.mg.gov.br/cte/services/CTeConsultaV4"},
		{"MG", "2", svcRecepcaoSinc, "https://hcte.fazenda.mg.gov.br/cte/services/CTeRecepcaoSincV4"},
		{"MS", "1", svcRecepcaoSinc, "https://producao.cte.ms.gov.br/ws/CTeRecepcaoSincV4"},
		{"MT", "2", svcStatusServico, "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeStatusServicoV4"},
		{"PR", "1", svcRecepcaoOS, "https://cte.fazenda.pr.gov.br/cte4/CTeRecepcaoOSV4"},
	}
	for _, k := range casos {
		if got := autorizadorUF[k.uf].url(k.s, k.amb, ""); got != k.want {
			t.Errorf("%s %s: %s != %s", k.uf, k.s.nome, got, k.want)
		}
	}
}

func TestEndpoint_SOAPAction(t *testing.T) {
	// não confirmado por WSDL (mTLS): sped-cte + MOC §4
	if got, want := svcRecepcaoSinc.soapAction(), "http://www.portalfiscal.inf.br/cte/wsdl/CTeRecepcaoSincV4/cteRecepcao"; got != want {
		t.Errorf("%s != %s", got, want)
	}
	for s, m := range map[servicoCTe]string{svcConsulta: "cteConsultaCT", svcRecepcaoEvento: "cteRecepcaoEvento", svcStatusServico: "cteStatusServicoCT"} {
		if got, want := s.soapAction(), nsWSDLBase+s.nome+"/"+m; got != want {
			t.Errorf("%s != %s", got, want)
		}
	}
}

func TestEndpoint_SVC(t *testing.T) {
	casos := map[string]string{"GO": "SVC-SP", "RS": "SVC-SP", "SP": "SVC-RS", "MG": "SVC-RS", "PR": "SVC-RS", "AP": "SVC-RS"}
	for uf, want := range casos {
		cl, err := novo(uf, "2", true)
		if err != nil || cl.Autorizador() != want {
			t.Errorf("%s: %v %v", uf, cl, err)
		}
	}
	if u := autSVCRS.url(svcRecepcaoSinc, "1", ""); u != "https://cte.svrs.rs.gov.br/ws/CTeRecepcaoSincV4/CTeRecepcaoSincV4.asmx" {
		t.Error(u)
	}
	if u := autSVCSP.url(svcRecepcaoSinc, "2", ""); u != "https://homologacao.nfe.fazenda.sp.gov.br/CTeWS/WS/CTeRecepcaoSincV4.asmx" {
		t.Error(u)
	}
	if _, err := novo("XX", "2", true); err == nil {
		t.Error("UF inválida")
	}
}
