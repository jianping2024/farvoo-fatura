package at

import (
	"context"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadLiveMaterials fills nil HTTP (mTLS) / PubKey from LiveMaterialsFromEnv — ONLY
// material loader shared by series (SOAPClient) and transport (SOAPTransportClient).
func loadLiveMaterials(env Env, hc **http.Client, pub **rsa.PublicKey) error {
	if *hc != nil && *pub != nil {
		return nil
	}
	m := LiveMaterialsFromEnv()
	if miss := m.MissingRequired(); len(miss) > 0 {
		return fmt.Errorf("at: AT %s materials missing in config.json: %s", env, strings.Join(miss, ", "))
	}
	if *hc == nil {
		tlsCfg, err := LoadClientTLS(m.ClientCertPath, m.ClientCertPass)
		if err != nil {
			return err
		}
		*hc = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: tlsCfg}}
	}
	if *pub == nil {
		k, err := LoadPasswordPublicKey(m.PasswordPubKeyPath)
		if err != nil {
			return err
		}
		*pub = k
	}
	return nil
}

// postSOAP sends one AT SOAP request with a fresh encrypted UsernameToken (ONLY live POST).
// Errors never include the password.
func postSOAP(ctx context.Context, hc *http.Client, url, soapAction string, pub *rsa.PublicKey, now time.Time,
	username, password, body string, trace func(op string, status int, raw []byte)) (int, []byte, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return 0, nil, fmt.Errorf("at: credentials required")
	}
	mat, err := EncryptATAuth(password, pub, EncryptOptions{Now: now})
	if err != nil {
		return 0, nil, err
	}
	tok, err := BuildUsernameTokenXML(username, mat)
	if err != nil {
		return 0, nil, err
	}
	envelope := WrapSOAPEnvelope(BuildSecurityHeaderXML(tok), body)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(envelope))
	if err != nil {
		return 0, nil, err
	}
	hreq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	hreq.Header.Set("SOAPAction", `"`+soapAction+`"`)
	hresp, err := hc.Do(hreq)
	if err != nil {
		return 0, nil, fmt.Errorf("at: transport: %w", err)
	}
	defer hresp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(hresp.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("at: read response: %w", err)
	}
	dumpSOAPExchange(url, soapAction, hresp.StatusCode, body, raw)
	if trace != nil {
		op := body
		if i := strings.IndexAny(body, " >"); i > 0 {
			op = body[:i]
		}
		trace(op, hresp.StatusCode, raw)
	}
	return hresp.StatusCode, raw, nil
}

// dumpSOAPExchange writes the business body + AT raw response when FISCAL_AT_SOAP_DUMP
// is a directory. Never writes the UsernameToken / password. Diagnosis only.
func dumpSOAPExchange(url, soapAction string, status int, body string, raw []byte) {
	dir := strings.TrimSpace(os.Getenv("FISCAL_AT_SOAP_DUMP"))
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := filepath.Join(dir, stamp)
	_ = os.WriteFile(base+"-request-body.xml", []byte(body), 0o600)
	_ = os.WriteFile(base+"-response.xml", raw, 0o600)
	meta := fmt.Sprintf("url=%s\nsoap_action=%s\nhttp_status=%d\nrequest_body_bytes=%d\nresponse_bytes=%d\n",
		url, soapAction, status, len(body), len(raw))
	_ = os.WriteFile(base+"-meta.txt", []byte(meta), 0o600)
	// also latest aliases for quick open
	_ = os.WriteFile(filepath.Join(dir, "latest-request-body.xml"), []byte(body), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "latest-response.xml"), raw, 0o600)
	_ = os.WriteFile(filepath.Join(dir, "latest-meta.txt"), []byte(meta), 0o600)
}
