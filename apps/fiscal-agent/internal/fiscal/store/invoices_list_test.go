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

	page, err := db.ListInvoices(store.InvoiceListQuery{StoreID: "store-demo-001", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("want 1 invoice, got total=%d items=%d", page.Total, len(page.Items))
	}
	it := page.Items[0]
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
	if it.PaymentMethod != "CASH" {
		t.Fatalf("payment_method: got %q want CASH", it.PaymentMethod)
	}
	if it.ATCUD == "" || it.InvoiceNo == "" {
		t.Fatalf("atcud/invoice_no empty: %+v", it)
	}
	if page.GrossTotalSum != "10.00" {
		t.Fatalf("gross_total_sum: got %q", page.GrossTotalSum)
	}

	detail, err := db.GetInvoiceDetail(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PaymentMethod != "CASH" {
		t.Fatalf("detail payment_method: got %q want CASH", detail.PaymentMethod)
	}
}

func TestListInvoicesFilterByInvoiceDateAndSearch(t *testing.T) {
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

	issue := func(reqID string, when time.Time, table string, taxID string) {
		t.Helper()
		_, err := db.IssueFT(context.Background(), sig, store.IssueParams{
			StoreID: "store-demo-001", RequestID: reqID, DocType: domain.DocumentFT,
			OperatorID: "op-demo-cashier", NowUTC: when,
			Snapshot: domain.SaleSnapshot{
				SourceSystem: "farvoo", SourceSaleID: "sale-" + reqID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
				DisplayMeta: map[string]string{"table_display_name": table},
				Lines: []domain.SaleLine{{
					ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
					UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
				}},
				Customer: domain.CustomerInput{TaxID: taxID, CompanyName: "Buyer " + taxID, Country: "PT"},
				Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	issue("req-a", time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC), "12", "502757191")
	issue("req-b", time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), "A-01", "999999990")

	only27, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, From: "2026-08-27", To: "2026-08-27",
	})
	if err != nil {
		t.Fatal(err)
	}
	if only27.Total != 1 || len(only27.Items) != 1 || only27.Items[0].CustomerTaxID != "502757191" {
		t.Fatalf("date filter: got %+v", only27)
	}

	byTable, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, Q: "A-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if byTable.Total != 1 || len(byTable.Items) != 1 || byTable.Items[0].OrderLabel != "桌 A-01" {
		t.Fatalf("search: got %+v", byTable)
	}

	empty, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, From: "2020-01-01", To: "2020-01-02",
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("expected empty range, got total=%d items=%d", empty.Total, len(empty.Items))
	}
}

func TestListInvoicesPagination(t *testing.T) {
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

	for i := 0; i < 12; i++ {
		reqID := "req-page-" + string(rune('a'+i))
		_, err := db.IssueFT(context.Background(), sig, store.IssueParams{
			StoreID: "store-demo-001", RequestID: reqID, DocType: domain.DocumentFT,
			OperatorID: "op-demo-cashier",
			NowUTC:     time.Date(2026, 8, 27, 10, i, 0, 0, time.UTC),
			Snapshot: domain.SaleSnapshot{
				SourceSystem: "farvoo", SourceSaleID: "sale-" + reqID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
				Lines: []domain.SaleLine{{
					ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
					UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
				}},
				Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Buyer", Country: "PT"},
				Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	page1, err := db.ListInvoices(store.InvoiceListQuery{StoreID: "store-demo-001", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 12 || len(page1.Items) != 10 || page1.Page != 1 {
		t.Fatalf("page1: total=%d items=%d page=%d", page1.Total, len(page1.Items), page1.Page)
	}
	if page1.GrossTotalSum != "120.00" {
		t.Fatalf("gross_total_sum: got %q", page1.GrossTotalSum)
	}

	page2, err := db.ListInvoices(store.InvoiceListQuery{StoreID: "store-demo-001", Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 12 || len(page2.Items) != 2 || page2.Page != 2 {
		t.Fatalf("page2: total=%d items=%d page=%d", page2.Total, len(page2.Items), page2.Page)
	}

	page20, err := db.ListInvoices(store.InvoiceListQuery{StoreID: "store-demo-001", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page20.Total != 12 || len(page20.Items) != 12 || page20.PageSize != 20 {
		t.Fatalf("page_size 20: total=%d items=%d pageSize=%d", page20.Total, len(page20.Items), page20.PageSize)
	}
}

func TestListInvoicesFilterByDocumentType(t *testing.T) {
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
	if err := db.UpsertActiveSeries("store-demo-001", "FS", "FS2026DEMO01", "FSVAL01", 2026); err != nil {
		t.Fatal(err)
	}

	issue := func(reqID string, docType domain.DocumentType) {
		t.Helper()
		_, err := db.IssueFT(context.Background(), sig, store.IssueParams{
			StoreID: "store-demo-001", RequestID: reqID, DocType: docType,
			OperatorID: "op-demo-cashier",
			NowUTC:     time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC),
			Snapshot: domain.SaleSnapshot{
				SourceSystem: "farvoo", SourceSaleID: "sale-" + reqID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
				Lines: []domain.SaleLine{{
					ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
					UnitPriceGross: "10.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
				}},
				Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Buyer", Country: "PT"},
				Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	issue("req-ft", domain.DocumentFT)
	issue("req-fs", domain.DocumentFS)

	all, err := db.ListInvoices(store.InvoiceListQuery{StoreID: "store-demo-001", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 {
		t.Fatalf("all types: total=%d", all.Total)
	}

	onlyFT, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, DocumentType: "FT",
	})
	if err != nil {
		t.Fatal(err)
	}
	if onlyFT.Total != 1 || len(onlyFT.Items) != 1 || onlyFT.Items[0].DocumentType != "FT" {
		t.Fatalf("FT filter: %+v", onlyFT)
	}

	onlyFS, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, DocumentType: "FS",
	})
	if err != nil {
		t.Fatal(err)
	}
	if onlyFS.Total != 1 || len(onlyFS.Items) != 1 || onlyFS.Items[0].DocumentType != "FS" {
		t.Fatalf("FS filter: %+v", onlyFS)
	}

	emptyNC, err := db.ListInvoices(store.InvoiceListQuery{
		StoreID: "store-demo-001", Page: 1, PageSize: 10, DocumentType: "NC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if emptyNC.Total != 0 || len(emptyNC.Items) != 0 {
		t.Fatalf("NC filter empty: total=%d items=%d", emptyNC.Total, len(emptyNC.Items))
	}
}
