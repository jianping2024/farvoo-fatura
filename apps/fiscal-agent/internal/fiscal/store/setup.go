package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// UpsertTaxpayer writes taxpayer_settings for a store (ONLY taxpayer write path).
func (d *DB) UpsertTaxpayer(p TaxpayerInput) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var id string
	err := d.SQL.QueryRow(`SELECT id FROM taxpayer_settings WHERE store_id = ?`, p.StoreID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		id = "tax-" + uuid.NewString()
		_, err = d.SQL.Exec(`INSERT INTO taxpayer_settings (
			id, store_id, tax_registration_number, legal_name, business_name,
			address_detail, city, postal_code, country, timezone, phone,
			software_certificate_number, product_id, product_version,
			fs_amount_threshold, tax_country_region, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, 'Farvoo/InvoiceEngine', '0.0.0', '100.00', 'PT', ?, ?)`,
			id, p.StoreID, p.TaxRegistrationNumber, p.LegalName, nullStr(p.BusinessName),
			p.AddressDetail, p.City, p.PostalCode, p.Country, p.Timezone,
			p.SoftwareCertificateNumber, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`UPDATE taxpayer_settings SET
		tax_registration_number=?, legal_name=?, business_name=?, address_detail=?, city=?, postal_code=?,
		country=?, timezone=?, software_certificate_number=?, updated_at=?
		WHERE store_id=?`,
		p.TaxRegistrationNumber, p.LegalName, nullStr(p.BusinessName), p.AddressDetail, p.City, p.PostalCode,
		p.Country, p.Timezone, p.SoftwareCertificateNumber, now, p.StoreID)
	return err
}

// TaxpayerInput is setup form data.
type TaxpayerInput struct {
	StoreID                   string `json:"store_id"`
	TaxRegistrationNumber     string `json:"tax_registration_number"`
	LegalName                 string `json:"legal_name"`
	BusinessName              string `json:"business_name"`
	AddressDetail             string `json:"address_detail"`
	City                      string `json:"city"`
	PostalCode                string `json:"postal_code"`
	Country                   string `json:"country"`
	Timezone                  string `json:"timezone"`
	SoftwareCertificateNumber string `json:"software_certificate_number"`
}

// UpsertATCredentials stores sealed AT password (ONLY at_credentials write path).
func (d *DB) UpsertATCredentials(storeID, username, ciphertext, wrapMeta string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var id string
	err := d.SQL.QueryRow(`SELECT id FROM at_credentials WHERE store_id = ?`, storeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = d.SQL.Exec(`INSERT INTO at_credentials (
			id, store_id, username, password_ciphertext, salt, wrap_meta, last_ok_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, NULL, ?, NULL, NULL, ?, ?)`,
			"atc-"+uuid.NewString(), storeID, username, ciphertext, wrapMeta, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`UPDATE at_credentials SET username=?, password_ciphertext=?, salt=NULL, wrap_meta=?,
		last_error=NULL, updated_at=? WHERE store_id=?`,
		username, ciphertext, wrapMeta, now, storeID)
	return err
}

// ATCredentialRow is stored AT login.
type ATCredentialRow struct {
	Username           string
	PasswordCiphertext string
	WrapMeta           string
}

// GetATCredentials loads sealed credentials.
func (d *DB) GetATCredentials(storeID string) (*ATCredentialRow, error) {
	var r ATCredentialRow
	err := d.SQL.QueryRow(`SELECT username, password_ciphertext, COALESCE(wrap_meta,'') FROM at_credentials WHERE store_id = ?`,
		storeID).Scan(&r.Username, &r.PasswordCiphertext, &r.WrapMeta)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// MarkATCredentialsOK updates last_ok_at and clears last_error.
func (d *DB) MarkATCredentialsOK(storeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.SQL.Exec(`UPDATE at_credentials SET last_ok_at=?, last_error=NULL, updated_at=? WHERE store_id=?`,
		now, now, storeID)
	return err
}

// MarkATCredentialsError stores a sanitized error.
func (d *DB) MarkATCredentialsError(storeID, msg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	_, err := d.SQL.Exec(`UPDATE at_credentials SET last_error=?, updated_at=? WHERE store_id=?`, msg, now, storeID)
	return err
}

// UpsertActiveSeries writes/updates an ACTIVE series with validation_code (ONLY series registration write).
func (d *DB) UpsertActiveSeries(storeID, docType, seriesCode, validationCode string, fiscalYear int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var id string
	err := d.SQL.QueryRow(`SELECT id FROM series WHERE store_id=? AND series_code=?`, storeID, seriesCode).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = d.SQL.Exec(`INSERT INTO series (
			id, store_id, document_type, series_code, validation_code, fiscal_year,
			last_number, last_hash, status, registered_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 0, '', 'ACTIVE', ?, ?, ?)`,
			"series-"+uuid.NewString(), storeID, docType, seriesCode, validationCode, fiscalYear, now, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`UPDATE series SET validation_code=?, status='ACTIVE', document_type=?, fiscal_year=?,
		registered_at=COALESCE(registered_at, ?), updated_at=? WHERE id=?`,
		validationCode, docType, fiscalYear, now, now, id)
	return err
}

// UpsertOperator creates cashier/owner used as invoices.source_id.
func (d *DB) UpsertOperator(id, storeID, role, displayName, mesaUserID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if mesaUserID == "" {
		mesaUserID = "mesa-" + id
	}
	_, err := d.SQL.Exec(`INSERT INTO operators (
		id, mesa_user_id, store_id, role, display_name, active, pin_hash, can_issue_nc, synced_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 1, NULL, 0, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET display_name=excluded.display_name, role=excluded.role, active=1, updated_at=excluded.updated_at`,
		id, mesaUserID, storeID, role, displayName, now, now, now)
	return err
}

// EnsureConsumidorFinal inserts the system default customer once.
func (d *DB) EnsureConsumidorFinal() error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.SQL.Exec(`INSERT OR IGNORE INTO customers (
		id, customer_tax_id, company_name, address_detail, city, postal_code, country,
		account_id, self_billing_indicator, completeness_status, created_at, updated_at
	) VALUES ('cust-final', '999999990', 'Consumidor Final', 'Desconhecido', 'Desconhecido', 'Desconhecido', 'PT',
		'Desconhecido', 0, 'SYSTEM_DEFAULT', ?, ?)`, now, now)
	return err
}

// SaveActivation writes installation + signing_keys (ONLY activation write path).
func (d *DB) SaveActivation(p ActivationInput) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`INSERT OR REPLACE INTO signing_keys (
		id, key_version, public_key_pem, wrapped_private_key, status, created_at, retired_at, submitted_to_at_at
	) VALUES (?, ?, ?, ?, 'ACTIVE', ?, NULL, NULL)`,
		fmt.Sprintf("sk-%d", p.KeyVersion), p.KeyVersion, p.PublicKeyPEM, p.WrappedPrivateKey, now)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT OR REPLACE INTO agent_installations (
		installation_id, store_id, taxpayer_nif, device_id, device_public_key,
		hardware_fingerprint, key_protection_level, signing_key_version, provisioned_at, revoked_at
	) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, NULL)`,
		p.InstallationID, p.StoreID, p.TaxpayerNIF, p.DeviceID, p.DevicePublicKey,
		p.KeyProtectionLevel, p.KeyVersion, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ActivationInput is persisted at activate time.
type ActivationInput struct {
	InstallationID      string
	StoreID             string
	TaxpayerNIF         string
	DeviceID            string
	DevicePublicKey     string
	KeyProtectionLevel  string
	KeyVersion          int
	PublicKeyPEM        string
	WrappedPrivateKey   string
}

// ActiveSigningKey loads ACTIVE product key row.
func (d *DB) ActiveSigningKey() (wrapped string, pub string, version int, err error) {
	err = d.SQL.QueryRow(`SELECT wrapped_private_key, public_key_pem, key_version FROM signing_keys
		WHERE status='ACTIVE' ORDER BY key_version DESC LIMIT 1`).Scan(&wrapped, &pub, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", 0, ErrNotFound
	}
	return wrapped, pub, version, err
}

// SetupStatus is GET /setup/status payload source.
type SetupStatus struct {
	TaxpayerOK            bool   `json:"taxpayer_ok"`
	ATCredsOK             bool   `json:"at_credentials_ok"`
	SeriesOK              bool   `json:"series_ok"`
	ActivatedOK           bool   `json:"activated_ok"`
	OperatorOK            bool   `json:"operator_ok"`
	SeriesCode            string `json:"series_code,omitempty"`
	Validation            string `json:"validation_code,omitempty"`
	ReadyToIssue          bool   `json:"ready_to_issue"`
	LocalProvisionAllowed bool   `json:"local_provision_allowed"`
}

// GetSetupStatus summarizes readiness for storeID.
func (d *DB) GetSetupStatus(storeID string) (*SetupStatus, error) {
	s := &SetupStatus{}
	var n int
	_ = d.SQL.QueryRow(`SELECT COUNT(1) FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&n)
	s.TaxpayerOK = n > 0
	_ = d.SQL.QueryRow(`SELECT COUNT(1) FROM at_credentials WHERE store_id=?`, storeID).Scan(&n)
	s.ATCredsOK = n > 0
	var code, val sql.NullString
	err := d.SQL.QueryRow(`SELECT series_code, validation_code FROM series
		WHERE store_id=? AND document_type='FT' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != ''
		LIMIT 1`, storeID).Scan(&code, &val)
	if err == nil {
		s.SeriesOK = true
		s.SeriesCode = code.String
		s.Validation = val.String
	}
	_ = d.SQL.QueryRow(`SELECT COUNT(1) FROM signing_keys WHERE status='ACTIVE'`).Scan(&n)
	s.ActivatedOK = n > 0
	_ = d.SQL.QueryRow(`SELECT COUNT(1) FROM operators WHERE store_id=? AND active=1`, storeID).Scan(&n)
	s.OperatorOK = n > 0
	s.LocalProvisionAllowed = os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") == "1"
	s.ReadyToIssue = s.TaxpayerOK && s.SeriesOK && s.ActivatedOK && s.OperatorOK
	return s, nil
}
