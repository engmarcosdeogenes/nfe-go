package sefaz

import (
	"regexp"
	"testing"
	"time"
)

// TestIDInutilizacaoBateSchema cobre bug real (cStat=215): o Id do infInut
// tinha CNPJ/ano trocados e série sem zeros à esquerda, violando o pattern
// ID[0-9]{4}[0-9A-Z]{12}[0-9]{25} do leiauteInutNFe_v4.00.xsd.
func TestIDInutilizacaoBateSchema(t *testing.T) {
	id := idInutilizacao("52", "26", "34152609000106", "55", "1", "905", "905")

	const esperado = "ID52263415260900010655001000000905000000905"
	if id != esperado {
		t.Fatalf("id = %q\nesperado %q", id, esperado)
	}
	if !regexp.MustCompile(`^ID[0-9]{4}[0-9A-Z]{12}[0-9]{25}$`).MatchString(id) {
		t.Errorf("id %q nao bate o pattern do XSD", id)
	}
}

// TestAgoraBrasiliaTemOffsetReal cobre o cStat=578: o layout "-03:00" é texto
// literal no Go, então UTC formatado com ele vira um horário 3h no futuro.
func TestAgoraBrasiliaTemOffsetReal(t *testing.T) {
	got := agoraBrasilia()
	parsed, err := time.Parse("2006-01-02T15:04:05-07:00", got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	if delta := time.Since(parsed); delta < -time.Minute || delta > time.Minute {
		t.Errorf("dhEvento %q está a %v do agora real", got, delta)
	}
}
