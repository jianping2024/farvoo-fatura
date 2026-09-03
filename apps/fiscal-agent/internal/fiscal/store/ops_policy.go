package store

import (
	"database/sql"
	"errors"
	"time"
)

// OpsStorePolicy is the cached Ops fiscal store policy — ONLY policy row shape.
type OpsStorePolicy struct {
	FiscalProfile      string
	MaxFiscalTerminals int
	OpsPolicySyncedAt  string
}

// SaveOpsFiscalProfile writes Ops fiscal_profile only — does NOT touch max_fiscal_terminals
// (max is local admin authority; see SetMaxFiscalTerminals).
func (d *DB) SaveOpsFiscalProfile(storeID, fiscalProfile string) error {
	if fiscalProfile != "restaurant" && fiscalProfile != "retail" {
		return errors.New("store: invalid fiscal_profile")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE taxpayer_settings SET
		fiscal_profile=?, ops_policy_synced_at=?, updated_at=?
		WHERE store_id=?`, fiscalProfile, now, now, storeID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOpsStorePolicy reads cached profile + local max for storeID.
func (d *DB) GetOpsStorePolicy(storeID string) (*OpsStorePolicy, error) {
	var profile sql.NullString
	var max sql.NullInt64
	var synced sql.NullString
	err := d.SQL.QueryRow(`SELECT fiscal_profile, max_fiscal_terminals, ops_policy_synced_at
		FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&profile, &max, &synced)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := &OpsStorePolicy{}
	if profile.Valid {
		out.FiscalProfile = profile.String
	}
	if max.Valid && max.Int64 > 0 {
		out.MaxFiscalTerminals = int(max.Int64)
	} else {
		out.MaxFiscalTerminals = 1
	}
	if synced.Valid {
		out.OpsPolicySyncedAt = synced.String
	}
	return out, nil
}

// FiscalProfileOK reports whether Ops policy has been synced with a profile.
func (d *DB) FiscalProfileOK(storeID string) (bool, string, int, error) {
	p, err := d.GetOpsStorePolicy(storeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, "", 1, nil
		}
		return false, "", 1, err
	}
	ok := p.FiscalProfile == "restaurant" || p.FiscalProfile == "retail"
	return ok, p.FiscalProfile, p.MaxFiscalTerminals, nil
}
