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

func TestTerminalActivateDeleteAndStations(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-term-2"
	if err := db.UpsertTaxpayer(TaxpayerInput{
		StoreID: storeID, LegalName: "T", TaxRegistrationNumber: "456",
		Country: "PT", Timezone: "Europe/Lisbon", SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}
	id, err := db.RegisterLocalFiscalTerminal(storeID, "Counter")
	if err != nil {
		t.Fatal(err)
	}
	station := "11111111-1111-1111-1111-111111111111"
	if err := db.SetFiscalTerminalDefaultStation(storeID, id, station); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetFiscalTerminalByID(storeID, id)
	if err != nil || row.DefaultStationID != station {
		t.Fatalf("terminal station: %+v err=%v", row, err)
	}
	if err := db.RevokeFiscalTerminal(storeID, id); err != nil {
		t.Fatal(err)
	}
	if err := db.ActivateFiscalTerminal(storeID, id); err != nil {
		t.Fatal(err)
	}
	active, err := db.CountActiveFiscalTerminals(storeID)
	if err != nil || active != 1 {
		t.Fatalf("reactivated count=%d err=%v", active, err)
	}
	if err := db.RevokeFiscalTerminal(storeID, id); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteInactiveFiscalTerminal(storeID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetFiscalTerminalByID(storeID, id); err != ErrNotFound {
		t.Fatalf("deleted terminal should be gone: %v", err)
	}
	if err := db.DeleteInactiveFiscalTerminal(storeID, id); err != ErrNotFound {
		t.Fatalf("double delete want ErrNotFound got %v", err)
	}
	if err := db.ActivateFiscalTerminal(storeID, id); err != ErrNotFound {
		t.Fatalf("activate missing want ErrNotFound got %v", err)
	}

	localStation := "22222222-2222-2222-2222-222222222222"
	if err := db.SetLocalDefaultStation(storeID, localStation); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetLocalDefaultStation(storeID)
	if err != nil || got != localStation {
		t.Fatalf("local station=%q err=%v", got, err)
	}
	if err := db.SetLocalDefaultStation(storeID, ""); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetLocalDefaultStation(storeID)
	if err != nil || got != "" {
		t.Fatalf("cleared local station=%q err=%v", got, err)
	}
}

func TestSetFiscalTerminalLabel(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-term-label"
	if err := db.UpsertTaxpayer(TaxpayerInput{
		StoreID: storeID, LegalName: "T", TaxRegistrationNumber: "789",
		Country: "PT", Timezone: "Europe/Lisbon", SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}
	id, err := db.RegisterLocalFiscalTerminal(storeID, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetFiscalTerminalLabel(storeID, id, "  "); err == nil {
		t.Fatal("empty label must fail")
	}
	if err := db.SetFiscalTerminalLabel(storeID, id, " 吧台 "); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetFiscalTerminalByID(storeID, id)
	if err != nil || row.Label != "吧台" {
		t.Fatalf("label=%q err=%v", row.Label, err)
	}
	if err := db.SetFiscalTerminalLabel(storeID, "missing", "x"); err != ErrNotFound {
		t.Fatalf("missing want ErrNotFound got %v", err)
	}
}
