package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestCreditOriginalForNC(t *testing.T) {
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

	nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-read-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Devolucao total", CreditFull: true,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	ref, err := db.CreditOriginalForNC(nc.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil {
		t.Fatal("expected ref")
	}
	if ref.OriginalInvoiceID != ftID || ref.OriginalInvoiceNo != "FT FT2026DEMO01/1" || ref.CreditReason != "Devolucao total" {
		t.Fatalf("ref %+v", ref)
	}

	if got, err := db.CreditOriginalForNC(ftID); err != nil || got != nil {
		t.Fatalf("FT should have no NC ref: got=%v err=%v", got, err)
	}
}
