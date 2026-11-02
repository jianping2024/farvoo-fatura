package at

import (
	"fmt"
	"strings"
)

const (
	wsseNS = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd"
	wsuNS  = "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"
)

// BuildUsernameTokenXML is the ONLY UsernameToken XML builder for AT SOAP headers.
// Expects AuthMaterial from EncryptATAuth; does not encrypt.
func BuildUsernameTokenXML(username string, mat *AuthMaterial) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", fmt.Errorf("at: username required")
	}
	if mat == nil || mat.PasswordB64 == "" || mat.NonceB64 == "" || mat.CreatedB64 == "" {
		return "", fmt.Errorf("at: auth material incomplete")
	}
	return fmt.Sprintf(
		`<wss:UsernameToken>`+
			`<wss:Username>%s</wss:Username>`+
			`<wss:Password>%s</wss:Password>`+
			`<wss:Nonce>%s</wss:Nonce>`+
			`<wss:Created>%s</wss:Created>`+
			`</wss:UsernameToken>`,
		xmlEscape(username),
		xmlEscape(mat.PasswordB64),
		xmlEscape(mat.NonceB64),
		xmlEscape(mat.CreatedB64),
	), nil
}

// BuildSecurityHeaderXML wraps a UsernameToken in wsse:Security.
func BuildSecurityHeaderXML(usernameTokenXML string) string {
	return fmt.Sprintf(
		`<wss:Security xmlns:wss="%s" xmlns:wsu="%s">%s</wss:Security>`,
		wsseNS, wsuNS, usernameTokenXML,
	)
}

// WrapSOAPEnvelope builds a minimal SOAP 1.1 envelope. bodyInner is raw XML for S:Body.
// ONLY envelope wrapper for AT live clients (A1/A2 will call this).
func WrapSOAPEnvelope(headerInner, bodyInner string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/">`)
	if strings.TrimSpace(headerInner) != "" {
		b.WriteString(`<S:Header>`)
		b.WriteString(headerInner)
		b.WriteString(`</S:Header>`)
	}
	b.WriteString(`<S:Body>`)
	b.WriteString(bodyInner)
	b.WriteString(`</S:Body></S:Envelope>`)
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
		`'`, "&apos;",
	)
	return r.Replace(s)
}
