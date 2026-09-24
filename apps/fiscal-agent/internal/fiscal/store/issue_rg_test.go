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

func seedFTOnAccount(t *testing.T, db *store.DB, sig *signer.PEMSigner, requestID, amount string) string {
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
	if err := db.UpsertActiveSeries("store-demo-001", "RG", "RG2026DEMO01", "RGVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: requestID, DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1", NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-" + requestID, ScopeType: "session", ScopeID: "s-" + requestID, FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: amount, VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "509442013", CompanyName: "Cliente Conta", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: domain.PaymentAccount, Amount: amount}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec.DocumentID
}

func TestIssueRGFullThenRejectOver(t *testing.T) {
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
	ftID := seedFTOnAccount(t, db, sig, "ft-rg-1", "100.00")

	rem, err := db.ReceiptRemainingForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if rem.RemainingReceivableTotal != "100.00" {
		t.Fatalf("remaining want 100.00 got %s (settled=%s received=%s)", rem.RemainingReceivableTotal, rem.SettledAtIssueTotal, rem.ReceivedGrossTotal)
	}

	rg, err := db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ReceiveFull: true, PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rg.DocumentType != domain.DocumentRG || rg.InvoiceNo != "RG RG2026DEMO01/1" {
		t.Fatalf("rg=%+v", rg)
	}

	rem2, err := db.ReceiptRemainingForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if rem2.RemainingReceivableTotal != "0.00" || rem2.ReceivedGrossTotal != "100.00" {
		t.Fatalf("after full: %+v", rem2)
	}

	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-over", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Amount: "1.00", PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC),
	})
	if err != store.ErrReceiptAmountExceeded {
		t.Fatalf("want ErrReceiptAmountExceeded got %v", err)
	}
}

func TestIssueRGPartialSplit(t *testing.T) {
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
	ftID := seedFTOnAccount(t, db, sig, "ft-rg-2", "100.00")

	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-a", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Amount: "40.00", PaymentMethod: domain.PaymentCard,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-b", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Amount: "60.00", PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	rem, err := db.ReceiptRemainingForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if rem.RemainingReceivableTotal != "0.00" || rem.ReceivedGrossTotal != "100.00" {
		t.Fatalf("%+v", rem)
	}
}

func TestIssueRGRejectsFSAndCashSettledFT(t *testing.T) {
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
	if err := db.UpsertActiveSeries("store-demo-001", "FS", "FS2026DEMO01", "FSVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "RG", "RG2026DEMO01", "RGVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	fs, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "fs-1", DocType: domain.DocumentFS,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1", NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-fs", ScopeType: "session", ScopeID: "s-fs", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: domain.PaymentCash, Amount: "12.50"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-fs", OriginalInvoiceID: fs.DocumentID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ReceiveFull: true, PaymentMethod: domain.PaymentCash, NowUTC: now.Add(time.Hour),
	})
	if err != store.ErrReceiptNotAllowed {
		t.Fatalf("FS want ErrReceiptNotAllowed got %v", err)
	}

	ftCash, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-cash", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1", NowUTC: now.Add(2 * time.Hour),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-ftc", ScopeType: "session", ScopeID: "s-ftc", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: domain.PaymentCash, Amount: "12.50"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-cash-ft", OriginalInvoiceID: ftCash.DocumentID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ReceiveFull: true, PaymentMethod: domain.PaymentCash, NowUTC: now.Add(3 * time.Hour),
	})
	if err != store.ErrReceiptAmountExceeded {
		t.Fatalf("cash-settled FT want amount exceeded got %v", err)
	}
}

func TestIssueRGAfterPartialNC(t *testing.T) {
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
	ftID := seedFTOnAccount(t, db, sig, "ft-rg-nc", "100.00")
	_, err = db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-partial", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Reason: "Desconto", CreditFull: false,
		Lines: []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "30.00"}},
		NowUTC: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	rem, err := db.ReceiptRemainingForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if rem.RemainingReceivableTotal != "70.00" {
		t.Fatalf("want 70.00 got %+v", rem)
	}
	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-71", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Amount: "71.00", PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != store.ErrReceiptAmountExceeded {
		t.Fatalf("want exceeded got %v", err)
	}
	_, err = db.IssueRG(context.Background(), sig, store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-70", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Amount: "70.00", PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIssueRGIdempotent(t *testing.T) {
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
	ftID := seedFTOnAccount(t, db, sig, "ft-rg-idem", "50.00")
	p := store.IssueRGParams{
		StoreID: "store-demo-001", RequestID: "rg-idem", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ReceiveFull: true, PaymentMethod: domain.PaymentCash,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	}
	a, err := db.IssueRG(context.Background(), sig, p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.IssueRG(context.Background(), sig, p)
	if err != nil {
		t.Fatal(err)
	}
	if a.DocumentID != b.DocumentID || !b.IdempotentHit {
		t.Fatalf("idempotent a=%+v b=%+v", a, b)
	}
}
