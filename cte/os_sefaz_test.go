package cte

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
)

const chaveOS = "52261011222333000181670010000000771123456780"

func cteOSAssinadoSF() []byte {
	return []byte(`<?xml version="1.0"?><CTeOS xmlns="http://www.portalfiscal.inf.br/cte" versao="4.00"><infCte Id="CTe` + chaveOS + `" versao="4.00"><ide><tpAmb>2</tpAmb></ide></infCte></CTeOS>`)
}

func TestSefaz_AutorizarOS(t *testing.T) {
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_os_autorizado.xml"))
	r, err := cl.AutorizarOS(context.Background(), cteOSAssinadoSF())
	if err != nil {
		t.Fatal(err)
	}
	if v.path != "/ws/CTeRecepcaoOSV4/CTeRecepcaoOSV4.asmx" {
		t.Errorf("path %s", v.path)
	}
	wantAct := "http://www.portalfiscal.inf.br/cte/wsdl/CTeRecepcaoOSV4/cteRecepcaoOS"
	if v.action != wantAct || !strings.Contains(v.ct, `action="`+wantAct+`"`) {
		t.Errorf("action/ct: %q %q", v.action, v.ct)
	}
	zb, err := base64.StdEncoding.DecodeString(dadosMsgSF(t, v.corpo))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zb))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	enviado, _ := io.ReadAll(zr)
	if !strings.HasPrefix(string(enviado), "<CTeOS ") || strings.Contains(string(enviado), "<?xml") {
		t.Errorf("CTeOS enviado: %s", enviado)
	}
	if r.CStat != "100" || r.NProt != "926260000000002" || r.ChCTe != chaveOS || r.DhRecbto == "" {
		t.Errorf("%+v", r)
	}
	px := string(r.XMLProc)
	if !strings.HasPrefix(px, `<cteOSProc versao="4.00" xmlns="http://www.portalfiscal.inf.br/cte"><CTeOS `) || !strings.HasSuffix(px, "</protCTe></cteOSProc>") {
		t.Errorf("cteOSProc: %s", px)
	}
	if xml.Unmarshal(r.XMLProc, &struct{ XMLName xml.Name }{}) != nil {
		t.Error("cteOSProc mal formado")
	}
	// o retorno de teste respeita retCTeOS_v4.00.xsd
	valSF(t, "retCTeOS_v4.00.xsd", sfPrimeiro(fixtureSF(t, "ret_os_autorizado.xml"), "retCTeOS"), false)
	valSF(t, "retCTeOS_v4.00.xsd", sfPrimeiro(fixtureSF(t, "ret_os_rejeicao.xml"), "retCTeOS"), false)
}

func TestSefaz_AutorizarOSRejeicaoEErros(t *testing.T) {
	cl, _ := srvSF(t, "SP", 200, fixtureSF(t, "ret_os_rejeicao.xml"))
	r, err := cl.AutorizarOS(context.Background(), cteOSAssinadoSF())
	if err != nil || r.CStat != "539" || r.XMLProc != nil || !strings.Contains(r.XMotivo, "Duplicidade") {
		t.Errorf("%+v %v", r, err)
	}
	cl, _ = srvSF(t, "SP", 500, []byte("boom"))
	if _, err := cl.AutorizarOS(context.Background(), cteOSAssinadoSF()); err == nil {
		t.Error("HTTP 500")
	}
	// retCTe do 57 não é resposta do OS
	cl, _ = srvSF(t, "SP", 200, fixtureSF(t, "ret_autorizado.xml"))
	if _, err := cl.AutorizarOS(context.Background(), cteOSAssinadoSF()); err == nil {
		t.Error("resposta sem retCTeOS deveria dar erro")
	}
}

// Os eventos do 57 servem ao 67: mesmo eventoCTe, chave com modelo 67.
func TestXSD_OS_EventoCancelamento(t *testing.T) {
	c := certSF(t)
	cl, v := srvSF(t, "GO", 200, fixtureSF(t, "ret_evento.xml"))
	cl.agora = func() time.Time { return time.Date(2026, 10, 26, 14, 0, 0, 0, time.UTC) }
	r, err := cl.Cancelar(context.Background(), c, chaveOS, nProtSF, "Cancelamento por erro de digitacao")
	if err != nil || r.CStat != "135" {
		t.Fatalf("%+v %v", r, err)
	}
	ev := sfPrimeiro(v.corpo, "eventoCTe")
	valSF(t, "eventoCTe_v4.00.xsd", ev, false)
	valSF(t, "evCancCTe_v4.00.xsd", ev, true)
	if !strings.Contains(string(ev), `Id="ID110111`+chaveOS+`001"`) {
		t.Errorf("Id do evento: %s", ev)
	}
}
