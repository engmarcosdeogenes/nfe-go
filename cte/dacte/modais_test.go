package dacte

import "testing"

// Bloco "Informações específicas do modal" por modal (MOC DACTE: seção exata
// não confirmada). Busca só trechos ASCII: o PDF grava acentos em cp1252.
func TestModaisEspecificos(t *testing.T) {
	casos := []struct {
		arq string
		tem []string
	}{
		{"cte_modal_aereo.xml", []string{"modal a", "123456789", "12345678901", "01/11/2026", "10X20X30", "02, 99", "AB12", "99,99", "1234", "2 caixas", "10,5000", "Modal: 02"}},
		{"cte_modal_aquaviario.xml", []string{"modal aquavi", "100,00", "AFRMM", "MV NAVIO ALFA", "BALSA B1", "123", "IRIN123", "ABCU1234567", "LAC1", "Modal: 03"}},
		{"cte_modal_ferroviario.xml", []string{"modal ferrovi", "FLX001", "250,75", "FERROVIA BETA", "11.222.333/0001-81", "Sao Paulo/SP", "Modal: 04"}},
		{"cte_modal_dutoviario.xml", []string{"modal dutovi", "0,500000", "01/10/2026", "31/10/2026", "PE01", "PS01", "CT-2026", "Modal: 05"}},
		{"cte_modal_multimodal.xml", []string{"modal multimodal", "COTM123", "1 - Negoci", "SEGURADORA Z", "AP1", "AV1", "Modal: 06"}},
	}
	for _, c := range casos {
		t.Run(c.arq, func(t *testing.T) {
			tem(t, gerarBytes(t, ler(t, c.arq), Opcoes{}), c.tem...)
		})
	}
}
