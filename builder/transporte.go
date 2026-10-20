package builder

import (
	"fmt"
	"regexp"
	"strings"
)

// Grupo X (transporte): montagem, rateio de frete/seguro/outras despesas entre
// os itens e as validações que a SEFAZ faz (845, 846, 868, 542, 543, 559).

var modalidadesFrete = map[string]bool{"0": true, "1": true, "2": true, "3": true, "4": true, "9": true}

// TVeiculo/placa do XSD: [A-Z]{2,3}[0-9]{4}|[A-Z]{3,4}[0-9]{3}|[A-Z0-9]{7}.
var rePlaca = regexp.MustCompile(`^([A-Z]{2,3}[0-9]{4}|[A-Z]{3,4}[0-9]{3}|[A-Z0-9]{7})$`)

// formatarPlaca tira hífen e espaço ("ABC-1D23") e põe em maiúsculas.
func formatarPlaca(p string) string {
	return strings.ToUpper(FormatarIE(p))
}

func modalidadeFrete(f EntradaFrete) string {
	if f.Modalidade == "" {
		return "9"
	}
	return f.Modalidade
}

// ratearFrete divide frete, seguro e outras despesas entre os itens,
// proporcional ao valor bruto de cada um (vProd). Cada parcela é truncada em
// centavos e a sobra vai toda para o último item, então a soma bate exatamente
// com o total informado (regras 535, 536 e 604). Devolve centavos por item.
func ratearFrete(e EntradaNFe) (frete, seg, outro []int64) {
	valores := make([]int64, len(e.Itens))
	var soma int64
	for i, it := range e.Itens {
		valores[i] = centavos(it.Quantidade * it.VUnitario)
		soma += valores[i]
	}
	return ratearPorValor(centavos(e.Frete.VFrete), valores, soma),
		ratearPorValor(centavos(e.Frete.VSeg), valores, soma),
		ratearPorValor(centavos(e.Frete.VOutro), valores, soma)
}

func ratearPorValor(total int64, valores []int64, soma int64) []int64 {
	out := make([]int64, len(valores))
	if total <= 0 || len(valores) == 0 {
		return out
	}
	var dado int64
	for i := range valores {
		if soma > 0 {
			out[i] = total * valores[i] / soma
		}
		dado += out[i]
	}
	out[len(out)-1] += total - dado
	return out
}

func montarTransporte(f EntradaFrete, modFrete string) Transporte {
	t := Transporte{ModFrete: modFrete}
	if tr := f.Transportador; tr != nil {
		ie := FormatarIE(tr.IE)
		if strings.EqualFold(ie, "ISENTO") {
			ie = "ISENTO"
		}
		t.Transp = &Transportadora{
			CNPJ:   FormatarCNPJ(tr.CNPJ),
			CPF:    FormatarCPF(tr.CPF),
			XNome:  tr.Nome,
			IE:     ie,
			XEnder: tr.End,
			XMun:   tr.Mun,
			UF:     tr.UF,
		}
	}
	if f.Veiculo != nil {
		v := montarVeiculo(*f.Veiculo)
		t.VeicTransp = &v
	}
	for _, r := range f.Reboques {
		t.Reboque = append(t.Reboque, montarVeiculo(r))
	}
	for _, v := range f.Volumes {
		vol := Volume{Esp: v.Especie, Marca: v.Marca, NVol: v.Numeracao}
		if v.Quantidade > 0 {
			vol.QVol = fmt.Sprintf("%d", v.Quantidade)
		}
		if v.PesoLiq > 0 {
			vol.PesoL = fmt.Sprintf("%.3f", v.PesoLiq)
		}
		if v.PesoBruto > 0 {
			vol.PesoB = fmt.Sprintf("%.3f", v.PesoBruto)
		}
		for _, l := range v.Lacres {
			vol.Lacres = append(vol.Lacres, Lacre{NLacre: l})
		}
		t.Vol = append(t.Vol, vol)
	}
	return t
}

func montarVeiculo(v EntradaVeiculo) Veiculo {
	return Veiculo{Placa: formatarPlaca(v.Placa), UF: v.UF, RNTC: v.RNTC}
}

// validarTransporte confere o grupo X e os valores de rateio.
func validarTransporte(e EntradaNFe) error {
	f := e.Frete
	if !modalidadesFrete[modalidadeFrete(f)] {
		return fmt.Errorf("modalidade do frete inválida: %q (esperado 0, 1, 2, 3, 4 ou 9)", f.Modalidade)
	}
	if f.VFrete < 0 || f.VSeg < 0 || f.VOutro < 0 {
		return fmt.Errorf("frete, seguro e outras despesas não podem ser negativos")
	}
	mod := modalidadeFrete(f)
	if tr := f.Transportador; tr != nil {
		if mod == "9" {
			return fmt.Errorf("transportador informado com modalidade de frete 9 (sem transporte) (cStat 845)")
		}
		cnpj, cpf := FormatarCNPJ(tr.CNPJ), FormatarCPF(tr.CPF)
		if cnpj != "" && cpf != "" {
			return fmt.Errorf("transportador com CNPJ e CPF ao mesmo tempo: informe só um")
		}
		if cnpj != "" && !cnpjValido(cnpj) {
			return fmt.Errorf("CNPJ do transportador inválido: %q (cStat 542)", tr.CNPJ)
		}
		if cpf != "" && !cpfValido(cpf) {
			return fmt.Errorf("CPF do transportador inválido: %q (cStat 543)", tr.CPF)
		}
		if FormatarIE(tr.IE) != "" && tr.UF == "" {
			return fmt.Errorf("transportador com inscrição estadual mas sem UF (cStat 559)")
		}
		if tr.UF != "" {
			if _, ok := EstadoCodigo[tr.UF]; !ok {
				return fmt.Errorf("UF do transportador inválida: %q", tr.UF)
			}
		}
		if mod == "3" && (cnpj != "" || cpf != "") {
			emit := FormatarCNPJ(e.Emitente.CNPJ)
			if cpf != "" || len(cnpj) < 8 || len(emit) < 8 || cnpj[:8] != emit[:8] {
				return fmt.Errorf("frete por conta do remetente (modalidade 3) exige que o transportador seja o próprio emitente (mesmo CNPJ base) (cStat 846)")
			}
		}
	}
	if f.Veiculo != nil || len(f.Reboques) > 0 {
		if e.Dest.End.UF != "" && e.Emitente.End.UF != e.Dest.End.UF {
			return fmt.Errorf("veículo/reboque não pode ser informado em operação interestadual (cStat 868)")
		}
	}
	if len(f.Reboques) > 5 {
		return fmt.Errorf("no máximo 5 reboques (informados: %d)", len(f.Reboques))
	}
	if f.Veiculo != nil {
		if err := validarVeiculo("veículo", *f.Veiculo); err != nil {
			return err
		}
	}
	for i, r := range f.Reboques {
		if err := validarVeiculo(fmt.Sprintf("reboque %d", i+1), r); err != nil {
			return err
		}
	}
	for i, v := range f.Volumes {
		if v.Quantidade < 0 || v.PesoLiq < 0 || v.PesoBruto < 0 {
			return fmt.Errorf("volume %d com quantidade ou peso negativo", i+1)
		}
		for _, l := range v.Lacres {
			if strings.TrimSpace(l) == "" {
				return fmt.Errorf("volume %d com número de lacre vazio", i+1)
			}
		}
	}
	return nil
}

func validarVeiculo(rotulo string, v EntradaVeiculo) error {
	if !rePlaca.MatchString(formatarPlaca(v.Placa)) {
		return fmt.Errorf("placa do %s inválida: %q (ex: ABC1D23)", rotulo, v.Placa)
	}
	if v.UF != "" {
		if _, ok := EstadoCodigo[v.UF]; !ok {
			return fmt.Errorf("UF do %s inválida: %q", rotulo, v.UF)
		}
	}
	if len(v.RNTC) > 20 {
		return fmt.Errorf("RNTC do %s com mais de 20 caracteres", rotulo)
	}
	return nil
}

// cnpjValido confere os dois dígitos verificadores, inclusive no CNPJ
// alfanumérico (valor do caractere = código ASCII - 48).
func cnpjValido(c string) bool {
	if len(c) != 14 {
		return false
	}
	val := func(b byte) int { return int(b) - '0' }
	for i := 0; i < 12; i++ {
		if !((c[i] >= '0' && c[i] <= '9') || (c[i] >= 'A' && c[i] <= 'Z')) {
			return false
		}
	}
	if c[12] < '0' || c[12] > '9' || c[13] < '0' || c[13] > '9' {
		return false
	}
	if strings.Count(c, string(c[0])) == 14 {
		return false
	}
	dv := func(n int) int {
		soma, peso := 0, 2
		for i := n - 1; i >= 0; i-- {
			soma += val(c[i]) * peso
			peso++
			if peso > 9 {
				peso = 2
			}
		}
		r := soma % 11
		if r < 2 {
			return 0
		}
		return 11 - r
	}
	return dv(12) == val(c[12]) && dv(13) == val(c[13])
}

func cpfValido(c string) bool {
	if len(c) != 11 || strings.Count(c, string(c[0])) == 11 {
		return false
	}
	dv := func(n int) int {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(c[i]-'0') * (n + 1 - i)
		}
		r := soma * 10 % 11
		if r == 10 {
			return 0
		}
		return r
	}
	return dv(9) == int(c[9]-'0') && dv(10) == int(c[10]-'0')
}
