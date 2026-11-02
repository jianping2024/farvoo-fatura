//go:build at_series_live

// Live probe against AT Séries TEST endpoint (A1.1). Read-only consultarSeries.
// Default CI never builds this file. Run:
//
//	go test -tags=at_series_live -run TestLiveConsultSeries -v ./internal/fiscal/at
//
// Overrides (all optional): FISCAL_AT_CLIENT_CERT / _PASS, FISCAL_AT_PASSWORD_PUBKEY,
// FISCAL_AT_SERIES_URL, AT_LIVE_USER / AT_LIVE_PASS (default: manual test sub-user).
package at

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const defaultSeriesTestURL = "https://servicos.portaldasfinancas.gov.pt:722/SeriesWSService"

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func TestLiveConsultSeries(t *testing.T) {
	certDir := filepath.Join("testdata", "transport", "certificados", "current")
	m := LiveMaterialsFromEnv()
	certPath := envOr("FISCAL_AT_CLIENT_CERT", filepath.Join(certDir, "TesteWebservices.pfx"))
	certPass := m.ClientCertPass
	if certPass == "" {
		certPass = "TESTEwebservice"
	}
	pubPath := envOr("FISCAL_AT_PASSWORD_PUBKEY", filepath.Join(certDir, "ChavePublicaAT.cer"))
	url := envOr("FISCAL_AT_SERIES_URL", defaultSeriesTestURL)
	user := envOr("AT_LIVE_USER", "599999993/37")
	pass := envOr("AT_LIVE_PASS", "testes1234")

	tlsCfg, err := LoadClientTLS(certPath, certPass)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := LoadPasswordPublicKey(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	mat, err := EncryptATAuth(pass, pub, EncryptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := BuildUsernameTokenXML(user, mat)
	if err != nil {
		t.Fatal(err)
	}
	body := `<at:consultarSeries xmlns:at="http://at.gov.pt/"/>`
	envelope := WrapSOAPEnvelope(BuildSecurityHeaderXML(tok), body)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(envelope))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `""`)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}

	t.Logf("cert=%s url=%s user=%s created=%s", filepath.Base(certPath), url, user, mat.CreatedUTC)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	t.Logf("HTTP %d\n%s", resp.StatusCode, raw)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(raw), "consultarSeriesResp") {
		t.Fatalf("not a consultarSeries business response")
	}
}
