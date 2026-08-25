package store

import (
	"path/filepath"
	"testing"
)

func TestBillDraftWritersFireUIHook(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var calls []struct {
		n    int
		hint string
		kind string
	}
	db.OnBillDraftsChanged = func(openCount int, tableHint, kind string) {
		calls = append(calls, struct {
			n    int
			hint string
			kind string
		}{openCount, tableHint, kind})
	}

	payload := map[string]any{
		"request_id": "req-ui-1", "source_sale_id": "sale-ui-1", "source_system": "farvoo",
		"scope_type": "whole_table", "table_display_name": "B-12",
		"lines": []map[string]string{{
			"item_code": "T", "name": "Tea", "qty": "1",
			"unit_price_gross": "1.00", "line_gross": "1.00", "vat_rate": "13.00",
		}},
	}
	if _, err := db.UpsertBillDraftOpen("req-ui-1", "sale-ui-1", "job-1", payload, "{}", 0); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].n != 1 || calls[0].hint != "B-12" || calls[0].kind != "upsert" {
		t.Fatalf("upsert hook %#v", calls)
	}

	// Idempotent same request_id: no second notify.
	if _, err := db.UpsertBillDraftOpen("req-ui-1", "sale-ui-1", "job-1", payload, "{}", 0); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("idempotent must not re-notify, got %d", len(calls))
	}

	if err := db.DeleteBillDraftsBySale("sale-ui-1"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1].n != 0 || calls[1].kind != "delete" {
		t.Fatalf("delete hook %#v", calls)
	}
}
