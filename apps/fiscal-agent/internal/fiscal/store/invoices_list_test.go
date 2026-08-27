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

func TestListInvoicesIncludesHashCustomerAndSource(t *testing.T) {
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
		StoreID: "store-demo-001", RequestID: "req-list-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		NowUTC:     time.Date(2026, 8, 27, 18, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-list-1", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			DisplayMeta: map[string]string{"table_display_name": "12", "split_name": "Ana"},
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "502757191", CompanyName: "Acme Lda", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	}
	rec, err := db.IssueFT(context.Background(), sig, req)
	if err != nil {
		t.Fatal(err)
	}

	list, err := db.ListInvoices("store-demo-001", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 invoice, got %d", len(list))
	}
	it := list[0]
	if it.DocumentID != rec.DocumentID {
		t.Fatalf("document_id: got %s want %s", it.DocumentID, rec.DocumentID)
	}
	if it.Hash == "" || it.SystemEntryDate == "" {
		t.Fatalf("hash/system_entry_date required: %+v", it)
	}
	if it.IssuedAt != it.SystemEntryDate {
		t.Fatalf("issued_at must mirror system_entry_date: %q vs %q", it.IssuedAt, it.SystemEntryDate)
	}
	if it.CustomerTaxID != "502757191" || it.CustomerName != "Acme Lda" {
		t.Fatalf("customer snapshot: tax=%q name=%q", it.CustomerTaxID, it.CustomerName)
	}
	if it.OrderLabel != "桌 12 · Ana" {
		t.Fatalf("order_label: got %q", it.OrderLabel)
	}
	if it.ATCUD == "" || it.InvoiceNo == "" {
		t.Fatalf("atcud/invoice_no empty: %+v", it)
	}
}
