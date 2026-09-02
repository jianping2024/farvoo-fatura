package api

import "farvoo-fiscal-agent/internal/fiscal/store"

// SetupStatusPublic is the ONLY anonymous GET /setup/status JSON shape.
type SetupStatusPublic struct {
	BootstrapRequired bool   `json:"bootstrap_required"`
	OperatorsCount    int    `json:"operators_count"`
	FiscalProfile     string `json:"fiscal_profile,omitempty"`
	StoreDisplayName  string `json:"store_display_name,omitempty"`
	ReadyToIssue      bool   `json:"ready_to_issue"`
}

// BuildSetupStatusPublic maps full status + store fields — ONLY anonymous status builder.
func BuildSetupStatusPublic(storeID string, full *store.SetupStatus, db *store.DB) (*SetupStatusPublic, error) {
	if full == nil {
		return &SetupStatusPublic{}, nil
	}
	count, err := db.CountOperators(storeID)
	if err != nil {
		return nil, err
	}
	display, _ := db.StoreDisplayName(storeID)
	return &SetupStatusPublic{
		BootstrapRequired: count == 0,
		OperatorsCount:    count,
		FiscalProfile:     full.FiscalProfile,
		StoreDisplayName:  display,
		ReadyToIssue:      full.ReadyToIssue,
	}, nil
}
