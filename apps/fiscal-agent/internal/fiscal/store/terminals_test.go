package store

import (
	"path/filepath"
	"testing"
)

func TestLocalTerminalPairAndMax(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-term-1"
	if err := db.UpsertTaxpayer(TaxpayerInput{
		StoreID: storeID, LegalName: "T", TaxRegistrationNumber: "123",
		Country: "PT", Timezone: "Europe/Lisbon", SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMaxFiscalTerminals(storeID, 2); err != nil {
		t.Fatal(err)
	}
	code, _, err := db.CreateTerminalPairCode(storeID, "op-1", "Bar")
	if err != nil {
		t.Fatal(err)
	}
	id, lab, err := db.RedeemTerminalPairCode(storeID, code, "")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" || lab != "Bar" {
		t.Fatalf("id=%q lab=%q", id, lab)
	}
	if _, _, err := db.RedeemTerminalPairCode(storeID, code, ""); err == nil {
		t.Fatal("reuse must fail")
	}
	n, err := db.CountActiveFiscalTerminals(storeID)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if err := db.TouchFiscalTerminal(storeID, id, "192.168.1.9"); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetFiscalTerminalByID(storeID, id)
	if err != nil || row.LastSeenIP != "192.168.1.9" {
		t.Fatalf("touch ip: %+v err=%v", row, err)
	}
	if err := db.RevokeFiscalTerminal(storeID, id); err != nil {
		t.Fatal(err)
	}
	if err := db.TouchFiscalTerminal(storeID, id, "1.1.1.1"); err != ErrNotFound {
		t.Fatalf("revoked touch want ErrNotFound got %v", err)
	}
}
