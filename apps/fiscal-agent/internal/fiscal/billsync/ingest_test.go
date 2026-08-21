package billsync_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/store"
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

func TestIngest_AlreadyInvoiced(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r1", SourceSaleID: "s1", ScopeType: "whole_table",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"}},
	})
	if _, err := billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j1", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkBillDraftInvoiced("s1"); err != nil {
		t.Fatal(err)
	}
	payload2, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r2", SourceSaleID: "s1", ScopeType: "whole_table",
		Lines: []billsync.Line{{ItemCode: "A", Name: "X", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"}},
	})
	_, err = billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j2", Payload: payload2})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeAlreadyInvoiced {
		t.Fatalf("got %v", err)
	}
}

func TestIngest_ItemCodeConflict(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	payload, _ := json.Marshal(billsync.Snapshot{
		RequestID: "r1", SourceSaleID: "s1", ScopeType: "whole_table",
		Lines: []billsync.Line{
			{ItemCode: "A", Name: "One", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"},
			{ItemCode: "A", Name: "Two", UnitPriceGross: "1.00", LineGross: "1.00", VATRate: "23.00", Qty: "1"},
		},
	})
	_, err = billsync.IngestCloudJob(db, billsync.CloudJob{ID: "j1", Payload: payload})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeItemCodeConflict {
		t.Fatalf("got %v", err)
	}
}
