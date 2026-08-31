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

func TestCorrectiveOriginalForDocument_NC(t *testing.T) {
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

	ref, err := db.CorrectiveOriginalForDocument(nc.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil {
		t.Fatal("expected ref")
	}
	if ref.OriginalInvoiceID != ftID || ref.OriginalInvoiceNo != "FT FT2026DEMO01/1" || ref.CreditReason != "Devolucao total" {
		t.Fatalf("ref %+v", ref)
	}

	if got, err := db.CorrectiveOriginalForDocument(ftID); err != nil || got != nil {
		t.Fatalf("FT should have no corrective ref: got=%v err=%v", got, err)
	}
}

func TestCorrectiveOriginalForDocument_ND(t *testing.T) {
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
	seedDemoM6(t, db, sig)
	ftID := issueDemoFT(t, db, sig, domain.DocumentFT, "ft-nd-read")

	nd, err := db.IssueND(context.Background(), sig, store.IssueNDParams{
		StoreID: "store-demo-001", RequestID: "nd-read-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Ajuste parcial", DebitFull: false,
		Lines:  []store.CreditLineInput{{OriginalLineNumber: 1, LineGross: "2.00"}},
		NowUTC: time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	ref, err := db.CorrectiveOriginalForDocument(nd.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if ref == nil {
		t.Fatal("expected ND ref")
	}
	if ref.OriginalInvoiceID != ftID || ref.CreditReason != "Ajuste parcial" {
		t.Fatalf("ref %+v", ref)
	}
	if ref.OriginalInvoiceNo == "" {
		t.Fatal("expected original invoice no")
	}

	drem, err := db.DebitLinesForInvoice(ftID)
	if err != nil {
		t.Fatal(err)
	}
	if drem.DebitedGrossTotal != "2.00" {
		t.Fatalf("debited=%s", drem.DebitedGrossTotal)
	}
	if len(drem.Lines) < 1 {
		t.Fatal("expected debit lines")
	}
}
