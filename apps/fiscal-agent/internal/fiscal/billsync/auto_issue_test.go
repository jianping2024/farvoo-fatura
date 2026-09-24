package billsync_test

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
)

func TestResolveAutoIssueMode_WholeTable(t *testing.T) {
	mode, scope, err := billsync.ResolveAutoIssueMode(billsync.Snapshot{ScopeType: "whole_table"})
	if err != nil || mode != "whole_table" || scope != "" {
		t.Fatalf("got mode=%q scope=%q err=%v", mode, scope, err)
	}
}

func TestResolveAutoIssueMode_SplitNeedsScope(t *testing.T) {
	_, _, err := billsync.ResolveAutoIssueMode(billsync.Snapshot{ScopeType: "split"})
	ie := billsync.AsIngestError(err)
	if ie == nil || ie.Code != billsync.CodeValidationFailed {
		t.Fatalf("want validation_failed, got %v", err)
	}
	mode, scope, err := billsync.ResolveAutoIssueMode(billsync.Snapshot{
		ScopeType: "split", IssueScopeID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})
	if err != nil || mode != "person" || scope == "" {
		t.Fatalf("got mode=%q scope=%q err=%v", mode, scope, err)
	}
}

func TestResolveAutoIssueMode_ScopeIDAlias(t *testing.T) {
	mode, scope, err := billsync.ResolveAutoIssueMode(billsync.Snapshot{
		ScopeType: "split", ScopeID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})
	if err != nil || mode != "person" || scope != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("got mode=%q scope=%q err=%v", mode, scope, err)
	}
}
