package billsync_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestAutoIssue_WholeTableUsesProvidedDocumentType(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-ai-wt", SourceSaleID: "sale-ai-wt", ScopeType: "whole_table",
		TableDisplayName: "B-01", GrossTotal: "12.50",
		AutoIssue: true, DocumentType: "FT", PaymentMethod: "CARD",
		Lines: []billsync.Line{
			{ItemCode: "P1", Name: "Prato", Qty: "1", UnitPriceGross: "12.50", LineGross: "12.50", VATRate: "23.00"},
		},
	})
	out, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "job-ai-wt", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if out.Issue == nil || out.Issue.InvoiceNo == "" {
		t.Fatalf("want invoice_no, got %+v", out.Issue)
	}
	if out.Issue.DocumentType != domain.DocumentFT {
		t.Fatalf("want FT (Farvoo-provided), got %s", out.Issue.DocumentType)
	}
	list, _ := db.ListBillDrafts(10)
	if len(list) != 0 {
		t.Fatalf("whole_table auto_issue should delete draft, got %d", len(list))
	}
	tax, err := db.InvoiceCustomerTaxID(out.Issue.DocumentID)
	if err != nil || tax != "999999990" {
		t.Fatalf("empty nif → scatter, tax=%s err=%v", tax, err)
	}
}

func TestAutoIssue_RequiresDocumentType(t *testing.T) {
	dir := t.TempDir()
	_, svc := seedFiscal(t, dir)

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-ai-nodoc", SourceSaleID: "sale-ai-nodoc", ScopeType: "whole_table",
		GrossTotal: "1.00", AutoIssue: true,
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
		},
	})
	_, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "j", Payload: payload})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeValidationFailed || !strings.Contains(ie.Message, "document_type") {
		t.Fatalf("want document_type validation, got %v", err)
	}
}

func TestAutoIssue_SplitPersonThenSecondPerson(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()

	scopeA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	scopeB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	base := billsync.Snapshot{
		SourceSaleID: "sale-ai-split", ScopeType: "split",
		AutoIssue: true, DocumentType: "FS", PaymentMethod: "CASH",
		Splits: []billsync.SplitPart{
			{ScopeID: scopeA, Name: "A", GrossTotal: "1.00", Lines: []billsync.Line{
				{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
			}},
			{ScopeID: scopeB, Name: "B", GrossTotal: "2.00", Lines: []billsync.Line{
				{ItemCode: "B", Name: "Y", Qty: "1", UnitPriceGross: "2.00", LineGross: "2.00", VATRate: "23.00"},
			}},
		},
	}
	snapA := base
	snapA.RequestID = "req-ai-a"
	snapA.IssueScopeID = scopeA
	payloadA, _ := json.Marshal(snapA)
	outA, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "ja", Payload: payloadA})
	if err != nil {
		t.Fatal(err)
	}
	if outA.Issue == nil || outA.Issue.DocumentType != domain.DocumentFS {
		t.Fatalf("person A: %+v", outA.Issue)
	}
	list, _ := db.ListBillDrafts(10)
	if len(list) != 1 {
		t.Fatalf("draft should remain after first person, got %d", len(list))
	}

	snapB := base
	snapB.RequestID = "req-ai-b"
	snapB.IssueScopeID = scopeB
	payloadB, _ := json.Marshal(snapB)
	outB, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "jb", Payload: payloadB})
	if err != nil {
		t.Fatal(err)
	}
	if outB.Issue == nil || outB.Issue.InvoiceNo == "" {
		t.Fatalf("person B: %+v", outB.Issue)
	}
	list, _ = db.ListBillDrafts(10)
	if len(list) != 0 {
		t.Fatalf("all people done → draft deleted, got %d", len(list))
	}
}

func TestAutoIssue_DoesNotBreakManualIngest(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()

	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-manual", SourceSaleID: "sale-manual", ScopeType: "whole_table",
		GrossTotal: "1.00",
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
		},
	})
	out, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "jm", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if out.Issue != nil {
		t.Fatal("manual sync must not auto-issue")
	}
	list, _ := db.ListBillDrafts(10)
	if len(list) != 1 || list[0].Status != store.BillDraftOpen {
		t.Fatalf("%+v", list)
	}
	// Admin manual path still works
	res, err := svc.IssueFromBillDraft(context.Background(), service.IssueBillDraftInput{
		DraftID: out.Draft.ID, StationID: "st-uat", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1", Mode: "whole_table",
	})
	if err != nil || res.InvoiceNo == "" {
		t.Fatalf("manual issue: %v %+v", err, res)
	}
}

func TestProcessBillSyncJob_ReturnsInvoiceNo(t *testing.T) {
	dir := t.TempDir()
	_, svc := seedFiscal(t, dir)
	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-proc", SourceSaleID: "sale-proc", ScopeType: "whole_table",
		GrossTotal: "1.00", AutoIssue: true, DocumentType: "FS",
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
		},
	})
	no, err := svc.ProcessBillSyncJob(context.Background(), billsync.CloudJob{ID: "jp", Payload: payload})
	if err != nil || no == "" {
		t.Fatalf("invoice_no=%q err=%v", no, err)
	}
}

func TestAutoIssue_StationRequired(t *testing.T) {
	dir := t.TempDir()
	db, svc := seedFiscal(t, dir)
	defer db.Close()
	if err := db.SetLocalDefaultStation("store-demo-001", ""); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "req-nost", SourceSaleID: "sale-nost", ScopeType: "whole_table",
		GrossTotal: "1.00", AutoIssue: true, DocumentType: "FS",
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "X", Qty: "1", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00"},
		},
	})
	_, err := svc.IngestBillSyncJob(context.Background(), billsync.CloudJob{ID: "jn", Payload: payload})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != "station_required" {
		t.Fatalf("want station_required, got %v", err)
	}
}
