package at

import (
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

// soapFaults is the ONLY table of AT SOAP faultcodes (series + transport), as observed on
// the AT test endpoints 2026-11-02. Unknown faults are treated as unavailable.
var soapFaults = map[string]codeInfo{
	"1199": {ErrTransportUnavailable, "check AT sub-user/password, its WSE/WDT profile, and the PC clock"},
	"118":  {ErrTransportUnavailable, "check fiscal_at_password_pubkey is the current ChavePublicaAT"},
	"33":   {ErrTransportRejected, "request does not match the AT schema"},
}

// transportReturnCodes is the ONLY table of non-zero envioDocumentoTransporte ReturnCodes
// with known meaning. Unknown non-zero codes are rejections without a hint.
var transportReturnCodes = map[int]codeInfo{
	54: {ErrTransportRejected, "AT sub-user lacks the WDT transport-documents profile"},
}

func faultError(code, msg string) *TransportError {
	info, ok := soapFaults[code]
	if !ok {
		info.kind = ErrTransportUnavailable
	}
	return &TransportError{Kind: info.kind, Code: code, Message: msg, Hint: info.hint}
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

// faultHint is the series client's view of soapFaults (" (hint)" or "").
func faultHint(code string) string {
	if h := soapFaults[code].hint; h != "" {
		return " (" + h + ")"
	}
	return ""
}
