package builder

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"
)

// EstadoCodigo mapeia UF → código IBGE (cUF) usado na chave de acesso.
var EstadoCodigo = map[string]string{
	"AC": "12", "AL": "27", "AP": "16", "AM": "13",
	"BA": "29", "CE": "23", "DF": "53", "ES": "32",
	"GO": "52", "MA": "21", "MT": "51", "MS": "50",
	"MG": "31", "PA": "15", "PB": "25", "PR": "41",
	"PE": "26", "PI": "22", "RJ": "33", "RN": "24",
	"RS": "43", "RO": "11", "RR": "14", "SC": "42",
	"SP": "35", "SE": "28", "TO": "17",
}

// ChaveAcesso representa a chave de 44 dígitos de uma NF-e.
// Estrutura: cUF(2) + AAMM(4) + CNPJ(14) + mod(2) + serie(3) + nNF(9) + tpEmis(1) + cNF(8) + cDV(1)
type ChaveAcesso struct {
	CUF    string // 2 dígitos: código do estado
	AAMM   string // 4 dígitos: ano-mês de emissão
	CNPJ   string // 14 dígitos: CNPJ do emitente
	Mod    string // 2 dígitos: 55 ou 65
	Serie  string // 3 dígitos
	NNF    string // 9 dígitos: número da nota
	TpEmis string // 1 dígito: tipo de emissão
	CNF    string // 8 dígitos: código numérico aleatório
	CDV    string // 1 dígito: dígito verificador
}

// NovaChave gera uma ChaveAcesso com cNF aleatório e calcula o cDV.
// mod deve ser "55" (NF-e) ou "65" (NFC-e); se vazio assume "55".
func NovaChave(uf, cnpj, serie, nNF, tpEmis, mod string, dhEmi time.Time) ChaveAcesso {
	cuf := EstadoCodigo[uf]
	if cuf == "" {
		cuf = "99"
	}
	aamm := dhEmi.Format("0601")
	cnf := fmt.Sprintf("%08d", gerarCNF())
	if mod == "" {
		mod = ModeloNFe
	}

	serieF := fmt.Sprintf("%03s", serie)
	nnfF := fmt.Sprintf("%09s", nNF)

	base := cuf + aamm + cnpj + mod + serieF + nnfF + tpEmis + cnf
	cdv := calcularDV(base)

	return ChaveAcesso{
		CUF: cuf, AAMM: aamm, CNPJ: cnpj,
		Mod: mod, Serie: serieF, NNF: nnfF,
		TpEmis: tpEmis, CNF: cnf, CDV: cdv,
	}
}

// String retorna a chave como string de 44 dígitos.
func (c ChaveAcesso) String() string {
	return c.CUF + c.AAMM + c.CNPJ + c.Mod + c.Serie + c.NNF + c.TpEmis + c.CNF + c.CDV
}

// ID retorna o atributo Id do infNFe: "NFe" + chave44.
func (c ChaveAcesso) ID() string {
	return "NFe" + c.String()
}

// gerarCNF gera o código numérico aleatório (cNF) da chave usando crypto/rand.
// O cNF tem 8 dígitos (1–99999999).
func gerarCNF() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// fallback improvável: usa unix nano truncado (truncamento explícito, sem overflow)
		return uint32(uint64(time.Now().UnixNano())%99999999) + 1
	}
	return binary.BigEndian.Uint32(b[:])%99999999 + 1
}

// calcularDV calcula o dígito verificador (módulo 11) da chave de 43 dígitos.
func calcularDV(base43 string) string {
	pesos := []int{2, 3, 4, 5, 6, 7, 8, 9}
	soma := 0
	j := 0
	for i := len(base43) - 1; i >= 0; i-- {
		d, _ := strconv.Atoi(string(base43[i]))
		soma += d * pesos[j%8]
		j++
	}
	resto := soma % 11
	if resto < 2 {
		return "0"
	}
	return strconv.Itoa(11 - resto)
}

// FormatarCNPJ remove pontuação de um CNPJ e retorna só os 14 dígitos.
func FormatarCNPJ(cnpj string) string {
	out := make([]byte, 0, 14)
	for _, c := range cnpj {
		if c >= '0' && c <= '9' {
			out = append(out, byte(c))
		}
	}
	return string(out)
}

// FormatarCEP remove traço e retorna só os 8 dígitos.
func FormatarCEP(cep string) string {
	out := make([]byte, 0, 8)
	for _, c := range cep {
		if c >= '0' && c <= '9' {
			out = append(out, byte(c))
		}
	}
	return string(out)
}
