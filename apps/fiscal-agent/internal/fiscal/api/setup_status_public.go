package api

import "farvoo-fiscal-agent/internal/fiscal/store"

// SetupStatusPublic is the ONLY anonymous GET /setup/status JSON shape.
type SetupStatusPublic struct {
	BootstrapRequired bool   `json:"bootstrap_required"`
	OperatorsCount    int    `json:"operators_count"`
	FiscalProfile     string `json:"fiscal_profile,omitempty"`
	StoreDisplayName  string `json:"store_display_name,omitempty"`
	ReadyToIssue      bool   `json:"ready_to_issue"`
	CloudPairedOK     bool   `json:"cloud_paired_ok"`
	OpsFiscalEntitled bool   `json:"ops_fiscal_entitled"`
}

// BuildSetupStatusPublic maps full status + store fields — ONLY anonymous status builder.
// BootstrapRequired is true only for true greenfield (this store empty AND no operators under any
// other store_id) AND opsEntitled (scheme A: Ops fiscal_profile, or local-provision UAT).
func BuildSetupStatusPublic(storeID string, full *store.SetupStatus, db *store.DB, opsEntitled, cloudPaired bool, opsProfile string) (*SetupStatusPublic, error) {
	if full == nil {
		return &SetupStatusPublic{CloudPairedOK: cloudPaired, OpsFiscalEntitled: opsEntitled}, nil
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
	if bootstrapRequired && !opsEntitled {
		bootstrapRequired = false
	}
	display, _ := db.StoreDisplayName(storeID)
	profile := full.FiscalProfile
	if profile == "" {
		profile = opsProfile
	}
	return &SetupStatusPublic{
		BootstrapRequired: bootstrapRequired,
		OperatorsCount:    count,
		FiscalProfile:     profile,
		StoreDisplayName:  display,
		ReadyToIssue:      full.ReadyToIssue,
		CloudPairedOK:     cloudPaired,
		OpsFiscalEntitled: opsEntitled,
	}, nil
}
