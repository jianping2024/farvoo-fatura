package billsync_test

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
)

func TestListGrossTotalFromPayload(t *testing.T) {
	t.Parallel()
	whole := `{"scope_type":"whole_table","gross_total":"43.95"}`
	if got := billsync.ListGrossTotalFromPayload(whole); got != "43.95" {
		t.Fatalf("whole_table: got %q", got)
	}
	split := `{"scope_type":"split","splits":[{"gross_total":"4.50"},{"gross_total":"2.25"}]}`
	if got := billsync.ListGrossTotalFromPayload(split); got != "6.75" {
		t.Fatalf("split sum: got %q", got)
	}
}
