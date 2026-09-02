package sefaz

import (
	"bytes"
	"fmt"

	"github.com/engmarcosdeogenes/nfe-go/builder"
	"github.com/engmarcosdeogenes/nfe-go/cert"
	"github.com/engmarcosdeogenes/nfe-go/sign"
)

// AutorizarContingencia constrói e assina uma NF-e no modo FS-DA (tpEmis=5)
// sem transmitir à SEFAZ. O XML assinado retornado deve ser armazenado pelo
// emissor e retransmitido via Cliente.Autorizar quando a SEFAZ normalizar.
//
// A entrada deve ter TpEmis="5", DhCont e XJust preenchidos; erros de
// validação ou assinatura são propagados diretamente.
func AutorizarContingencia(e builder.EntradaNFe, c *cert.Certificado) ([]byte, error) {
	if e.TpEmis != "5" {
		return nil, fmt.Errorf("sefaz: AutorizarContingencia exige tpEmis=\"5\" (FS-DA), recebido %q", e.TpEmis)
	}

	xmlBytes, _, err := builder.Build(e)
	if err != nil {
		return nil, fmt.Errorf("sefaz: contingência build: %w", err)
	}

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		return nil, fmt.Errorf("sefaz: contingência assinar: %w", err)
	}

	return assinado, nil
}

// AutorizarContingenciaNFCe monta e assina uma NFC-e em contingência off-line
// (tpEmis=9) sem transmitir. O XML retornado já traz o <infNFeSupl> com o QR
// Code de contingência — que embute o DigestValue da assinatura, por isso só
// pode ser montado depois de assinar.
//
// Guardar e retransmitir via Cliente.Autorizar quando a SEFAZ normalizar,
// com a MESMA chave (2º retorno) e o MESMO XML (a SEFAZ trata reenvio de
// chave já autorizada como cStat=539). A entrada precisa de Mod="65",
// CSC/CSCId, UrlConsultaNFCe, DhCont e XJust (≥15 chars).
func AutorizarContingenciaNFCe(e builder.EntradaNFe, c *cert.Certificado) ([]byte, string, error) {
	e.TpEmis = "9"
	if e.Mod == "" {
		e.Mod = builder.ModeloNFCe
	}
	if e.Mod != builder.ModeloNFCe {
		return nil, "", fmt.Errorf("sefaz: AutorizarContingenciaNFCe exige mod=65 (NFC-e), recebido %q", e.Mod)
	}

	// Build pula o infNFeSupl quando tpEmis=9 (não dá pra montar o QR sem o
	// DigestValue, que ainda não existe).
	xmlBytes, chave, err := builder.Build(e)
	if err != nil {
		return nil, "", fmt.Errorf("sefaz: contingência NFC-e build: %w", err)
	}

	assinado, err := sign.AssinarNFe(xmlBytes, c)
	if err != nil {
		return nil, "", fmt.Errorf("sefaz: contingência NFC-e assinar: %w", err)
	}

	digestB64 := extrairTagXML(assinado, "DigestValue")
	if digestB64 == "" {
		return nil, "", fmt.Errorf("sefaz: DigestValue não encontrado no XML assinado")
	}
	vNF := extrairTagXML(assinado, "vNF")
	if vNF == "" {
		return nil, "", fmt.Errorf("sefaz: vNF não encontrado no XML assinado")
	}

	supl, err := builder.MontarQRCodeContingenciaNFCe(e, chave, vNF, digestB64)
	if err != nil {
		return nil, "", err
	}
	final, err := injetarInfNFeSupl(assinado, supl.XMLFragment())
	if err != nil {
		return nil, "", err
	}
	return final, chave.String(), nil
}

// injetarInfNFeSupl insere o fragmento <infNFeSupl> logo antes de <Signature>
// no XML já assinado. infNFeSupl é filho direto de <NFe> e NÃO é coberto pela
// assinatura (que assina só <infNFe>), então inserir depois é válido.
func injetarInfNFeSupl(xmlBytes []byte, fragmento string) ([]byte, error) {
	marca := []byte("<Signature ")
	idx := bytes.Index(xmlBytes, marca)
	if idx < 0 {
		return nil, fmt.Errorf("sefaz: <Signature> não encontrado pra inserir infNFeSupl")
	}
	out := make([]byte, 0, len(xmlBytes)+len(fragmento))
	out = append(out, xmlBytes[:idx]...)
	out = append(out, fragmento...)
	out = append(out, xmlBytes[idx:]...)
	return out, nil
}

func extrairTagXML(xmlBytes []byte, tag string) string {
	abre := []byte("<" + tag + ">")
	fecha := []byte("</" + tag + ">")
	ini := bytes.Index(xmlBytes, abre)
	if ini < 0 {
		return ""
	}
	ini += len(abre)
	fim := bytes.Index(xmlBytes[ini:], fecha)
	if fim < 0 {
		return ""
	}
	return string(xmlBytes[ini : ini+fim])
}
