package builder_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/engmarcosdeogenes/nfe-go/builder"
)

// Emitente 11.222.333/0001-81 (GO); CNPJs de transportador com DV válido.
const (
	cnpjTranspOK  = "11444777000161"
	cnpjMesmaBase = "11222333000262" // mesma base do emitente, filial 0002 (DV calculado)
)

func entradaTransporte(mut func(*builder.EntradaNFe)) builder.EntradaNFe {
	e := entradaExemplo()
	e.Frete = builder.EntradaFrete{Modalidade: "1"}
	if mut != nil {
		mut(&e)
	}
	return e
}

func tresItens(e *builder.EntradaNFe) {
	it := e.Itens[0]
	it.VDesconto = 0
	it.Quantidade, it.VUnitario = 1, 1
	e.Itens = []builder.EntradaItem{it, it, it}
}

func decodificar(t *testing.T, e builder.EntradaNFe) (builder.NFe, []byte) {
	t.Helper()
	b, _, err := builder.Build(e)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var nfe builder.NFe
	if err := xml.Unmarshal(b[len(xml.Header):], &nfe); err != nil {
		t.Fatalf("XML inválido: %v", err)
	}
	return nfe, b
}

func TestTransporte_Erros(t *testing.T) {
	casos := []struct {
		nome string
		mut  func(*builder.EntradaNFe)
		erro string
	}{
		{"845 transporta com modFrete 9", func(e *builder.EntradaNFe) {
			e.Frete.Modalidade = "9"
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjTranspOK, Nome: "TRANSP"}
		}, "845"},
		{"845 modalidade vazia vale 9", func(e *builder.EntradaNFe) {
			e.Frete.Modalidade = ""
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjTranspOK, Nome: "TRANSP"}
		}, "845"},
		{"846 modFrete 3 com outro CNPJ base", func(e *builder.EntradaNFe) {
			e.Frete.Modalidade = "3"
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjTranspOK, Nome: "TRANSP"}
		}, "846"},
		{"846 modFrete 3 com CPF", func(e *builder.EntradaNFe) {
			e.Frete.Modalidade = "3"
			e.Frete.Transportador = &builder.EntradaTransportador{CPF: "52998224725", Nome: "AUTONOMO"}
		}, "846"},
		{"868 veículo com idDest 2", func(e *builder.EntradaNFe) {
			e.Dest.End.UF = "SP"
			e.Frete.Veiculo = &builder.EntradaVeiculo{Placa: "ABC1D23", UF: "GO"}
		}, "868"},
		{"868 reboque com idDest 2", func(e *builder.EntradaNFe) {
			e.Dest.End.UF = "SP"
			e.Frete.Reboques = []builder.EntradaVeiculo{{Placa: "ABC1234"}}
		}, "868"},
		{"542 CNPJ com DV errado", func(e *builder.EntradaNFe) {
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: "11444777000162", Nome: "TRANSP"}
		}, "542"},
		{"543 CPF 111...", func(e *builder.EntradaNFe) {
			e.Frete.Transportador = &builder.EntradaTransportador{CPF: "11111111111", Nome: "AUTONOMO"}
		}, "543"},
		{"543 CPF com DV errado", func(e *builder.EntradaNFe) {
			e.Frete.Transportador = &builder.EntradaTransportador{CPF: "52998224726", Nome: "AUTONOMO"}
		}, "543"},
		{"559 IE sem UF", func(e *builder.EntradaNFe) {
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjTranspOK, Nome: "TRANSP", IE: "123456789"}
		}, "559"},
		{"modalidade inválida", func(e *builder.EntradaNFe) { e.Frete.Modalidade = "7" }, "modalidade"},
		{"placa inválida", func(e *builder.EntradaNFe) {
			e.Frete.Veiculo = &builder.EntradaVeiculo{Placa: "12"}
		}, "placa"},
		{"mais de 5 reboques", func(e *builder.EntradaNFe) {
			r := builder.EntradaVeiculo{Placa: "ABC1234"}
			e.Frete.Reboques = []builder.EntradaVeiculo{r, r, r, r, r, r}
		}, "reboques"},
		{"frete negativo", func(e *builder.EntradaNFe) { e.Frete.VFrete = -1 }, "negativo"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, _, err := builder.Build(entradaTransporte(c.mut))
			if err == nil || !strings.Contains(err.Error(), c.erro) {
				t.Fatalf("esperava erro com %q, veio: %v", c.erro, err)
			}
		})
	}
}

func TestTransporte_CNPJMesmaBaseNaoE846(t *testing.T) {
	// Garante que o DV do cnpjMesmaBase usado nos testes é válido (senão o
	// caso cairia em 542 e não provaria nada sobre a regra 846).
	e := entradaTransporte(func(e *builder.EntradaNFe) {
		e.Frete.Modalidade = "3"
		e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjMesmaBase, Nome: "PROPRIO"}
	})
	nfe, _ := decodificar(t, e)
	if nfe.InfNFe.Transp.Transp == nil || nfe.InfNFe.Transp.ModFrete != "3" {
		t.Fatalf("transp = %+v", nfe.InfNFe.Transp)
	}
}

func TestTransporte_Rateio(t *testing.T) {
	// frete 10,00 em 3 itens de 1,00: 3,33 / 3,33 / 3,34 (sobra no último).
	e := entradaTransporte(func(e *builder.EntradaNFe) {
		tresItens(e)
		e.Frete.VFrete = 10
	})
	nfe, _ := decodificar(t, e)
	got := []string{}
	for _, d := range nfe.InfNFe.Det {
		got = append(got, d.Prod.VFrete)
	}
	if strings.Join(got, "/") != "3.33/3.33/3.34" {
		t.Errorf("vFrete por item = %v", got)
	}
	if nfe.InfNFe.Total.ICMSTot.VFrete != "10.00" {
		t.Errorf("vFrete total = %s", nfe.InfNFe.Total.ICMSTot.VFrete)
	}

	// proporcional ao valor: itens 100,00 e 300,00, frete 10,01.
	e = entradaTransporte(func(e *builder.EntradaNFe) {
		a, b := e.Itens[0], e.Itens[0]
		a.VDesconto, b.VDesconto = 0, 0
		a.Quantidade, a.VUnitario = 1, 100
		b.Quantidade, b.VUnitario = 1, 300
		e.Itens = []builder.EntradaItem{a, b}
		e.Frete.VFrete = 10.01
	})
	nfe, _ = decodificar(t, e)
	if a, b := nfe.InfNFe.Det[0].Prod.VFrete, nfe.InfNFe.Det[1].Prod.VFrete; a != "2.50" || b != "7.51" {
		t.Errorf("rateio proporcional = %s / %s, esperava 2.50 / 7.51", a, b)
	}
}

func TestTransporte_SeguroEOutrosNoVNF(t *testing.T) {
	base := entradaTransporte(func(e *builder.EntradaNFe) { tresItens(e) })
	nfe0, _ := decodificar(t, base)

	e := entradaTransporte(func(e *builder.EntradaNFe) {
		tresItens(e)
		e.Frete.VSeg = 5
		e.Frete.VOutro = 2
	})
	nfe, _ := decodificar(t, e)
	tot, tot0 := nfe.InfNFe.Total.ICMSTot, nfe0.InfNFe.Total.ICMSTot
	if tot.VSeg != "5.00" || tot.VOutro != "2.00" {
		t.Errorf("vSeg/vOutro = %s/%s", tot.VSeg, tot.VOutro)
	}
	if tot.VNF != "10.00" || tot0.VNF != "3.00" {
		t.Errorf("vNF = %s (esperava 3.00 + 5 + 2 = 10.00; sem seguro %s)", tot.VNF, tot0.VNF)
	}
	var seg, outro string
	for _, d := range nfe.InfNFe.Det {
		seg += d.Prod.VSeg + "/"
		outro += d.Prod.VOutro + "/"
	}
	if seg != "1.66/1.66/1.68/" || outro != "0.66/0.66/0.68/" {
		t.Errorf("rateio seg=%s outro=%s", seg, outro)
	}
}

func TestTransporte_XMLCompleto(t *testing.T) {
	e := entradaTransporte(func(e *builder.EntradaNFe) {
		e.Frete.VFrete = 50
		e.Frete.Transportador = &builder.EntradaTransportador{
			CNPJ: "11.444.777/0001-61", Nome: "TRANSPORTES RAPIDO LTDA", IE: "10.987.654-3",
			End: "Rodovia BR-060 km 10", Mun: "Goiania", UF: "GO",
		}
		e.Frete.Veiculo = &builder.EntradaVeiculo{Placa: "abc-1d23", UF: "GO", RNTC: "12345"}
		e.Frete.Reboques = []builder.EntradaVeiculo{{Placa: "XYZ1234", UF: "GO"}, {Placa: "QWE4567"}}
		e.Frete.Volumes = []builder.EntradaVolume{{
			Quantidade: 3, Especie: "SUCATA", Marca: "LOTE 7", Numeracao: "1",
			PesoLiq: 1500.5, PesoBruto: 1600, Lacres: []string{"L1", "L2"},
		}}
	})
	nfe, b := decodificar(t, e)
	tr := nfe.InfNFe.Transp
	if tr.Transp == nil || tr.Transp.CNPJ != "11444777000161" || tr.Transp.IE != "109876543" {
		t.Errorf("transporta = %+v", tr.Transp)
	}
	if tr.VeicTransp == nil || tr.VeicTransp.Placa != "ABC1D23" || len(tr.Reboque) != 2 {
		t.Errorf("veículo/reboque = %+v / %+v", tr.VeicTransp, tr.Reboque)
	}
	if len(tr.Vol) != 1 || tr.Vol[0].PesoL != "1500.500" || tr.Vol[0].QVol != "3" || len(tr.Vol[0].Lacres) != 2 {
		t.Errorf("vol = %+v", tr.Vol)
	}
	// Ordem do XSD: transporta -> veicTransp -> reboque -> vol.
	s := string(b)
	ordem := []string{"<transporta>", "<veicTransp>", "<reboque>", "<vol>", "<lacres>"}
	ult := -1
	for _, tag := range ordem {
		i := strings.Index(s, tag)
		if i < 0 || i < ult {
			t.Fatalf("tag %s fora de ordem ou ausente", tag)
		}
		ult = i
	}
	validarXSD(t, b)
}

// TestTransporte_XSD valida contra o XSD as variantes do grupo X (no estilo de
// TestXSD_VariantesDoBuilder; pula quando não há python com lxml).
func TestTransporte_XSD(t *testing.T) {
	transp := &builder.EntradaTransportador{CNPJ: cnpjTranspOK, Nome: "TRANSPORTES RAPIDO LTDA", IE: "109876543", UF: "GO"}
	casos := map[string]func(*builder.EntradaNFe){
		"so_modalidade": func(e *builder.EntradaNFe) {},
		"transportador": func(e *builder.EntradaNFe) { e.Frete.Transportador = transp },
		"transportador_cpf": func(e *builder.EntradaNFe) {
			e.Frete.Transportador = &builder.EntradaTransportador{CPF: "52998224725", Nome: "AUTONOMO"}
		},
		"proprio_remetente": func(e *builder.EntradaNFe) {
			e.Frete.Modalidade = "3"
			e.Frete.Transportador = &builder.EntradaTransportador{CNPJ: cnpjMesmaBase, Nome: "PROPRIO"}
		},
		"veiculo_reboque_vol": func(e *builder.EntradaNFe) {
			e.Frete.Transportador = transp
			e.Frete.Veiculo = &builder.EntradaVeiculo{Placa: "ABC1D23", UF: "GO"}
			e.Frete.Reboques = []builder.EntradaVeiculo{{Placa: "XYZ1234"}}
			e.Frete.Volumes = []builder.EntradaVolume{{Quantidade: 2, Especie: "CAIXA", PesoBruto: 10, Lacres: []string{"1"}}}
		},
		"frete_seg_outro_3_itens": func(e *builder.EntradaNFe) {
			tresItens(e)
			e.Frete.VFrete, e.Frete.VSeg, e.Frete.VOutro = 10, 5, 1
		},
		"interestadual_sem_veiculo": func(e *builder.EntradaNFe) {
			e.Dest.End.UF = "SP"
			e.Frete.Transportador = transp
			e.Frete.Volumes = []builder.EntradaVolume{{Quantidade: 1}}
		},
		"sem_transporte": func(e *builder.EntradaNFe) { e.Frete = builder.EntradaFrete{} },
	}
	for nome, mut := range casos {
		t.Run(nome, func(t *testing.T) {
			_, b := decodificar(t, entradaTransporte(mut))
			validarXSD(t, b)
		})
	}
}

// Nota sem transporte sai igual a antes: só <modFrete>, sem grupos novos.
func TestTransporte_SemTransporteNaoMudaXML(t *testing.T) {
	e := entradaTransporte(func(e *builder.EntradaNFe) { e.Frete = builder.EntradaFrete{} })
	_, b := decodificar(t, e)
	if !strings.Contains(string(b), "<transp><modFrete>9</modFrete></transp>") {
		t.Errorf("transp inesperado: %s", string(b))
	}
}
