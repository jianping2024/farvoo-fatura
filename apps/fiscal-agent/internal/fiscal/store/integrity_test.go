package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestVerifySeriesIntegrity_BlockAndHeal(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, filepath.Join("..", "testdata", "dev_signing_key.pem"), pub); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	_, err = db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "s1", ScopeType: "session", ScopeID: "a", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "X", SaftName: "X", Quantity: "1",
				UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "CF", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	okRep, err := db.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{BlockOnFail: true})
	if err != nil || !okRep.OK {
		t.Fatalf("want ok, got %+v err=%v", okRep, err)
	}

	if _, err := db.SQL.Exec(`UPDATE series SET last_hash='tampered' WHERE document_type='FT'`); err != nil {
		t.Fatal(err)
	}
	bad, err := db.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{BlockOnFail: true, OperatorID: "op"})
	if err != nil {
		t.Fatal(err)
	}
	if bad.OK || bad.Blocked < 1 {
		t.Fatalf("want blocked fail, got %+v", bad)
	}
	var st string
	_ = db.SQL.QueryRow(`SELECT status FROM series WHERE document_type='FT'`).Scan(&st)
	if st != "FAILED" {
		t.Fatalf("status=%s", st)
	}

	if _, err := db.SQL.Exec(`UPDATE series SET last_hash=(SELECT hash FROM invoices WHERE document_type='FT' LIMIT 1) WHERE document_type='FT'`); err != nil {
		t.Fatal(err)
	}
	healed, err := db.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{HealOnPass: true, OperatorID: "op"})
	if err != nil {
		t.Fatal(err)
	}
	if !healed.OK || healed.Healed < 1 {
		t.Fatalf("want heal, got %+v", healed)
	}
	_ = db.SQL.QueryRow(`SELECT status FROM series WHERE document_type='FT'`).Scan(&st)
	if st != "ACTIVE" {
		t.Fatalf("status after heal=%s", st)
	}
}

func TestBackupFiscalDB_VacuumInto(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	path, size, err := db.BackupFiscalDB("")
	if err != nil {
		t.Fatal(err)
	}
	if size <= 0 {
		t.Fatalf("size=%d", size)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	db2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = db2.Close()
}
