package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestIssueFTEnqueuesOutboxOnce(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pem := filepath.Join("..", "testdata", "dev_signing_key.pem")
	sig, err := signer.LoadPEMFile(pem, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := sig.PublicKeyPEM()
	year := 2026
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-outbox", TaxpayerNIF: "517535009", LegalName: "Demo",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01",
		ValidationCode: "CSDF7T5H", FiscalYear: year,
		OperatorID: "op-demo-cashier", OperatorName: "Demo", SigningKeyVersion: 1,
		InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "pub",
	}, pem, pub); err != nil {
		t.Fatal(err)
	}
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-outbox", RequestID: "req-ob-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", StationID: "st-1", NowUTC: time.Now().UTC(),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-1", ScopeType: "whole_table",
			ScopeID: "sale-1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.CountOutboxByStatus("store-outbox", store.OutboxPending)
	if err != nil || n != 1 {
		t.Fatalf("pending outbox want 1 got %d err=%v", n, err)
	}
	row, err := db.ClaimNextOutbox(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkOutboxSent(row.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	n, _ = db.CountOutboxByStatus("store-outbox", store.OutboxSent)
	if n != 1 {
		t.Fatalf("sent want 1 got %d doc=%s", n, rec.DocumentID)
	}
	// idempotent hit must NOT enqueue a second outbox row
	_, err = db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-outbox", RequestID: "req-ob-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", StationID: "st-1", NowUTC: time.Now().UTC(),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-1", ScopeType: "whole_table",
			ScopeID: "sale-1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := db.CountOutboxByStatus("store-outbox", store.OutboxPending)
	sent, _ := db.CountOutboxByStatus("store-outbox", store.OutboxSent)
	if pending != 0 || sent != 1 {
		t.Fatalf("after idempotent hit pending=%d sent=%d", pending, sent)
	}
}
