package store_test

import (
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestCreditRemainingForInvoice(t *testing.T) {
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
	ftID := seedDemoWithNC(t, db, sig)

	rem, err := db.CreditRemainingForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if rem.CreditedGrossTotal != "0.00" {
		t.Fatalf("credited %s", rem.CreditedGrossTotal)
	}
	if rem.RemainingGrossTotal != "12.50" {
		t.Fatalf("remaining total %s", rem.RemainingGrossTotal)
	}
	if len(rem.Lines) != 1 || rem.Lines[0].RemainingLineGross != "12.50" {
		t.Fatalf("lines %+v", rem.Lines)
	}
}
