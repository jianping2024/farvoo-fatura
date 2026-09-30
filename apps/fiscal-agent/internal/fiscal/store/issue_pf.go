package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"

	"github.com/google/uuid"
)

// ErrPFSeriesMissing indicates no ACTIVE PF series with validation_code.
var ErrPFSeriesMissing = errors.New("store: PF series missing")

// ErrProformaNotAllowed indicates PF cannot be linked or annulled in this state.
var ErrProformaNotAllowed = errors.New("store: proforma not allowed")

// IssuePFParams is input for IssuePF (validated by service.IssueProforma).
type IssuePFParams struct {
	StoreID             string
	RequestID           string
	Snapshot            domain.SaleSnapshot
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	InvoiceLocale       string
	ValidUntil          string // optional YYYY-MM-DD
	NowUTC              time.Time
}

// IssuePF is the ONLY SQLite write path for PF (pró-forma) documents.
// Must not be called via IssueFT.
func (d *DB) IssuePF(ctx context.Context, signer Signer, p IssuePFParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	validUntil := strings.TrimSpace(p.ValidUntil)
	if validUntil != "" {
		if _, err := time.Parse("2006-01-02", validUntil); err != nil {
			return nil, fmt.Errorf("store: valid_until must be YYYY-MM-DD")
		}
	}
	payloadHash := pfPayloadHash(p.Snapshot, validUntil)
	businessKey := pfBusinessKey(p.StoreID, p.Snapshot)

	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if rec, hit, err := d.lookupIdempotency(tx, p.StoreID, p.RequestID, businessKey, payloadHash); err != nil {
		return nil, err
	} else if hit {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return rec, nil
	}

	var tz, nif, legalName, businessName, addr, city, postal, cert, taxRegion string
	err = tx.QueryRow(`SELECT timezone, tax_registration_number, legal_name, COALESCE(business_name,''),
		address_detail, city, postal_code, software_certificate_number, tax_country_region
		FROM taxpayer_settings WHERE store_id = ?`, p.StoreID).
		Scan(&tz, &nif, &legalName, &businessName, &addr, &city, &postal, &cert, &taxRegion)
	if err != nil {
		return nil, fmt.Errorf("store: taxpayer_settings: %w", err)
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("Europe/Lisbon", 0)
	}
	localNow := p.NowUTC.In(loc)
	invoiceDate := localNow.Format("2006-01-02")
	systemEntry := localNow.Format("2006-01-02T15:04:05")

	var seriesID, seriesCode, validationCode, lastHash string
	var lastNumber int64
	err = tx.QueryRow(`SELECT id, series_code, validation_code, last_number, last_hash
		FROM series WHERE store_id = ? AND document_type = 'PF' AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash)
	if err != nil {
		return nil, ErrPFSeriesMissing
	}
	if validationCode == "" {
		return nil, ErrPFSeriesMissing
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo("PF", seriesCode, seq)
	atcud := compliance.FormatATCUD(validationCode, seq)

	lines, buckets, gross, net, tax, err := buildLines(p.Snapshot.Lines, taxRegion)
	if err != nil {
		return nil, err
	}
	grossStr := compliance.Money2(gross)
	netStr := compliance.Money2(net)
	taxStr := compliance.Money2(tax)

	signPayload := compliance.BuildSignPayload(invoiceDate, systemEntry, invoiceNo, grossStr, lastHash)
	hashB64, hashControl, keyVersion, err := signer.Sign(signPayload)
	if err != nil {
		return nil, fmt.Errorf("store: sign: %w", err)
	}

	cust := normalizeCustomer(p.Snapshot.Customer)
	custID, err := d.ensureCustomerIDTx(tx, cust)
	if err != nil {
		return nil, err
	}
	qr, err := compliance.BuildQR(compliance.QRInput{
		IssuerNIF:              nif,
		CustomerTaxID:          cust.TaxID,
		CustomerCountry:        cust.Country,
		DocumentType:           "PF",
		DocumentStatus:         "N",
		InvoiceDate:            localNow,
		InvoiceNo:              invoiceNo,
		ATCUD:                  atcud,
		Buckets:                buckets,
		TaxPayable:             taxStr,
		GrossTotal:             grossStr,
		HashBase64:             hashB64,
		SoftwareCertificateNum: cert,
	})
	if err != nil {
		return nil, err
	}

	docID := uuid.NewString()
	printJobID := uuid.NewString()
	idemID := uuid.NewString()
	nowRFC := p.NowUTC.Format(time.RFC3339)

	tableName := ""
	if p.Snapshot.DisplayMeta != nil {
		tableName = p.Snapshot.DisplayMeta["table_display_name"]
	}
	printPayload, payloadHashPrint, err := fiscalprint.BuildPayload(fiscalprint.BuildInput{
		DocumentID:                docID,
		DocumentType:              "PF",
		PrintPurpose:              string(domain.PrintOriginal),
		InvoiceNo:                 invoiceNo,
		IssuedAt:                  systemEntry,
		TableDisplayName:          tableName,
		LegalName:                 legalName,
		BusinessName:              businessName,
		TaxRegistrationNumber:     nif,
		Address:                   fmt.Sprintf("%s, %s %s", addr, postal, city),
		SoftwareCertificateNumber: cert,
		CustomerTaxID:             cust.TaxID,
		CustomerName:              cust.CompanyName,
		CustomerCountry:           cust.Country,
		Lines:                     toPrintLines(lines),
		Buckets:                   buckets,
		NetTotal:                  netStr,
		TaxPayable:                taxStr,
		GrossTotal:                grossStr,
		Payments:                  nil,
		ATCUD:                     atcud,
		QRContent:                 qr,
		Hash:                      hashB64,
		HashControl:               hashControl,
		InvoiceLocale:             p.InvoiceLocale,
	})
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`UPDATE series SET last_number = ?, last_hash = ?, updated_at = ? WHERE id = ?`,
		seq, hashB64, nowRFC, seriesID); err != nil {
		return nil, err
	}

	termID, termLabel, err := requireFiscalTerminalFreeze(p.FiscalTerminalID, p.FiscalTerminalLabel)
	if err != nil {
		return nil, err
	}
	sourceSystem := p.Snapshot.SourceSystem
	if sourceSystem == "" {
		sourceSystem = "LOCAL"
	}

	_, err = tx.Exec(`INSERT INTO invoices (
		id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
		atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
		invoice_date, system_entry_date, document_status, print_status,
		gross_total, net_total, tax_payable, customer_id, source_id,
		software_certificate_number, source_system, source_sale_id, scope_type, scope_id,
		fiscal_purpose, external_bill_id, display_meta_json, credited_gross_total, received_gross_total, created_at,
		fiscal_terminal_id, fiscal_terminal_label, status_reason, status_changed_at, valid_until
	) VALUES (?, ?, 'PF', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N', 'PENDING',
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, '0.00', '0.00', ?, ?, ?, NULL, ?, ?)`,
		docID, p.StoreID, seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, custID, p.OperatorID,
		cert, nullStr(sourceSystem), nullStr(p.Snapshot.SourceSaleID),
		nullStr(p.Snapshot.ScopeType), nullStr(p.Snapshot.ScopeID),
		"proforma", displayMetaJSON(p.Snapshot.DisplayMeta), nowRFC, termID, termLabel,
		systemEntry, nullStr(validUntil))
	if err != nil {
		return nil, fmt.Errorf("store: insert PF invoice: %w", err)
	}

	for _, ln := range lines {
		lineID := uuid.NewString()
		_, err = tx.Exec(`INSERT INTO invoice_lines (
			id, invoice_id, line_number, product_code, product_description, display_name,
			quantity, unit_of_measure, unit_price_gross, unit_price_net, line_gross, line_net, line_tax,
			vat_rate, tax_type, tax_country_region, tax_code, tax_exemption_code, tax_exemption_reason, product_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'IVA', ?, ?, NULL, NULL, ?)`,
			lineID, docID, ln.LineNumber, ln.ProductCode, ln.ProductDescription, nullStr(ln.DisplayName),
			ln.Quantity, ln.UnitOfMeasure, ln.UnitPriceGross, ln.UnitPriceNet, ln.LineGross, ln.LineNet, ln.LineTax,
			ln.VATRate, taxRegion, ln.TaxCode, ln.ProductType)
		if err != nil {
			return nil, fmt.Errorf("store: insert PF line: %w", err)
		}
	}

	_, err = tx.Exec(`INSERT INTO invoice_customer_snapshots (
		invoice_id, customer_tax_id, company_name, address_detail, city, postal_code, country, account_id, self_billing_indicator
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0)`,
		docID, cust.TaxID, cust.CompanyName, cust.AddressDetail, cust.City, cust.PostalCode, cust.Country)
	if err != nil {
		return nil, err
	}

	payloadJSON, err := json.Marshal(printPayload)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO local_print_jobs (
		id, invoice_id, document_type, print_purpose, job_status, logical_role,
		payload_json, payload_hash, attempts, last_error, created_at, updated_at, printed_at, created_by, station_id
	) VALUES (?, ?, 'PF', 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
		printJobID, docID, string(payloadJSON), payloadHashPrint, nowRFC, nowRFC, p.OperatorID, nullStr(p.StationID))
	if err != nil {
		return nil, fmt.Errorf("store: insert print job: %w", err)
	}

	_, err = tx.Exec(`INSERT INTO idempotency_keys (
		id, store_id, request_id, request_payload_hash, business_key, invoice_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		idemID, p.StoreID, p.RequestID, payloadHash, businessKey, docID, nowRFC)
	if err != nil {
		return nil, fmt.Errorf("store: idempotency: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &IssueRecord{
		DocumentID:     docID,
		InvoiceNo:      invoiceNo,
		ATCUD:          atcud,
		DocumentType:   domain.DocumentPF,
		DocumentStatus: domain.WorkStatusNormal,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

func pfPayloadHash(snap domain.SaleSnapshot, validUntil string) string {
	body := struct {
		Snapshot   domain.SaleSnapshot `json:"snapshot"`
		ValidUntil string              `json:"valid_until,omitempty"`
	}{Snapshot: snap, ValidUntil: validUntil}
	return hashJSON(body)
}

func pfBusinessKey(storeID string, snap domain.SaleSnapshot) string {
	sum := sha256.Sum256([]byte(storeID + "|PF|" + snap.SourceSystem + "|" + snap.SourceSaleID + "|" + snap.ScopeType + "|" + snap.ScopeID))
	return "pf:" + hex.EncodeToString(sum[:16])
}

// linkProformaInTx is the ONLY writer for invoice_proforma_references (inside IssueFT tx).
func linkProformaInTx(tx *sql.Tx, storeID, saleInvoiceID, proformaID, statusChangedAt, operatorID string) error {
	pfID := strings.TrimSpace(proformaID)
	if pfID == "" {
		return nil
	}
	var docType, docStatus string
	err := tx.QueryRow(`SELECT document_type, document_status FROM invoices WHERE id = ? AND store_id = ?`,
		pfID, storeID).Scan(&docType, &docStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if docType != string(domain.DocumentPF) {
		return ErrProformaNotAllowed
	}
	if !domain.IsLinkableProformaWorkStatus(docStatus) {
		return ErrProformaNotAllowed
	}
	_, err = tx.Exec(`INSERT INTO invoice_proforma_references (id, sale_invoice_id, proforma_invoice_id, created_at)
		VALUES (?, ?, ?, ?)`, uuid.NewString(), saleInvoiceID, pfID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrProformaNotAllowed
		}
		return fmt.Errorf("store: insert proforma reference: %w", err)
	}
	if docStatus == string(domain.WorkStatusNormal) {
		_, err = tx.Exec(`UPDATE invoices SET document_status = 'F', status_changed_at = ?, source_id = COALESCE(NULLIF(?, ''), source_id)
			WHERE id = ? AND document_status = 'N'`, statusChangedAt, strings.TrimSpace(operatorID), pfID)
		if err != nil {
			return fmt.Errorf("store: mark PF invoiced: %w", err)
		}
	}
	return nil
}

// AnnulPFParams is input for AnnulPF.
type AnnulPFParams struct {
	StoreID    string
	RequestID  string
	DocumentID string
	OperatorID string
	Reason     string
	NowUTC     time.Time
}

// AnnulPF is the ONLY SQLite write path for PF annulment (N→A).
func (d *DB) AnnulPF(ctx context.Context, p AnnulPFParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		return nil, fmt.Errorf("store: annul reason required")
	}
	if len([]rune(reason)) > 50 {
		reason = string([]rune(reason)[:50])
	}
	payloadHash := hashJSON(struct {
		DocumentID string `json:"document_id"`
		Reason     string `json:"reason"`
	}{DocumentID: p.DocumentID, Reason: reason})
	sum := sha256.Sum256([]byte(p.StoreID + "|ANNUL-PF|" + p.DocumentID))
	businessKey := "pf-annul:" + hex.EncodeToString(sum[:16])

	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if rec, hit, err := d.lookupIdempotency(tx, p.StoreID, p.RequestID, businessKey, payloadHash); err != nil {
		return nil, err
	} else if hit {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return rec, nil
	}

	var docType, docStatus, invoiceNo, atcud, hash, qr, createdAt string
	err = tx.QueryRow(`SELECT document_type, document_status, invoice_no, atcud, hash, qr_content, created_at
		FROM invoices WHERE id = ? AND store_id = ?`, p.DocumentID, p.StoreID).
		Scan(&docType, &docStatus, &invoiceNo, &atcud, &hash, &qr, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if docType != string(domain.DocumentPF) || docStatus != string(domain.WorkStatusNormal) {
		return nil, ErrProformaNotAllowed
	}

	var tz string
	_ = tx.QueryRow(`SELECT timezone FROM taxpayer_settings WHERE store_id = ?`, p.StoreID).Scan(&tz)
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("Europe/Lisbon", 0)
	}
	statusAt := p.NowUTC.In(loc).Format("2006-01-02T15:04:05")
	nowRFC := p.NowUTC.Format(time.RFC3339)

	_, err = tx.Exec(`UPDATE invoices SET document_status = 'A', status_reason = ?, status_changed_at = ?, source_id = ?
		WHERE id = ? AND document_status = 'N'`, reason, statusAt, p.OperatorID, p.DocumentID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`INSERT INTO idempotency_keys (
		id, store_id, request_id, request_payload_hash, business_key, invoice_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), p.StoreID, p.RequestID, payloadHash, businessKey, p.DocumentID, nowRFC)
	if err != nil {
		return nil, fmt.Errorf("store: idempotency: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	issuedAt := p.NowUTC
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		issuedAt = t
	}

	return &IssueRecord{
		DocumentID:     p.DocumentID,
		InvoiceNo:      invoiceNo,
		ATCUD:          atcud,
		DocumentType:   domain.DocumentPF,
		DocumentStatus: domain.WorkStatusAnnulled,
		IssuedAt:       issuedAt,
		Hash:           hash,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

// ProformaRef is a sale→PF or PF→sale link for detail/SAF-T readers.
type ProformaRef struct {
	SaleInvoiceID      string
	SaleInvoiceNo      string
	ProformaInvoiceID  string
	ProformaInvoiceNo  string
	ProformaInvoiceDate string
}

// ProformaRefForSale is the ONLY reader for a sale invoice's linked PF (at most one).
func (d *DB) ProformaRefForSale(saleInvoiceID string) (*ProformaRef, error) {
	var ref ProformaRef
	err := d.SQL.QueryRow(`SELECT r.sale_invoice_id, s.invoice_no, r.proforma_invoice_id, p.invoice_no, p.invoice_date
		FROM invoice_proforma_references r
		JOIN invoices s ON s.id = r.sale_invoice_id
		JOIN invoices p ON p.id = r.proforma_invoice_id
		WHERE r.sale_invoice_id = ?`, saleInvoiceID).
		Scan(&ref.SaleInvoiceID, &ref.SaleInvoiceNo, &ref.ProformaInvoiceID, &ref.ProformaInvoiceNo, &ref.ProformaInvoiceDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

// SalesForProforma lists sale invoices linked to a PF (reader only).
func (d *DB) SalesForProforma(proformaID string) ([]ProformaRef, error) {
	rows, err := d.SQL.Query(`SELECT r.sale_invoice_id, s.invoice_no, r.proforma_invoice_id, p.invoice_no, p.invoice_date
		FROM invoice_proforma_references r
		JOIN invoices s ON s.id = r.sale_invoice_id
		JOIN invoices p ON p.id = r.proforma_invoice_id
		WHERE r.proforma_invoice_id = ?
		ORDER BY s.created_at`, proformaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProformaRef
	for rows.Next() {
		var ref ProformaRef
		if err := rows.Scan(&ref.SaleInvoiceID, &ref.SaleInvoiceNo, &ref.ProformaInvoiceID, &ref.ProformaInvoiceNo, &ref.ProformaInvoiceDate); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// ProformaExtra holds PF-only detail fields.
type ProformaExtra struct {
	ValidUntil        string
	StatusReason      string
	StatusChangedAt   string
	LinkedSaleNumbers []string
}

// LoadProformaExtra is the ONLY reader for PF detail extras.
func (d *DB) LoadProformaExtra(proformaID string) (*ProformaExtra, error) {
	var validUntil, reason, changedAt sql.NullString
	err := d.SQL.QueryRow(`SELECT valid_until, status_reason, status_changed_at FROM invoices WHERE id = ? AND document_type = 'PF'`,
		proformaID).Scan(&validUntil, &reason, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	refs, err := d.SalesForProforma(proformaID)
	if err != nil {
		return nil, err
	}
	nos := make([]string, 0, len(refs))
	for _, r := range refs {
		nos = append(nos, r.SaleInvoiceNo)
	}
	return &ProformaExtra{
		ValidUntil:        validUntil.String,
		StatusReason:      reason.String,
		StatusChangedAt:   changedAt.String,
		LinkedSaleNumbers: nos,
	}, nil
}

// LoadInvoiceLinesAsCreditRemaining loads lines for PF (and similar) detail display.
func (d *DB) LoadInvoiceLinesAsCreditRemaining(invoiceID string) ([]CreditLineRemaining, error) {
	rows, err := d.SQL.Query(`SELECT id, line_number, product_description, line_gross
		FROM invoice_lines WHERE invoice_id = ? ORDER BY line_number`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CreditLineRemaining
	for rows.Next() {
		var ln CreditLineRemaining
		if err := rows.Scan(&ln.LineID, &ln.LineNumber, &ln.Description, &ln.LineGross); err != nil {
			return nil, err
		}
		ln.RemainingLineGross = ln.LineGross
		out = append(out, ln)
	}
	return out, rows.Err()
}
