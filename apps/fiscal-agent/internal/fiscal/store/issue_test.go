package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestIssueFTEndToEnd(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sig, err := signer.LoadPEMFile(keyPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := sig.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}

	req := store.IssueParams{
		StoreID: "store-demo-001", RequestID: "req-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		NowUTC:     time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-1", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "12.50"}},
		},
	}
	rec, err := db.IssueFT(context.Background(), sig, req)
	if err != nil {
		t.Fatal(err)
	}
	if rec.InvoiceNo != "FT FT2026DEMO01/1" {
		t.Fatalf("invoice_no %s", rec.InvoiceNo)
	}
	if rec.ATCUD != "CSDF7T5H-1" {
		t.Fatalf("atcud %s", rec.ATCUD)
	}
	if rec.Hash == "" || len(rec.Hash) < 100 {
		t.Fatalf("hash short: %s", rec.Hash)
	}

	// idempotent same request
	rec2, err := db.IssueFT(context.Background(), sig, req)
	if err != nil {
		t.Fatal(err)
	}
	if !rec2.IdempotentHit || rec2.DocumentID != rec.DocumentID {
		t.Fatalf("idempotency failed")
	}

	// second document advances series
	req.RequestID = "req-2"
	req.Snapshot.SourceSaleID = "sale-2"
	rec3, err := db.IssueFT(context.Background(), sig, req)
	if err != nil {
		t.Fatal(err)
	}
	if rec3.InvoiceNo != "FT FT2026DEMO01/2" {
		t.Fatalf("seq2 %s", rec3.InvoiceNo)
	}

	sink := &worker.MemorySink{}
	w := &worker.Worker{DB: db, Sink: sink}
	ok, err := w.RunOnce(context.Background())
	if err != nil || !ok {
		t.Fatalf("print worker: ok=%v err=%v", ok, err)
	}
	if len(sink.LastBytes) == 0 {
		t.Fatal("no escpos bytes")
	}
	job, err := db.GetPrintJob(rec.PrintJobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != "PRINTED" {
		t.Fatalf("job status %s", job.JobStatus)
	}
}
