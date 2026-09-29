package danfe

import "testing"

// Chave com CNPJ alfanumérico tem letras: o Code 128 tem que trocar pro set B.
func TestBarcodeCode128_ChaveAlfanumerica(t *testing.T) {
	if _, err := gerarBarcodeCode128("35260912ABC34501DE35550010000000421123456784"); err != nil {
		t.Fatal(err)
	}
}
