package sefaz

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// nsNFe é o namespace dos documentos da NF-e.
const nsNFe = "http://www.portalfiscal.inf.br/nfe"

// MontarProcEventoNFe junta o evento assinado que foi enviado e o retEvento devolvido pela SEFAZ no
// documento de distribuição procEventoNFe versao 1.00 (MOC 7.0, leiaute procEventoNFe_v1.00.xsd):
// <procEventoNFe versao="1.00" xmlns="..."><evento>...</evento><retEvento>...</retEvento></procEventoNFe>.
// É esse arquivo que vale como prova do evento (CC-e, cancelamento) para o contador e o destinatário.
func MontarProcEventoNFe(eventoAssinado, retEvento []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`<procEventoNFe versao="1.00" xmlns="` + nsNFe + `">`)
	b.Write(semDeclaracaoXML(eventoAssinado))
	b.Write(retEvento)
	b.WriteString(`</procEventoNFe>`)
	return b.Bytes()
}

// ExtrairProcEventos devolve, crus e na ordem em que vieram, os procEventoNFe que a SEFAZ manda dentro
// do retConsSitNFe (consulta de protocolo): cancelamento, CC-e e demais eventos vinculados à nota.
// Aceita o retConsSitNFe sozinho ou dentro do Body do SOAP.
func ExtrairProcEventos(retConsSitNFe []byte) ([][]byte, error) {
	return extrairElementos(retConsSitNFe, "procEventoNFe")
}

// ProcEvento são os campos de um procEventoNFe que interessam para imprimir ou casar o evento.
// TpEvento/NSeqEvento/DhEvento/textos vêm do evento enviado; CStat/NProt/DhRegEvento do retEvento.
type ProcEvento struct {
	ChNFe       string
	CNPJ        string // autor do evento
	TpAmb       string
	TpEvento    string
	NSeqEvento  string
	DhEvento    string
	DescEvento  string
	XCorrecao   string // CC-e
	XCondUso    string // CC-e
	XJust       string // cancelamento
	CStat       string
	XMotivo     string
	NProt       string
	DhRegEvento string
}

// LerProcEvento lê um procEventoNFe (ou só o par evento+retEvento dentro de qualquer raiz).
func LerProcEvento(proc []byte) (*ProcEvento, error) {
	var x struct {
		Evento struct {
			Inf struct {
				ChNFe      string `xml:"chNFe"`
				CNPJ       string `xml:"CNPJ"`
				TpAmb      string `xml:"tpAmb"`
				DhEvento   string `xml:"dhEvento"`
				TpEvento   string `xml:"tpEvento"`
				NSeqEvento string `xml:"nSeqEvento"`
				Det        struct {
					DescEvento string `xml:"descEvento"`
					XCorrecao  string `xml:"xCorrecao"`
					XCondUso   string `xml:"xCondUso"`
					XJust      string `xml:"xJust"`
				} `xml:"detEvento"`
			} `xml:"infEvento"`
		} `xml:"evento"`
		Ret struct {
			Inf struct {
				CStat       string `xml:"cStat"`
				XMotivo     string `xml:"xMotivo"`
				NProt       string `xml:"nProt"`
				DhRegEvento string `xml:"dhRegEvento"`
			} `xml:"infEvento"`
		} `xml:"retEvento"`
	}
	if err := xml.Unmarshal(proc, &x); err != nil {
		return nil, fmt.Errorf("sefaz: ler procEventoNFe: %w", err)
	}
	e, r := x.Evento.Inf, x.Ret.Inf
	if e.TpEvento == "" {
		return nil, fmt.Errorf("sefaz: procEventoNFe sem evento/infEvento")
	}
	return &ProcEvento{
		ChNFe: e.ChNFe, CNPJ: e.CNPJ, TpAmb: e.TpAmb, TpEvento: e.TpEvento, NSeqEvento: e.NSeqEvento,
		DhEvento: e.DhEvento, DescEvento: e.Det.DescEvento, XCorrecao: e.Det.XCorrecao,
		XCondUso: e.Det.XCondUso, XJust: e.Det.XJust,
		CStat: r.CStat, XMotivo: r.XMotivo, NProt: r.NProt, DhRegEvento: r.DhRegEvento,
	}, nil
}

// extrairElementos devolve os bytes crus de cada elemento <nome> (qualquer prefixo) de doc, sem
// descer dentro de um já achado. Elemento sem prefixo e sem xmlns próprio recebe o xmlns da NF-e, que
// ele herdava do pai — sem isso o pedaço solto deixa de valer contra o XSD.
// shortcut: elemento com prefixo (<ns2:retEvento>) sai como veio, sem redeclarar o namespace; a SEFAZ
// de NF-e 4.00 responde sem prefixo. Se aparecer UF com prefixo, reserializar aqui.
func extrairElementos(doc []byte, nome string) ([][]byte, error) {
	d := xml.NewDecoder(bytes.NewReader(doc))
	var out [][]byte
	inicio, prof := int64(-1), 0
	for {
		antes := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("sefaz: ler <%s>: %w", nome, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if inicio >= 0 {
				prof++
			} else if t.Name.Local == nome {
				inicio, prof = antes, 1
			}
		case xml.EndElement:
			if inicio < 0 {
				continue
			}
			prof--
			if prof == 0 {
				out = append(out, comXmlnsNFe(doc[inicio:d.InputOffset()], nome))
				inicio = -1
			}
		}
	}
}

func comXmlnsNFe(el []byte, nome string) []byte {
	abre := []byte("<" + nome)
	if !bytes.HasPrefix(el, abre) {
		return append([]byte(nil), el...)
	}
	fim := bytes.IndexByte(el, '>')
	if fim < 0 || bytes.Contains(el[:fim], []byte("xmlns=")) {
		return append([]byte(nil), el...)
	}
	var b bytes.Buffer
	b.Write(abre)
	b.WriteString(` xmlns="` + nsNFe + `"`)
	b.Write(el[len(abre):])
	return b.Bytes()
}

func semDeclaracaoXML(x []byte) []byte {
	x = bytes.TrimSpace(x)
	if bytes.HasPrefix(x, []byte("<?xml")) {
		if i := bytes.Index(x, []byte("?>")); i >= 0 {
			return bytes.TrimSpace(x[i+2:])
		}
	}
	return x
}
