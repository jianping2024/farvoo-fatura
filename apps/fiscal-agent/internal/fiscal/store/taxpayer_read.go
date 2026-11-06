package store

import (
	"database/sql"
	"strings"
)

// TaxpayerSettings is the ONLY read model for taxpayer_settings (SAF-T header, issue).
// Program identity (certificate, product id/version) is package software, not a store setting.
type TaxpayerSettings struct {
	StoreID               string
	TaxRegistrationNumber string
	LegalName             string
	BusinessName          string
	AddressDetail         string
	City                  string
	PostalCode            string
	Country               string
	Timezone              string
	TaxCountryRegion      string
}

// GetTaxpayerSettings loads taxpayer_settings for a store.
func (d *DB) GetTaxpayerSettings(storeID string) (*TaxpayerSettings, error) {
	var t TaxpayerSettings
	err := d.SQL.QueryRow(`SELECT store_id, tax_registration_number, legal_name, COALESCE(business_name,''),
		address_detail, city, postal_code, country, timezone,
		tax_country_region
		FROM taxpayer_settings WHERE store_id = ?`, storeID).
		Scan(&t.StoreID, &t.TaxRegistrationNumber, &t.LegalName, &t.BusinessName,
			&t.AddressDetail, &t.City, &t.PostalCode, &t.Country, &t.Timezone,
			&t.TaxCountryRegion)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// StoreDisplayName returns business_name or legal_name for UI — ONLY store display name read path.
func (d *DB) StoreDisplayName(storeID string) (string, error) {
	var business, legal string
	err := d.SQL.QueryRow(`SELECT COALESCE(business_name,''), legal_name FROM taxpayer_settings WHERE store_id=?`, storeID).
		Scan(&business, &legal)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(business) != "" {
		return strings.TrimSpace(business), nil
	}
	return strings.TrimSpace(legal), nil
}
