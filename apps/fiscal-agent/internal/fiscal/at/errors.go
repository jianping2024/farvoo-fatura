package at

import (
	"encoding/xml"
	"errors"
	"strconv"
	"strings"
)

// Transport error classes. Callers branch with errors.Is — never on message text.
var (
	// ErrTransportInvalid: the document failed local checks; nothing was sent to AT.
	ErrTransportInvalid = errors.New("at: transport document invalid")
	// ErrTransportUnavailable: connectivity, configuration or authentication failure (retryable).
	ErrTransportUnavailable = errors.New("at: transport unavailable")
	// ErrTransportRejected: AT processed the request and refused it (fix the data; do not retry).
	ErrTransportRejected = errors.New("at: transport rejected")
)

// TransportError is a classified AT transport failure; Unwrap yields its class.
type TransportError struct {
	Kind    error  // ErrTransportUnavailable | ErrTransportRejected
	Code    string // SOAP faultcode or ReturnCode; "" for transport-level failures
	Message string
	Hint    string // operator guidance from codeTable
}

func (e *TransportError) Error() string {
	var b strings.Builder
	b.WriteString(e.Kind.Error())
	if e.Code != "" {
		b.WriteString(": " + e.Code)
	}
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	if e.Hint != "" {
		b.WriteString(" (" + e.Hint + ")")
	}
	return b.String()
}

func (e *TransportError) Unwrap() error { return e.Kind }

type codeInfo struct {
	kind error
	hint string
}

// soapFaults is the ONLY table of AT SOAP faultcodes raised by the called service (series +
// transport), as observed on the AT test endpoints 2026-11-02. Unknown faults are
// unavailable. Faults from the authentication layer bypass this table (see classifyFault).
var soapFaults = map[string]codeInfo{
	"1199": {ErrTransportUnavailable, "check AT sub-user/password, its WSE/WDT profile, and the PC clock"},
	"118":  {ErrTransportUnavailable, "check fiscal_at_password_pubkey is the current ChavePublicaAT"},
	"33":   {ErrTransportRejected, "request does not match the AT schema"},
}

// transportReturnCodes is the ONLY table of non-zero envioDocumentoTransporte ReturnCodes
// with known meaning. Unknown non-zero codes are rejections without a hint.
// Hint "movement_start_past" is the sole machine key for ReturnCode -100 (start before now).
var transportReturnCodes = map[int]codeInfo{
	54:   {ErrTransportRejected, "AT sub-user lacks the WDT transport-documents profile"},
	-100: {ErrTransportRejected, "movement_start_past"},
}

// atAuthNSPrefix qualifies <detail> children of faults raised by AT's credential gateway
// (ValidateCredencialService), e.g. "http://autenticacao.portaldasfinancas.gov.pt/services/…".
const atAuthNSPrefix = "http://autenticacao.portaldasfinancas.gov.pt/"

const authLayerHint = "AT authentication service failed before reading the document; retry later"

// soapFault is the ONLY decoded SOAP 1.1 Fault (series + transport envelopes).
type soapFault struct {
	Code   string `xml:"faultcode"`
	String string `xml:"faultstring"`
	Detail struct {
		Items []struct {
			XMLName xml.Name
		} `xml:",any"`
	} `xml:"detail"`
}

// fromAuthLayer reports a fault raised by the AT authentication gateway, identified by the
// namespace of its <detail> elements (structure, not message text).
func (f *soapFault) fromAuthLayer() bool {
	for _, it := range f.Detail.Items {
		if strings.HasPrefix(it.XMLName.Space, atAuthNSPrefix) {
			return true
		}
	}
	return false
}

// classifyFault is the ONLY SOAP fault classifier. The authentication layer never judges the
// business payload, so any fault it raises is unavailable (2026-11-02: code 33 "mismatched
// tag, expected hr" when its credential backend answered with an HTML page). Service faults
// use soapFaults.
func classifyFault(f *soapFault) codeInfo {
	info, known := soapFaults[strings.TrimSpace(f.Code)]
	if f.fromAuthLayer() {
		if !known || info.kind != ErrTransportUnavailable {
			info.hint = authLayerHint
		}
		info.kind = ErrTransportUnavailable
		return info
	}
	if !known {
		info.kind = ErrTransportUnavailable
	}
	return info
}

func faultError(f *soapFault) *TransportError {
	info := classifyFault(f)
	return &TransportError{Kind: info.kind, Code: strings.TrimSpace(f.Code), Message: strings.TrimSpace(f.String), Hint: info.hint}
}

func returnCodeError(code int, msg string) *TransportError {
	info, ok := transportReturnCodes[code]
	if !ok {
		info.kind = ErrTransportRejected
	}
	return &TransportError{Kind: info.kind, Code: strconv.Itoa(code), Message: msg, Hint: info.hint}
}

func unavailable(msg string) *TransportError {
	return &TransportError{Kind: ErrTransportUnavailable, Message: msg}
}

// faultHint is the series client's view of classifyFault (" (hint)" or "").
func faultHint(f *soapFault) string {
	if h := classifyFault(f).hint; h != "" {
		return " (" + h + ")"
	}
	return ""
}
