package main

import (
	"os"
	"strings"
	"testing"
)

// TestRealtimeSubscribeLocksSingleConnectionMultiTable guards bill-sync-api §3/§7:
// one phx_join must list both print_jobs and bill_sync_jobs — never a second client.
func TestRealtimeSubscribeLocksSingleConnectionMultiTable(t *testing.T) {
	rawB, err := os.ReadFile("realtime.go")
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawB)
	if !strings.Contains(raw, `"print_jobs"`) {
		t.Fatal("realtime subscribe must include print_jobs")
	}
	if !strings.Contains(raw, `"bill_sync_jobs"`) {
		t.Fatal("realtime subscribe must include bill_sync_jobs on the same join")
	}
	if strings.Count(raw, "func (r *RealtimeNotifier) subscribe()") != 1 {
		t.Fatal("expected exactly one subscribe method")
	}
	if strings.Count(raw, "DialContext") != 1 {
		t.Fatal("expected exactly one websocket DialContext — single Realtime connection")
	}
	if !strings.Contains(raw, "pullBillSyncsOnce") {
		t.Fatal("compensation/doorbell must call pullBillSyncsOnce")
	}
	pollB, err := os.ReadFile("polling.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pollB), "pullBillSyncsOnce") {
		t.Fatal("polling fallback must call pullBillSyncsOnce in the same fetch loop")
	}
}
