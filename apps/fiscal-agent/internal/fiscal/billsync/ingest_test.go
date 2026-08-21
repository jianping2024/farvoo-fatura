package billsync_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestIngestCloudJob_HappyAndIdempotent(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-1", SourceSystem: "farvoo", SourceSaleID: "sale-1",
		TableDisplayName: "018", ScopeType: "whole_table", GrossTotal: "47.10",
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "Prato", Qty: "1", UnitPriceGross: "10.00", LineGross: "10.00", VATRate: "23.00"},
			{ItemCode: "B", Name: "Cerveja", Qty: "1", UnitPriceGross: "2.25", LineGross: "2.25", VATRate: "13.00"},
		},
	})
	job := billsync.CloudJob{ID: "job-1", Status: "pending", Payload: payload}
	d1, err := billsync.IngestCloudJob(db, job)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := billsync.IngestCloudJob(db, job)
	if err != nil {
		t.Fatal(err)
	}
	if d1.ID != d2.ID {
		t.Fatalf("idempotent request_id should keep same draft")
	}
	list, _ := db.ListBillDrafts(10)
	if len(list) != 1 {
		t.Fatalf("drafts=%d", len(list))
	}
}

func TestIngest_RejectDecimalVAT(t *testing.T) {
	if err := billsync.ValidateVATPercent("0.23"); err == nil {
		t.Fatal("expected reject")
	}
	if err := billsync.ValidateVATPercent("23.00"); err != nil {
		t.Fatal(err)
	}
}

func TestDraftToSaleSnapshot_PercentToDecimal(t *testing.T) {
	snap := billsync.Snapshot{
		RequestID: "r", SourceSaleID: "sale-x", ScopeType: "whole_table", GrossTotal: "12.50",
		TableDisplayName: "A-01",
		Lines: []billsync.Line{
			{ItemCode: "P1", Name: "Prato", Qty: "1", UnitPriceGross: "12.50", LineGross: "12.50", VATRate: "23.00"},
		},
	}
	sale, err := billsync.DraftToSaleSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if sale.ScopeType != "whole_table" || sale.ScopeID != "sale-x" || sale.FiscalPurpose != "sale" {
		t.Fatalf("scope %+v", sale)
	}
	if sale.Lines[0].ProductCode != "P1" || sale.Lines[0].VATRate != "0.23" {
		t.Fatalf("line %+v", sale.Lines[0])
	}
	if sale.Customer.TaxID != "999999990" || sale.Payments[0].Method != "CASH" {
		t.Fatalf("defaults customer=%+v pay=%+v", sale.Customer, sale.Payments)
	}
}

func TestDraftToSaleSnapshot_RejectSplit(t *testing.T) {
	_, err := billsync.DraftToSaleSnapshot(billsync.Snapshot{
		SourceSaleID: "s", ScopeType: "split",
		Splits: []billsync.SplitPart{{ScopeID: "p1", Lines: []billsync.Line{
			{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
		}}},
	})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeValidationFailed {
		t.Fatalf("got %v", err)
	}
}

func TestIngest_AlreadyInvoicedViaTaxDB(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r1", SourceSaleID: "s1", ScopeType: "whole_table", GrossTotal: "1.00",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"}},
	})
	draft, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j1", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.IssueFromBillDraft(context.Background(), draft.ID, "op-demo-cashier"); err != nil {
		t.Fatal(err)
	}
	list, _ := db.ListBillDrafts(10)
	if len(list) != 0 {
		t.Fatalf("expected hard-delete drafts, got %d", len(list))
	}
	payload2, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r2", SourceSaleID: "s1", ScopeType: "whole_table", GrossTotal: "1.00",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"}},
	})
	_, err = billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j2", Payload: payload2})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeAlreadyInvoiced {
		t.Fatalf("got %v", err)
	}
}

func TestIssueFromBillDraft_WholeTablePrintsAndDeletes(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-draft", SourceSaleID: "sale-draft", ScopeType: "whole_table",
		TableDisplayName: "A-01", GrossTotal: "12.50",
		Lines: []billsync.Line{
			{ItemCode: "P1", Name: "Prato", Qty: "1", UnitPriceGross: "12.50", LineGross: "12.50", VATRate: "23.00"},
		},
	})
	draft, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "job-d", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	// second sync → open + discarded
	payload2, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-draft-2", SourceSaleID: "sale-draft", ScopeType: "whole_table",
		TableDisplayName: "A-01", GrossTotal: "12.50",
		Lines: []billsync.Line{
			{ItemCode: "P1", Name: "Prato", Qty: "1", UnitPriceGross: "12.50", LineGross: "12.50", VATRate: "23.00"},
		},
	})
	draft2, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "job-d2", Payload: payload2})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := db.ListBillDrafts(10)
	if len(before) != 2 {
		t.Fatalf("want 2 drafts got %d", len(before))
	}

	res, err := svc.IssueFromBillDraft(context.Background(), draft2.ID, "op-demo-cashier")
	if err != nil {
		t.Fatal(err)
	}
	if res.InvoiceNo == "" || res.ATCUD == "" {
		t.Fatalf("result %+v", res)
	}
	_ = draft
	after, _ := db.ListBillDrafts(10)
	if len(after) != 0 {
		t.Fatalf("all drafts for sale must be hard-deleted, got %d", len(after))
	}

	sink := &worker.MemorySink{}
	w := &worker.Worker{DB: db, Sink: sink}
	ok, err := w.RunOnce(context.Background())
	if err != nil || !ok {
		t.Fatalf("print worker: ok=%v err=%v", ok, err)
	}
	if len(sink.LastBytes) == 0 {
		t.Fatal("no escpos bytes")
	}
	job, err := db.GetPrintJob(res.PrintJobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != "PRINTED" {
		t.Fatalf("job status %s", job.JobStatus)
	}
}

func TestIssueFromBillDraft_RejectNonOpen(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()
	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r1", SourceSaleID: "s-disc", ScopeType: "whole_table", GrossTotal: "1.00",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"}},
	})
	d1, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j1", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	payload2, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r2", SourceSaleID: "s-disc", ScopeType: "whole_table", GrossTotal: "1.00",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"}},
	})
	if _, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j2", Payload: payload2}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueFromBillDraft(context.Background(), d1.ID, "op-demo-cashier")
	if err == nil {
		t.Fatal("expected reject discarded draft")
	}
}

func seedFiscal(t *testing.T, dir string) (*store.DB, *service.FiscalService) {
	t.Helper()
	dbPath := filepath.Join(dir, "fiscal.db")
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
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
	return db, service.New(db, sig, nil, dir, "store-demo-001")
}
