package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

type captureTransport struct {
	docs   []at.TransportDocument
	before func() // runs inside RegisterDocument (simulate concurrent state change)
}

func (c *captureTransport) RegisterDocument(ctx context.Context, doc at.TransportDocument) (*at.TransportResult, error) {
	c.docs = append(c.docs, doc)
	if c.before != nil {
		c.before()
	}
	return &at.TransportResult{ATDocCodeID: "ATDOC-" + doc.DocumentNumber}, nil
}

// transportFixture: store 519655290 (address Rua 1, 1000-001 Lisboa), GR series GR2026T02, AT creds.
func transportFixture(t *testing.T) (*service.FiscalService, *store.DB, *captureTransport) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	sig, err := signer.LoadPEMFile(keyPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "519655290", LegalName: "FAINA VAIDOSA LDA",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "GR", "GR2026T02", "AAJFJB8H8B", 2026); err != nil {
		t.Fatal(err)
	}
	ct := &captureTransport{}
	svc := service.New(db, sig, nil, filepath.Join(dir, "secure"), "store-demo-001")
	svc.SetTransportClient(ct)
	if err := svc.UpsertATCredentials("store-demo-001", "519655290/1", "pw"); err != nil {
		t.Fatal(err)
	}
	return svc, db, ct
}

func grRequest(id string) domain.DeliveryNoteRequest {
	return domain.DeliveryNoteRequest{
		StoreID: "store-demo-001", RequestID: id, OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		MovementStartAt:     time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		RecipientIsVATPayer: true,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: id, ScopeType: "manual", ScopeID: id,
			FiscalPurpose: "delivery_note",
			Customer: domain.CustomerInput{TaxID: "509442013", CompanyName: "Cliente GR",
				AddressDetail: "Rua B 2", City: "Porto", PostalCode: "4000-123", Country: "PT"},
			Lines: []domain.SaleLine{{ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "2",
				UnitPriceGross: "12.30", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN"}},
		},
	}
}

// AT receives the real number the GR keeps, issuer + lines + creds + structured points
// (omitted points default to store / customer address); a retry never re-sends.
func TestIssueDeliveryNote_RealNumberToAT_NoDoubleSend(t *testing.T) {
	svc, db, ct := transportFixture(t)
	req := grRequest("gr-live-1")
	res, err := svc.IssueDeliveryNote(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct.docs) != 1 {
		t.Fatalf("AT calls %d", len(ct.docs))
	}
	d := ct.docs[0]
	if d.DocumentNumber != res.InvoiceNo || d.DocumentNumber != "GR GR2026T02/1" || d.ATCUD != "AAJFJB8H8B-1" {
		t.Fatalf("AT number %q / ATCUD %q vs issued %q", d.DocumentNumber, d.ATCUD, res.InvoiceNo)
	}
	if d.TaxRegistrationNumber != "519655290" || d.CompanyName != "FAINA VAIDOSA LDA" || d.Username != "519655290/1" || d.Password != "pw" {
		t.Fatalf("issuer/creds: %+v", d)
	}
	if d.ShipFrom != (at.TransportAddress{Detail: "Rua 1", City: "Lisboa", PostalCode: "1000-001"}) ||
		d.ShipTo != (at.TransportAddress{Detail: "Rua B 2", City: "Porto", PostalCode: "4000-123"}) {
		t.Fatalf("default points: from %+v to %+v", d.ShipFrom, d.ShipTo)
	}
	if len(d.Lines) != 1 || d.Lines[0].UnitPrice != "10.00" || d.Lines[0].Quantity != "2" {
		t.Fatalf("lines: %+v", d.Lines)
	}
	if len(d.MovementStartTime) != 19 || strings.ContainsAny(d.MovementStartTime[10:], "Z+") {
		t.Fatalf("start time not Lisbon wall clock: %q", d.MovementStartTime)
	}
	extra, err := db.LoadDeliveryExtra(res.DocumentID)
	if err != nil || extra.ShipFrom.String() != "Rua 1, 1000-001 Lisboa" {
		t.Fatalf("stored load point: %+v %v", extra, err)
	}
	again, err := svc.IssueDeliveryNote(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct.docs) != 1 || !again.IdempotentHit || again.InvoiceNo != res.InvoiceNo {
		t.Fatalf("retry must not re-send: calls=%d hit=%v", len(ct.docs), again.IdempotentHit)
	}
}

// Incomplete PT addresses are refused before AT in every env (mock included).
func TestIssueDeliveryNote_IncompleteAddressNeverReachesAT(t *testing.T) {
	svc, _, ct := transportFixture(t)
	for i, mut := range []func(*domain.DeliveryNoteRequest){
		func(r *domain.DeliveryNoteRequest) {
			r.ShipTo = domain.Address{Detail: "R. Almada Negreiros 456", City: "Lisboa"}
		},
		func(r *domain.DeliveryNoteRequest) {
			r.ShipFrom = domain.Address{Detail: "Room 401", City: "Hangzhou", PostalCode: "311258"}
		},
		func(r *domain.DeliveryNoteRequest) { r.Snapshot.Customer.PostalCode = "311258" },
	} {
		req := grRequest("gr-bad-" + string(rune('a'+i)))
		mut(&req)
		_, err := svc.IssueDeliveryNote(context.Background(), req)
		var ce *service.CodedError
		if !errors.As(err, &ce) || ce.Code != service.ErrCodeTransportAddressIncomplete {
			t.Fatalf("case %d: want %s, got %v", i, service.ErrCodeTransportAddressIncomplete, err)
		}
	}
	if len(ct.docs) != 0 {
		t.Fatalf("AT must not be called (%d calls)", len(ct.docs))
	}
}

// If the number moves after AT registered it, the GR is not saved under another number and
// the orphan AT registration is recorded for reconciliation.
func TestIssueDeliveryNote_SeqMovedAfterATIsAudited(t *testing.T) {
	svc, db, ct := transportFixture(t)
	ct.before = func() {
		if _, err := db.SQL.Exec(`UPDATE series SET last_number = last_number + 1 WHERE document_type = 'GR'`); err != nil {
			t.Fatal(err)
		}
	}
	_, err := svc.IssueDeliveryNote(context.Background(), grRequest("gr-moved-1"))
	if err == nil || !strings.Contains(err.Error(), "ATDOC-GR GR2026T02/1") {
		t.Fatalf("want commit failure naming the AT code, got %v", err)
	}
	var n int
	if err := db.SQL.QueryRow(`SELECT COUNT(1) FROM audit_log WHERE action = 'at_transport_unsaved' AND entity_id = 'GR GR2026T02/1'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows %d %v", n, err)
	}
}
