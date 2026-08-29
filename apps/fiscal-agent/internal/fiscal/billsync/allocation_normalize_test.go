package billsync_test

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
)

func TestNormalizeAllocationCoalescesDuplicateLineKeys(t *testing.T) {
	alloc := billsync.Allocation{
		People: []billsync.AllocPerson{{
			ScopeID: "11111111-1111-1111-1111-111111111111",
			Name:    "Jack",
			Shares: []billsync.AllocShare{
				{LineKey: "L1", Qty: billsync.Rational{Num: 1, Den: 2}},
				{LineKey: "L2", Qty: billsync.Rational{Num: 1, Den: 1}},
				{LineKey: "L1", Qty: billsync.Rational{Num: 1, Den: 2}},
			},
		}},
	}
	billsync.NormalizeAllocation(&alloc)
	if len(alloc.People[0].Shares) != 2 {
		t.Fatalf("want 2 shares after coalesce, got %+v", alloc.People[0].Shares)
	}
	if alloc.People[0].Shares[0].LineKey != "L1" || alloc.People[0].Shares[0].Qty.Num != 1 || alloc.People[0].Shares[0].Qty.Den != 1 {
		t.Fatalf("L1 should become 1/1, got %+v", alloc.People[0].Shares[0])
	}
	if alloc.People[0].Shares[1].LineKey != "L2" {
		t.Fatalf("order must keep first-seen keys, got %+v", alloc.People[0].Shares)
	}
}

func TestParseAllocationJSONCoalesces(t *testing.T) {
	raw := `{"people":[{"scope_id":"11111111-1111-1111-1111-111111111111","name":"Tom","shares":[
		{"line_key":"A","qty":{"num":1,"den":2}},
		{"line_key":"A","qty":{"num":1,"den":2}}
	]}]}`
	alloc, err := billsync.ParseAllocationJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(alloc.People) != 1 || len(alloc.People[0].Shares) != 1 {
		t.Fatalf("parse must coalesce, got %+v", alloc)
	}
	q := billsync.NormalizeRational(alloc.People[0].Shares[0].Qty)
	if q.Num != 1 || q.Den != 1 {
		t.Fatalf("want 1/1, got %+v", q)
	}
}

func TestDraftPersonFromAllocationCoalescesBeforeIssue(t *testing.T) {
	snap := billsync.Snapshot{
		SourceSystem: "farvoo",
		SourceSaleID: "sale-norm",
		ScopeType:    "whole_table",
		SourceLines: []billsync.Line{{
			LineKey: "L1", ItemCode: "TEA", Name: "Tea", Qty: "1",
			UnitPriceGross: "4.00", LineGross: "4.00", VATRate: "13.00",
		}},
	}
	alloc := billsync.Allocation{
		People: []billsync.AllocPerson{{
			ScopeID: "11111111-1111-1111-1111-111111111111",
			Name:    "Ana",
			Shares: []billsync.AllocShare{
				{LineKey: "L1", Qty: billsync.Rational{Num: 1, Den: 2}},
				{LineKey: "L1", Qty: billsync.Rational{Num: 1, Den: 2}},
			},
		}},
	}
	sale, err := billsync.DraftPersonFromAllocation(snap, alloc, "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if len(sale.Lines) != 1 {
		t.Fatalf("issue must emit one coalesced line, got %+v", sale.Lines)
	}
}
