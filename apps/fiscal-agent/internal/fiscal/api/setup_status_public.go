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
// BootstrapRequired is true only for true greenfield (this store empty AND no operators under any other store_id).
func BuildSetupStatusPublic(storeID string, full *store.SetupStatus, db *store.DB) (*SetupStatusPublic, error) {
	if full == nil {
		return &SetupStatusPublic{}, nil
	}
	count, err := db.CountOperators(storeID)
	if err != nil {
		return nil, err
	}
	bootstrapRequired := count == 0
	if bootstrapRequired {
		other, err := db.CountOperatorsExcludingStore(storeID)
		if err != nil {
			return nil, err
		}
		if other > 0 {
			bootstrapRequired = false
		}
	}
	display, _ := db.StoreDisplayName(storeID)
	return &SetupStatusPublic{
		BootstrapRequired: bootstrapRequired,
		OperatorsCount:    count,
		FiscalProfile:     full.FiscalProfile,
		StoreDisplayName:  display,
		ReadyToIssue:      full.ReadyToIssue,
	}, nil
}
