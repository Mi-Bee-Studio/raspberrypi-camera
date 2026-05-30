package onvif

import "encoding/xml"

// SOAPEnvelope represents a SOAP 1.2 envelope.
type SOAPEnvelope struct {
	XMLName xml.Name   `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Header  SOAPHeader `xml:"Header"`
	Body    SOAPBody   `xml:"Body"`
}

// SOAPHeader contains optional WS-Security elements.
type SOAPHeader struct {
	Security *WSSecurity `xml:"Security"`
}

// WSSecurity holds WS-Security header elements.
type WSSecurity struct {
	XMLName       xml.Name       `xml:"Security"`
	UsernameToken *UsernameToken `xml:"UsernameToken"`
}

// UsernameToken carries WS-UsernameToken credentials.
type UsernameToken struct {
	Username string `xml:"Username"`
	Password string `xml:"Password"`
	Nonce    string `xml:"Nonce"`
	Created  string `xml:"Created"`
}

// SOAPBody wraps the action-specific payload.
type SOAPBody struct {
	XMLName xml.Name
	RawXML  string `xml:",innerxml"`
}

// SOAPFault represents a SOAP 1.2 fault.
type SOAPFault struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2003/05/soap-envelope Fault"`
	Code    SOAPFaultCode   `xml:"Code"`
	Reason  SOAPFaultReason `xml:"Reason"`
}

// SOAPFaultCode holds the fault code value.
type SOAPFaultCode struct {
	Value string `xml:"Value"`
}

// SOAPFaultReason holds the human-readable fault reason.
type SOAPFaultReason struct {
	Text string `xml:"Text"`
}

// soapFaultEnvelope wraps a SOAPFault inside a SOAPEnvelope for marshalling.
type soapFaultEnvelope struct {
	XMLName xml.Name   `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Header  SOAPHeader `xml:"Header"`
	Body    soapFaultBody `xml:"Body"`
}

type soapFaultBody struct {
	Fault SOAPFault `xml:"Fault"`
}
