package billsync

import (
	"strings"
)

// ResolveAutoIssueMode is the ONLY resolver for Farvoo auto_issue issue mode + person scope.
// whole_table payload → mode whole_table; split payload → mode person (requires scope id).
func ResolveAutoIssueMode(snap Snapshot) (mode, scopeID string, err error) {
	mode = strings.TrimSpace(snap.IssueMode)
	scopeID = strings.TrimSpace(snap.IssueScopeID)
	if scopeID == "" {
		scopeID = strings.TrimSpace(snap.ScopeID)
	}
	payloadType := strings.TrimSpace(snap.ScopeType)
	if mode == "" {
		switch payloadType {
		case "whole_table":
			mode = "whole_table"
		case "split":
			mode = "person"
		default:
			return "", "", ingestErr(CodeValidationFailed, "scope_type required for auto_issue")
		}
	}
	if mode != "whole_table" && mode != "person" {
		return "", "", ingestErr(CodeValidationFailed, "issue_mode must be whole_table or person")
	}
	if mode == "whole_table" && payloadType != "whole_table" {
		return "", "", ingestErr(CodeValidationFailed, "mode=whole_table requires payload scope_type=whole_table")
	}
	if mode == "person" {
		if scopeID == "" {
			return "", "", ingestErr(CodeValidationFailed, "issue_scope_id or scope_id required for person auto_issue")
		}
		if payloadType != "split" && payloadType != "whole_table" {
			return "", "", ingestErr(CodeValidationFailed, "person auto_issue requires split or whole_table payload")
		}
	}
	return mode, scopeID, nil
}

// NewIngestError builds a typed ingest/ack error (Puller + auto_issue adapters).
func NewIngestError(code, msg string) error {
	return ingestErr(code, msg)
}
