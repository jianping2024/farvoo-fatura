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

func TestCreateReprintPrintJob(t *testing.T) {
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
		StoreID: "store-demo-001", RequestID: "req-reprint-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		NowUTC:     time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-r1", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	}
	rec, err := db.IssueFT(context.Background(), sig, req)
	if err != nil {
		t.Fatal(err)
	}
	hashBefore := rec.Hash

	mem := &worker.MemorySink{}
	w := &worker.Worker{DB: db, Sink: mem}
	ok, err := w.RunOnce(context.Background())
	if err != nil || !ok {
		t.Fatalf("print worker: ok=%v err=%v", ok, err)
	}

	reprint, err := db.CreateReprintPrintJob(rec.DocumentID, "op-demo-cashier", "")
	if err != nil {
		t.Fatal(err)
	}
	if reprint.PrintJobID == "" || reprint.PrintJobID == rec.PrintJobID {
		t.Fatalf("reprint job id %q orig %q", reprint.PrintJobID, rec.PrintJobID)
	}

	recAfter, err := db.GetIssueRecordByID(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if recAfter.Hash != hashBefore {
		t.Fatalf("invoice hash changed after reprint enqueue")
	}

	job, err := db.GetPrintJob(reprint.PrintJobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.PrintPurpose != string(domain.PrintReprint) {
		t.Fatalf("purpose %q", job.PrintPurpose)
	}

	ok, err = w.RunOnce(context.Background())
	if err != nil || !ok {
		t.Fatalf("reprint worker: ok=%v err=%v", ok, err)
	}
	recAfter, err = db.GetIssueRecordByID(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if recAfter.PrintStatus != domain.PrintReprinted {
		t.Fatalf("print_status %q", recAfter.PrintStatus)
	}
}
