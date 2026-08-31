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

func seedDemoM6(t *testing.T, db *store.DB, sig *signer.PEMSigner) {
	t.Helper()
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		docType, code, val string
	}{
		{"FS", "FS2026DEMO01", "FSVAL1234"},
		{"FR", "FR2026DEMO01", "FRVAL1234"},
		{"ND", "ND2026DEMO01", "NDVAL1234"},
	} {
		if err := db.UpsertActiveSeries("store-demo-001", pair.docType, pair.code, pair.val, 2026); err != nil {
			t.Fatal(err)
		}
	}
}

func issueDemoFT(t *testing.T, db *store.DB, sig *signer.PEMSigner, docType domain.DocumentType, requestID string) string {
	t.Helper()
	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: requestID, DocType: docType,
		OperatorID: "op-demo-cashier", NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: requestID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "12.50"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec.DocumentID
}

func TestIssueFSAndFR(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)

	fsID := issueDemoFT(t, db, sig, domain.DocumentFS, "fs-req-1")
	frID := issueDemoFT(t, db, sig, domain.DocumentFR, "fr-req-1")

	var fsNo, frNo string
	if err := db.SQL.QueryRow(`SELECT invoice_no FROM invoices WHERE id=?`, fsID).Scan(&fsNo); err != nil || fsNo != "FS FS2026DEMO01/1" {
		t.Fatalf("fs invoice_no %s err=%v", fsNo, err)
	}
	if err := db.SQL.QueryRow(`SELECT invoice_no FROM invoices WHERE id=?`, frID).Scan(&frNo); err != nil || frNo != "FR FR2026DEMO01/1" {
		t.Fatalf("fr invoice_no %s err=%v", frNo, err)
	}
}

func TestIssueNDFullDebitOnFS(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)
	fsID := issueDemoFT(t, db, sig, domain.DocumentFS, "fs-nd-1")

	nd, err := db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-req-1", OriginalInvoiceID: fsID,
		OperatorID: "op-demo-cashier", Reason: "Ajuste positivo", DebitFull: true,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if nd.InvoiceNo != "ND ND2026DEMO01/1" {
		t.Fatalf("invoice_no %s", nd.InvoiceNo)
	}

	var fsStatus, debited string
	if err := db.SQL.QueryRow(`SELECT document_status, debited_gross_total FROM invoices WHERE id=?`, fsID).
		Scan(&fsStatus, &debited); err != nil {
		t.Fatal(err)
	}
	if fsStatus != string(domain.DocumentDebitedFull) || debited != "12.50" {
		t.Fatalf("original status=%s debited=%s", fsStatus, debited)
	}
}

func TestIssueNDFullDebitOnFT(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)
	ftID := issueDemoFT(t, db, sig, domain.DocumentFT, "ft-nd-1")

	nd, err := db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-ft-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Ajuste FT", DebitFull: true,
		NowUTC: time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if nd.DocumentType != domain.DocumentND {
		t.Fatalf("type %s", nd.DocumentType)
	}
	var status string
	if err := db.SQL.QueryRow(`SELECT document_status FROM invoices WHERE id=?`, ftID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(domain.DocumentDebitedFull) {
		t.Fatalf("status %s", status)
	}
}

func TestIssueNDPartialDebit(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)
	ftID := issueDemoFT(t, db, sig, domain.DocumentFT, "ft-nd-partial")

	nd, err := db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-partial-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Partial debit", DebitFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "5.00"}},
		NowUTC: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if nd.InvoiceNo != "ND ND2026DEMO01/1" {
		t.Fatalf("invoice_no %s", nd.InvoiceNo)
	}
	var status, debited string
	if err := db.SQL.QueryRow(`SELECT document_status, debited_gross_total FROM invoices WHERE id=?`, ftID).
		Scan(&status, &debited); err != nil {
		t.Fatal(err)
	}
	if status != string(domain.DocumentDebitedPartial) || debited != "5.00" {
		t.Fatalf("status=%s debited=%s", status, debited)
	}
}

func TestIssueNDAmountExceeded(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)
	ftID := issueDemoFT(t, db, sig, domain.DocumentFT, "ft-nd-over")
	_, err = db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-over-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Too much", DebitFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "20.00"}},
	})
	if err != store.ErrDebitAmountExceeded {
		t.Fatalf("want exceeded, got %v", err)
	}
}

func TestIssueNDDebitedFullRejected(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedDemoM6(t, db, sig)
	ftID := issueDemoFT(t, db, sig, domain.DocumentFT, "ft-nd-full")
	_, err = db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-full-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Full", DebitFull: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-full-2", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Again", DebitFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "1.00"}},
	})
	if err != store.ErrDebitNotAllowed {
		t.Fatalf("want debit_not_allowed, got %v", err)
	}
}
