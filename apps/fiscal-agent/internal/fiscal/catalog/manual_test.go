package catalog

import (
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestBuildManualSaleSnapshot_fromLocalProduct(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	_, err = db.UpsertLocalFiscalProduct(store.LocalProductInput{
		ProductCode: "LOCAL-TEA", DisplayName: "Tea", SaftName: "Cha", UnitPriceGross: "4.50", VATRate: "13.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := BuildManualSaleSnapshot(db, ManualIssueInput{
		RequestID: "req-manual-1",
		Lines:     []ManualLineInput{{ProductCode: "LOCAL-TEA", Quantity: "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.SourceSystem != "manual" || len(snap.Lines) != 1 {
		t.Fatalf("%+v", snap)
	}
	if snap.Lines[0].VATRate != "0.13" {
		t.Fatalf("vat decimal: %q", snap.Lines[0].VATRate)
	}
	if snap.Payments[0].Amount != "9.00" {
		t.Fatalf("gross: %q", snap.Payments[0].Amount)
	}
}

func TestBuildManualSaleSnapshot_tempLine(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	snap, err := BuildManualSaleSnapshot(db, ManualIssueInput{
		RequestID: "req-temp-1",
		Lines: []ManualLineInput{{
			DisplayName: "Temp", SaftName: "Temp PT", UnitPriceGross: "1.00", VATRatePercent: "23.00", Quantity: "1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Lines[0].ProductCode != "TEMP-1" {
		t.Fatalf("%+v", snap.Lines[0])
	}
}
