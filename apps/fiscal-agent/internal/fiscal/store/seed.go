package store

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// SeedDemoOperatorPIN is the ONLY known PIN for FISCAL_SEED demo cashier (fiscal-local / UAT).
const SeedDemoOperatorPIN = "123456"

// SeedDemoParams seeds the minimum rows for local FT issuance (no AT SOAP).
type SeedDemoParams struct {
	StoreID                   string
	TaxpayerNIF               string
	LegalName                 string
	Address                   string
	City                      string
	PostalCode                string
	Timezone                  string
	SoftwareCertificateNumber string
	ProductID                 string
	ProductVersion            string
	SeriesCode                string
	ValidationCode            string // 8-char AT code (seeded for offline)
	FiscalYear                int
	OperatorID                string
	OperatorName              string
	SigningKeyVersion         int
	PublicKeyPEM              string
	WrappedPrivateKey         string // P0/dev: may store PEM marker; real wrap later
	InstallationID            string
	DeviceID                  string
	DevicePublicKey           string
}

// SeedDemo inserts identity + ACTIVE FT/FS series + consumidor final if missing.
// Demo cashier is inserted with SeedDemoOperatorPIN so Admin login is not stuck on pinless rows.
func (d *DB) SeedDemo(p SeedDemoParams) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.Timezone == "" {
		p.Timezone = "Europe/Lisbon"
	}
	if p.SoftwareCertificateNumber == "" {
		p.SoftwareCertificateNumber = "0"
	}
	if p.ProductID == "" {
		p.ProductID = "Farvoo/InvoiceEngine"
	}
	if p.ProductVersion == "" {
		p.ProductVersion = "0.0.0"
	}
	if p.SigningKeyVersion == 0 {
		p.SigningKeyVersion = 1
	}

	pinHash, err := hashPIN(SeedDemoOperatorPIN)
	if err != nil {
		return fmt.Errorf("seed operator pin: %w", err)
	}

	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`INSERT OR IGNORE INTO taxpayer_settings (
		id, store_id, tax_registration_number, legal_name, business_name,
		address_detail, city, postal_code, country, timezone, phone,
		software_certificate_number, product_id, product_version,
		fs_amount_threshold, tax_country_region, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'PT', ?, NULL, ?, ?, ?, '100.00', 'PT', ?, ?)`,
		"tax-"+p.StoreID, p.StoreID, p.TaxpayerNIF, p.LegalName, p.LegalName,
		p.Address, p.City, p.PostalCode, p.Timezone,
		p.SoftwareCertificateNumber, p.ProductID, p.ProductVersion, now, now)
	if err != nil {
		return fmt.Errorf("seed taxpayer: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO signing_keys (
		id, key_version, public_key_pem, wrapped_private_key, status, created_at, retired_at, submitted_to_at_at
	) VALUES (?, ?, ?, ?, 'ACTIVE', ?, NULL, NULL)`,
		fmt.Sprintf("sk-%d", p.SigningKeyVersion), p.SigningKeyVersion, p.PublicKeyPEM, p.WrappedPrivateKey, now)
	if err != nil {
		return fmt.Errorf("seed signing_keys: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO agent_installations (
		installation_id, store_id, taxpayer_nif, device_id, device_public_key,
		hardware_fingerprint, key_protection_level, signing_key_version, provisioned_at, revoked_at
	) VALUES (?, ?, ?, ?, ?, NULL, 'SOFTWARE', ?, ?, NULL)`,
		p.InstallationID, p.StoreID, p.TaxpayerNIF, p.DeviceID, p.DevicePublicKey, p.SigningKeyVersion, now)
	if err != nil {
		return fmt.Errorf("seed installation: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO operators (
		id, mesa_user_id, store_id, role, display_name, active, pin_hash, can_issue_nc, synced_at, created_at, updated_at
	) VALUES (?, ?, ?, 'cashier', ?, 1, ?, 0, ?, ?, ?)`,
		p.OperatorID, "mesa-"+p.OperatorID, p.StoreID, p.OperatorName, pinHash, now, now, now)
	if err != nil {
		return fmt.Errorf("seed operator: %w", err)
	}
	// Re-seed of older DBs: pinless demo cashier blocked Admin login (bootstrap_required=false + empty login list).
	_, err = tx.Exec(`UPDATE operators SET pin_hash=?, updated_at=?
		WHERE id=? AND store_id=? AND (pin_hash IS NULL OR pin_hash='')`,
		pinHash, now, p.OperatorID, p.StoreID)
	if err != nil {
		return fmt.Errorf("seed operator pin backfill: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO customers (
		id, customer_tax_id, company_name, address_detail, city, postal_code, country,
		account_id, self_billing_indicator, completeness_status, created_at, updated_at
	) VALUES ('cust-final', '999999990', 'Consumidor Final', 'Desconhecido', 'Desconhecido', 'Desconhecido', 'PT',
		'Desconhecido', 0, 'SYSTEM_DEFAULT', ?, ?)`, now, now)
	if err != nil {
		return fmt.Errorf("seed customer: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO series (
		id, store_id, document_type, series_code, validation_code, fiscal_year,
		last_number, last_hash, status, registered_at, created_at, updated_at
	) VALUES (?, ?, 'FT', ?, ?, ?, 0, '', 'ACTIVE', ?, ?, ?)`,
		"series-ft-"+p.SeriesCode, p.StoreID, p.SeriesCode, p.ValidationCode, p.FiscalYear, now, now, now)
	if err != nil {
		return fmt.Errorf("seed series: %w", err)
	}

	fsCode := strings.Replace(p.SeriesCode, "FT", "FS", 1)
	if fsCode == p.SeriesCode {
		fsCode = "FS" + p.SeriesCode
	}
	_, err = tx.Exec(`INSERT OR IGNORE INTO series (
		id, store_id, document_type, series_code, validation_code, fiscal_year,
		last_number, last_hash, status, registered_at, created_at, updated_at
	) VALUES (?, ?, 'FS', ?, ?, ?, 0, '', 'ACTIVE', ?, ?, ?)`,
		"series-fs-"+fsCode, p.StoreID, fsCode, p.ValidationCode, p.FiscalYear, now, now, now)
	if err != nil {
		return fmt.Errorf("seed fs series: %w", err)
	}

	return tx.Commit()
}

// SeedDemoFromKeyFile seeds using a PEM private key file as wrapped placeholder + pub PEM.
func (d *DB) SeedDemoFromKeyFile(p SeedDemoParams, privPEMPath string, pubPEM string) error {
	b, err := os.ReadFile(privPEMPath)
	if err != nil {
		return err
	}
	p.PublicKeyPEM = pubPEM
	p.WrappedPrivateKey = "DEV_PLAIN:" + string(b) // P0 offline only; TPM wrap replaces this
	return d.SeedDemo(p)
}
