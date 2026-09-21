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

func TestInvoiceRevenueSummaryNetCashBuckets(t *testing.T) {
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
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
	term := domain.LoopbackFiscalTerminalID
	cashFT, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "rev-cash", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", FiscalTerminalID: term, FiscalTerminalLabel: "本机",
		NowUTC: now,
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "s-cash", ScopeType: "session", ScopeID: "a", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "A", SaftName: "A", Quantity: "1",
				UnitPriceGross: "100.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "CF", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "100.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "rev-card", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", FiscalTerminalID: term, FiscalTerminalLabel: "本机",
		NowUTC: now.Add(time.Minute),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "s-card", ScopeType: "session", ScopeID: "b", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "A", SaftName: "A", Quantity: "1",
				UnitPriceGross: "50.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "CF", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CARD", Amount: "50.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "rev-nc", OriginalInvoiceID: cashFT.DocumentID,
		OperatorID: "op-demo-cashier", FiscalTerminalID: term, FiscalTerminalLabel: "本机",
		Reason: "partial", CreditFull: false,
		Lines:  []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "20.00"}},
		NowUTC: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	sum, err := db.InvoiceRevenueSummary(store.InvoiceRevenueSummaryQuery{
		StoreID: "store-demo-001", From: "2026-08-20", To: "2026-08-20",
	})
	if err != nil {
		t.Fatal(err)
	}
	// cash 100-20=80, noncash 50, net 130; count 3
	if sum.CashGrossSum != "80.00" || sum.NonCashGrossSum != "50.00" || sum.GrossNetSum != "130.00" {
		t.Fatalf("sums cash=%s non=%s net=%s", sum.CashGrossSum, sum.NonCashGrossSum, sum.GrossNetSum)
	}
	if sum.InvoiceCount != 3 {
		t.Fatalf("count %d", sum.InvoiceCount)
	}

	// Document-type filter must not exist on summary — issuing another day must not change.
	sumTerm, err := db.InvoiceRevenueSummary(store.InvoiceRevenueSummaryQuery{
		StoreID: "store-demo-001", From: "2026-08-20", To: "2026-08-20",
		FiscalTerminalID: term,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sumTerm.GrossNetSum != "130.00" {
		t.Fatalf("terminal filter net %s", sumTerm.GrossNetSum)
	}
}
