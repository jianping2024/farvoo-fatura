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

// ActiveSeriesCodeForDocType returns the ACTIVE series_code for store+document_type, or ErrNotFound.
func (d *DB) ActiveSeriesCodeForDocType(storeID, docType string) (seriesCode string, err error) {
	err = d.SQL.QueryRow(`
		SELECT series_code FROM series
		WHERE store_id=? AND document_type=? AND status='ACTIVE'
		  AND validation_code IS NOT NULL AND validation_code != ''
		ORDER BY fiscal_year DESC, updated_at DESC LIMIT 1`, storeID, docType).Scan(&seriesCode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return seriesCode, err
}

// ActiveSeriesHasCode reports whether store+series_code is ACTIVE with a validation_code.
func (d *DB) ActiveSeriesHasCode(storeID, seriesCode string) (bool, error) {
	var n int
	err := d.SQL.QueryRow(`
		SELECT COUNT(1) FROM series
		WHERE store_id=? AND series_code=? AND status='ACTIVE'
		  AND validation_code IS NOT NULL AND validation_code != ''`, storeID, seriesCode).Scan(&n)
	return n > 0, err
}

// UpsertOperator creates cashier/owner used as invoices.source_id.
func (d *DB) UpsertOperator(id, storeID, role, displayName, mesaUserID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if mesaUserID == "" {
		mesaUserID = "mesa-" + id
	}
	canNC := 0
	if role == "owner" {
		canNC = 1
	}
	_, err := d.SQL.Exec(`INSERT INTO operators (
		id, mesa_user_id, store_id, role, display_name, active, pin_hash, can_issue_nc, synced_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 1, NULL, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET display_name=excluded.display_name, role=excluded.role, active=1,
		can_issue_nc=CASE WHEN excluded.role='owner' THEN 1 ELSE operators.can_issue_nc END,
		updated_at=excluded.updated_at`,
		id, mesaUserID, storeID, role, displayName, canNC, now, now, now)
	return err
}

// SetOperatorCanIssueNC is the ONLY write path for operators.can_issue_nc.
func (d *DB) SetOperatorCanIssueNC(storeID, operatorID string, canIssue bool) error {
	v := 0
	if canIssue {
		v = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE operators SET can_issue_nc=?, updated_at=? WHERE store_id=? AND id=? AND active=1`,
		v, now, storeID, operatorID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// OperatorCanIssueNC returns whether operator may issue credit notes.
func (d *DB) OperatorCanIssueNC(storeID, operatorID string) (bool, error) {
	var v int
	err := d.SQL.QueryRow(`SELECT can_issue_nc FROM operators WHERE store_id=? AND id=? AND active=1`,
		storeID, operatorID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return v == 1, nil
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

// ClearLocalActivation retires ACTIVE signing_keys after cloud revoke/not-active — ONLY local deactivate path.
func (d *DB) ClearLocalActivation() error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.Exec(`UPDATE signing_keys SET status='RETIRED', retired_at=? WHERE status='ACTIVE'`, now); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE agent_installations SET revoked_at=? WHERE revoked_at IS NULL`, now); err != nil {
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
	NCSeriesOK            bool   `json:"nc_series_ok"`
	NDSeriesOK            bool   `json:"nd_series_ok"`
	FSSeriesOK            bool   `json:"fs_series_ok"`
	FRSeriesOK            bool   `json:"fr_series_ok"`
	ActivatedOK           bool   `json:"activated_ok"`
	OperatorOK            bool   `json:"operator_ok"`
	SeriesCode            string `json:"series_code,omitempty"`
	Validation            string `json:"validation_code,omitempty"`
	NCSeriesCode          string `json:"nc_series_code,omitempty"`
	NCValidation          string `json:"nc_validation_code,omitempty"`
	NDSeriesCode          string `json:"nd_series_code,omitempty"`
	NDValidation          string `json:"nd_validation_code,omitempty"`
	FSSeriesCode          string `json:"fs_series_code,omitempty"`
	FSValidation          string `json:"fs_validation_code,omitempty"`
	FRSeriesCode          string `json:"fr_series_code,omitempty"`
	FRValidation          string `json:"fr_validation_code,omitempty"`
	ReadyToIssue          bool   `json:"ready_to_issue"`
	ReadyToCredit         bool   `json:"ready_to_credit"`
	ReadyToDebit          bool   `json:"ready_to_debit"`
	OperatorCanIssueNC    bool   `json:"operator_can_issue_nc"`
	LocalProvisionAllowed bool   `json:"local_provision_allowed"`
	FiscalProfileOK       bool   `json:"fiscal_profile_ok"`
	FiscalProfile         string `json:"fiscal_profile,omitempty"`
	MaxFiscalTerminals    int    `json:"max_fiscal_terminals"`
	TerminalsUsed         int    `json:"terminals_used"`
	CloudPairedOK         bool   `json:"cloud_paired_ok"`
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
	var ncCode, ncVal sql.NullString
	err = d.SQL.QueryRow(`SELECT series_code, validation_code FROM series
		WHERE store_id=? AND document_type='NC' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != ''
		ORDER BY fiscal_year DESC LIMIT 1`, storeID).Scan(&ncCode, &ncVal)
	if err == nil {
		s.NCSeriesOK = true
		s.NCSeriesCode = ncCode.String
		s.NCValidation = ncVal.String
	}
	var ndCode, ndVal sql.NullString
	err = d.SQL.QueryRow(`SELECT series_code, validation_code FROM series
		WHERE store_id=? AND document_type='ND' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != ''
		ORDER BY fiscal_year DESC LIMIT 1`, storeID).Scan(&ndCode, &ndVal)
	if err == nil {
		s.NDSeriesOK = true
		s.NDSeriesCode = ndCode.String
		s.NDValidation = ndVal.String
	}
	var fsCode, fsVal sql.NullString
	err = d.SQL.QueryRow(`SELECT series_code, validation_code FROM series
		WHERE store_id=? AND document_type='FS' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != ''
		ORDER BY fiscal_year DESC LIMIT 1`, storeID).Scan(&fsCode, &fsVal)
	if err == nil {
		s.FSSeriesOK = true
		s.FSSeriesCode = fsCode.String
		s.FSValidation = fsVal.String
	}
	var frCode, frVal sql.NullString
	err = d.SQL.QueryRow(`SELECT series_code, validation_code FROM series
		WHERE store_id=? AND document_type='FR' AND status='ACTIVE' AND validation_code IS NOT NULL AND validation_code != ''
		ORDER BY fiscal_year DESC LIMIT 1`, storeID).Scan(&frCode, &frVal)
	if err == nil {
		s.FRSeriesOK = true
		s.FRSeriesCode = frCode.String
		s.FRValidation = frVal.String
	}
	_ = d.SQL.QueryRow(`SELECT COUNT(1) FROM signing_keys WHERE status='ACTIVE'`).Scan(&n)
	s.ActivatedOK = n > 0
	n, err = d.CountActiveOperatorsWithPIN(storeID)
	if err == nil {
		s.OperatorOK = n > 0
	}
	profileOK, profile, maxTerm, err := d.FiscalProfileOK(storeID)
	if err == nil {
		s.FiscalProfileOK = profileOK
		s.FiscalProfile = profile
		s.MaxFiscalTerminals = maxTerm
	}
	used, _ := d.CountActiveFiscalTerminals(storeID)
	s.TerminalsUsed = used
	s.LocalProvisionAllowed = os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") == "1"
	s.ReadyToIssue = s.TaxpayerOK && s.SeriesOK && s.FSSeriesOK && s.ActivatedOK && s.OperatorOK && s.FiscalProfileOK
	// ready_to_* includes can_issue_nc so Admin checklist matches detail Credit/Debit buttons.
	s.ReadyToCredit = s.NCSeriesOK && s.ActivatedOK && s.OperatorOK && s.OperatorCanIssueNC
	s.ReadyToDebit = s.NDSeriesOK && s.ActivatedOK && s.OperatorOK && s.OperatorCanIssueNC
	return s, nil
}
