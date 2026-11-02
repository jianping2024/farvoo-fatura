//go:build at_series_live

// Live probes against the AT Séries TEST endpoint (A1). Default CI never builds this file.
//
//	# A1.1 read-only connectivity
//	AT_LIVE_USER='519655290/1' AT_LIVE_PASS='…' \
//	  go test -count=1 -tags=at_series_live -run TestLiveConsultSeries -v ./internal/fiscal/at
//	# A1.2 register (writes to AT test; annul only same/next day) — opt-in per series
//	AT_LIVE_USER=… AT_LIVE_PASS=… AT_LIVE_SERIES_FT=FT2026T01 AT_LIVE_SERIES_GR=GR2026T01 \
//	  go test -count=1 -tags=at_series_live -run TestLiveRegisterSeries -v ./internal/fiscal/at
//
// Materials default to the public test pfx + ChavePublicaAT in testdata; override with
// FISCAL_AT_CLIENT_CERT / _PASS / FISCAL_AT_PASSWORD_PUBKEY / FISCAL_AT_SERIES_URL.
package at

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func liveSeriesClient(t *testing.T) (*SOAPClient, string, string) {
	t.Helper()
	certDir, _ := filepath.Abs(filepath.Join("testdata", "transport", "certificados", "current"))
	t.Setenv("FISCAL_AT_CLIENT_CERT", envOr("FISCAL_AT_CLIENT_CERT", filepath.Join(certDir, "TesteWebservices.pfx")))
	t.Setenv("FISCAL_AT_CLIENT_CERT_PASS", envOr("FISCAL_AT_CLIENT_CERT_PASS", "TESTEwebservice"))
	t.Setenv("FISCAL_AT_PASSWORD_PUBKEY", envOr("FISCAL_AT_PASSWORD_PUBKEY", filepath.Join(certDir, "ChavePublicaAT.cer")))
	user, pass := os.Getenv("AT_LIVE_USER"), os.Getenv("AT_LIVE_PASS")
	if user == "" || pass == "" {
		t.Skip("set AT_LIVE_USER / AT_LIVE_PASS (own-NIF sub-user with WSE profile)")
	}
	t.Logf("cert=%s user=%s", filepath.Base(os.Getenv("FISCAL_AT_CLIENT_CERT")), user)
	c := &SOAPClient{Env: EnvTest, Trace: func(op string, status int, raw []byte) {
		t.Logf("%s HTTP %d\n%s", op, status, raw)
	}}
	return c, user, pass
}

func TestLiveConsultSeries(t *testing.T) {
	c, user, pass := liveSeriesClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.consult(ctx, user, pass, "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("AT %d: %s", resp.Result.Cod, resp.Result.Msg)
	for _, in := range resp.Info {
		t.Logf("  %s %s estado=%s cod=%s", in.TipoDoc, in.Serie, in.Estado, in.CodValidacaoSerie)
	}
}

func TestLiveRegisterSeries(t *testing.T) {
	c, user, pass := liveSeriesClient(t)
	ran := false
	for _, doc := range []string{"FT", "GR"} {
		serie := strings.TrimSpace(os.Getenv("AT_LIVE_SERIES_" + doc))
		if serie == "" {
			continue
		}
		ran = true
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		res, err := c.RegisterSeries(ctx, RegisterRequest{Username: user, Password: pass, SeriesCode: serie, DocumentType: doc})
		cancel()
		if err != nil {
			t.Errorf("%s %s: %v", doc, serie, err)
			continue
		}
		t.Logf("%s %s → validation_code=%s (%s)", doc, serie, res.ValidationCode, res.RawMessage)
	}
	if !ran {
		t.Skip("set AT_LIVE_SERIES_FT and/or AT_LIVE_SERIES_GR to register (irreversible at AT test)")
	}
}

// A2 live: envioDocumentoTransporte on AT TEST. Uses series GR2026T01 (registered at AT as
// GR by TestLiveRegisterSeries, never issued locally) so it cannot collide with app GR numbers.
//
//	AT_LIVE_USER=… AT_LIVE_PASS=… AT_LIVE_GR_SEQ=1 \
//	  go test -count=1 -tags=at_series_live -run TestLiveTransportGR -v ./internal/fiscal/at
func TestLiveTransportGR(t *testing.T) {
	sc, user, pass := liveSeriesClient(t)
	seq := envOr("AT_LIVE_GR_SEQ", "")
	if seq == "" {
		t.Skip("set AT_LIVE_GR_SEQ (next unused number of GR2026T01 at AT; each run consumes one)")
	}
	c := &SOAPTransportClient{Env: EnvTest, Trace: sc.Trace}
	lisbon, _ := time.LoadLocation("Europe/Lisbon")
	start := time.Now().Add(15 * time.Minute).In(lisbon).Format("2006-01-02T15:04:05")
	doc := TransportDocument{
		Username: user, Password: pass,
		TaxRegistrationNumber: "519655290", CompanyName: "FAINA VAIDOSA LDA",
		CompanyAddress: TransportAddress{Detail: "Rua Teste 1", City: "Lisboa", PostalCode: "1000-001"},
		DocumentNumber: "GR GR2026T01/" + seq, ATCUD: "AAJFJ48HR6-" + seq, // GR2026T01 validation code at AT
		MovementType: "GR", MovementStartTime: start,
		CustomerTaxID: "999999990", CustomerName: "Consumidor final teste",
		CustomerAddress: TransportAddress{Detail: "Rua Destino 2", City: "Porto", PostalCode: "4000-123"},
		ShipFrom:        TransportAddress{Detail: "Rua Teste 1", City: "Lisboa", PostalCode: "1000-001"},
		ShipTo:          TransportAddress{Detail: "Rua Destino 2", City: "Porto", PostalCode: "4000-123"},
		Lines:           []TransportLine{{Description: "Artigo de teste", Quantity: "1", UnitOfMeasure: "UN", UnitPrice: "10.00"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := c.RegisterDocument(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s → ATDocCodeID=%s (%s)", doc.DocumentNumber, res.ATDocCodeID, res.RawMessage)
}
