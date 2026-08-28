package api

import "testing"

func TestBuildPrinterStationList_labelsAndSort(t *testing.T) {
	mapped := map[string]string{
		"2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff": "tcp:172.20.10.3:9100",
		"7e8facc6-1111-2222-3333-444455556666": "tcp:172.20.10.3:9100",
		"orphan-uuid-8903ce39-0000-0000-0000":  "winspool:UK56009",
	}
	meta := []StationMeta{
		{ID: "7e8facc6-1111-2222-3333-444455556666", NameZh: "吧台", SortOrder: 1},
		{ID: "2951b0b2-aaaa-bbbb-cccc-ddddeeeeffff", NameZh: "厨房", SortOrder: 0},
	}
	got := BuildPrinterStationList(mapped, meta)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Label != "厨房" || got[0].Printer != "tcp:172.20.10.3:9100" {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Label != "吧台" {
		t.Fatalf("second label=%q", got[1].Label)
	}
	if got[2].Label != "orphan-u…" {
		t.Fatalf("orphan label=%q", got[2].Label)
	}
}

func TestStationDisplayLabel_prefersZh(t *testing.T) {
	got := StationDisplayLabel(StationMeta{
		NameZh: "厨房", NameEn: "Kitchen", NamePt: "Cozinha",
	}, "x")
	if got != "厨房" {
		t.Fatalf("got %q", got)
	}
}

func TestStationDisplayLabel_fallbackID(t *testing.T) {
	got := StationDisplayLabel(StationMeta{}, "fb0d1333-aaaa-bbbb-cccc-ddddeeeeffff")
	if got != "fb0d1333…" {
		t.Fatalf("got %q", got)
	}
}
