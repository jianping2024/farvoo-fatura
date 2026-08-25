package store

import (
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureConsumidorFinal(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpsertLocalFiscalProduct_andRemoteGuard(t *testing.T) {
	db := openTestDB(t)
	row, err := db.UpsertLocalFiscalProduct(LocalProductInput{
		ProductCode: "LOCAL-1", DisplayName: "Tea", SaftName: "Cha", UnitPriceGross: "4.50", VATRate: "13.00",
	})
	if err != nil || row.Source != "LOCAL" {
		t.Fatalf("local upsert: %v %+v", err, row)
	}
	_, _, err = db.UpsertFiscalProductByCode(ProductUpsertInput{
		ProductCode: "LOCAL-1", DisplayName: "X", SaftName: "X", UnitPriceGross: "1.00", VATRate: "23.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetFiscalProductByCode("LOCAL-1")
	if got.DisplayName != "Tea" {
		t.Fatalf("REMOTE must not overwrite LOCAL: %+v", got)
	}
	_, _, err = db.UpsertFiscalProductByCode(ProductUpsertInput{
		ProductCode: "R-1", DisplayName: "B", SaftName: "B", UnitPriceGross: "2.00", VATRate: "23.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	remote, _ := db.GetFiscalProductByCode("R-1")
	if remote.Source != "REMOTE_SYNC" || remote.UnitPriceGross != "2.00" {
		t.Fatalf("remote sync: %+v", remote)
	}
	_, err = db.UpsertLocalFiscalProduct(LocalProductInput{
		ProductCode: "R-1", DisplayName: "C", SaftName: "C", UnitPriceGross: "3.00", VATRate: "23.00",
	})
	if err == nil {
		t.Fatal("LOCAL must not take REMOTE code")
	}
}

func TestUpsertLocalFiscalProduct_NormalizesVATPercent(t *testing.T) {
	db := openTestDB(t)
	row, err := db.UpsertLocalFiscalProduct(LocalProductInput{
		ProductCode: "VAT-23", DisplayName: "Beer", SaftName: "Cerveja", UnitPriceGross: "3.00", VATRate: "23",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.VATRate != "23.00" {
		t.Fatalf("want vat 23.00, got %q", row.VATRate)
	}
}

func TestUpsertLocalCustomer_andIssueCustomerID(t *testing.T) {
	db := openTestDB(t)
	cust, err := db.UpsertLocalCustomer(LocalCustomerInput{
		CustomerTaxID: "123456789", CompanyName: "Cliente Demo",
	})
	if err != nil || cust.ID == "" {
		t.Fatalf("customer: %v %+v", err, cust)
	}
	tx, err := db.SQL.Begin()
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.ensureCustomerIDTx(tx, domain.CustomerInput{
		TaxID: "123456789", CompanyName: "Cliente Demo", Country: "PT",
		AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001",
	})
	if err != nil || id != cust.ID {
		t.Fatalf("ensure existing: %v %q want %q", err, id, cust.ID)
	}
	id2, err := db.ensureCustomerIDTx(tx, domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"})
	if err != nil || id2 != consumidorFinalID {
		t.Fatalf("consumidor: %v %q", err, id2)
	}
	_ = tx.Rollback()
}
