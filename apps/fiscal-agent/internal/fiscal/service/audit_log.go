package service

import (
	"farvoo-fiscal-agent/internal/fiscal/audit"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// AuditLogListInput is input for the ONLY audit log list orchestration.
type AuditLogListInput struct {
	Page        int
	PageSize    int
	Action      string
	OperatorID  string
	From        string
	To          string
	ViewerRole  string
}

// AuditLogItem is one audit log API row.
type AuditLogItem struct {
	ID                    string `json:"id"`
	At                    string `json:"at"`
	OperatorID            string `json:"operator_id,omitempty"`
	OperatorDisplayName   string `json:"operator_display_name"`
	Action                string `json:"action"`
	ActionLabel           string `json:"action_label"`
	Summary               string `json:"summary"`
}

// AuditLogListResult is the audit log list API payload.
type AuditLogListResult struct {
	Items         []AuditLogItem      `json:"items"`
	Page          int                 `json:"page"`
	PageSize      int                 `json:"page_size"`
	Total         int                 `json:"total"`
	FilterActions []audit.FilterAction `json:"filter_actions"`
}

// ListAuditLog is the ONLY audit log list orchestration entry.
func (s *FiscalService) ListAuditLog(in AuditLogListInput) (*AuditLogListResult, error) {
	ownerFilter := in.ViewerRole == "owner"
	raw, err := s.db.ListAuditLog(store.AuditLogQuery{
		Page: in.Page, PageSize: in.PageSize, Action: in.Action,
		OperatorID: in.OperatorID, From: in.From, To: in.To,
		OwnerFilter: ownerFilter,
	})
	if err != nil {
		return nil, err
	}
	items := make([]AuditLogItem, 0, len(raw.Items))
	for _, row := range raw.Items {
		opName := row.OperatorDisplayName
		if row.OperatorID == "" {
			opName = "系统"
		} else if opName == "" {
			opName = row.OperatorID
		}
		summaryTarget := opName
		if row.EntityType == "operator" && row.OperatorDisplayName != "" {
			summaryTarget = row.OperatorDisplayName
		}
		items = append(items, AuditLogItem{
			ID:                  row.ID,
			At:                  row.At,
			OperatorID:          row.OperatorID,
			OperatorDisplayName: opName,
			Action:              row.Action,
			ActionLabel:           audit.ActionLabel(row.Action),
			Summary:               audit.FormatSummary(row.EntityType, row.EntityID, row.DetailJSON, summaryTarget),
		})
	}
	return &AuditLogListResult{
		Items: items, Page: raw.Page, PageSize: raw.PageSize, Total: raw.Total,
		FilterActions: audit.FilterActionsForRole(in.ViewerRole),
	}, nil
}
