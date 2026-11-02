package service_test

import (
	"context"
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
	docs []at.TransportDocument
}

func (c *captureTransport) RegisterDocument(ctx context.Context, doc at.TransportDocument) (*at.TransportResult, error) {
	c.docs = append(c.docs, doc)
	return &at.TransportResult{ATDocCodeID: "ATDOC-" + doc.DocumentNumber}, nil
}

// AT must receive the real number the GR keeps ("GR SERIE/N"), issuer + lines + creds;
// a retry of the same request must not communicate again.
func TestIssueDeliveryNote_RealNumberToAT_NoDoubleSend(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
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
	moveAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	req := domain.DeliveryNoteRequest{
		StoreID: "store-demo-001", RequestID: "gr-live-1", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ShipFromAddress: "Rua A 1, 1000-001 Lisboa", ShipToAddress: "Rua B 2, 4000-123 Porto",
		MovementStartAt: moveAt, RecipientIsVATPayer: true,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gr-sale-1", ScopeType: "manual", ScopeID: "gr-sale-1",
			FiscalPurpose: "delivery_note",
			Customer: domain.CustomerInput{TaxID: "509442013", CompanyName: "Cliente GR",
				AddressDetail: "Rua B 2", City: "Porto", PostalCode: "4000-123", Country: "PT"},
			Lines: []domain.SaleLine{{ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "2",
				UnitPriceGross: "12.30", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN"}},
		},
	}
	res, err := svc.IssueDeliveryNote(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct.docs) != 1 {
		t.Fatalf("AT calls %d", len(ct.docs))
	}
	d := ct.docs[0]
	if d.DocumentNumber != res.InvoiceNo || !strings.HasPrefix(d.DocumentNumber, "GR GR2026T02/") {
		t.Fatalf("AT number %q vs issued %q", d.DocumentNumber, res.InvoiceNo)
	}
	if d.ATCUD != "AAJFJB8H8B-"+strings.TrimPrefix(d.DocumentNumber, "GR GR2026T02/") {
		t.Fatalf("ATCUD %q for %q", d.ATCUD, d.DocumentNumber)
	}
	if d.TaxRegistrationNumber != "519655290" || d.CompanyName != "FAINA VAIDOSA LDA" || d.Username != "519655290/1" || d.Password != "pw" {
		t.Fatalf("issuer/creds: %+v", d)
	}
	if len(d.Lines) != 1 || d.Lines[0].UnitPrice != "10.00" || d.Lines[0].Quantity != "2" {
		t.Fatalf("lines: %+v", d.Lines)
	}
	if len(d.MovementStartTime) != 19 || strings.ContainsAny(d.MovementStartTime[10:], "Z+") {
		t.Fatalf("start time not Lisbon wall clock: %q", d.MovementStartTime)
	}
	again, err := svc.IssueDeliveryNote(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(ct.docs) != 1 || !again.IdempotentHit || again.InvoiceNo != res.InvoiceNo {
		t.Fatalf("retry must not re-send: calls=%d hit=%v", len(ct.docs), again.IdempotentHit)
	}
}
