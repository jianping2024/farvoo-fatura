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

// ErrGRSeriesMissing indicates no ACTIVE GR series with validation_code.
var ErrGRSeriesMissing = errors.New("store: GR series missing")

// ErrDeliveryNotAllowed indicates GR cannot be linked or annulled in this state.
var ErrDeliveryNotAllowed = errors.New("store: delivery note not allowed")

// IssueGRParams is input for IssueGR (validated by service.IssueDeliveryNote).
type IssueGRParams struct {
	StoreID             string
	RequestID           string
	Snapshot            domain.SaleSnapshot
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	InvoiceLocale       string
	ShipFromAddress     string
	ShipToAddress       string
	MovementStartAt     string
	RecipientIsVATPayer bool
	ATDocCodeID         string // required; from TransportClient
	NowUTC              time.Time
}

// IssueGR is the ONLY SQLite write path for GR (guia de remessa) documents.
// Must not be called via IssueFT. ATDocCodeID must already be obtained.
func (d *DB) IssueGR(ctx context.Context, signer Signer, p IssueGRParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	shipFrom := strings.TrimSpace(p.ShipFromAddress)
	shipTo := strings.TrimSpace(p.ShipToAddress)
	moveAt := strings.TrimSpace(p.MovementStartAt)
	atCode := strings.TrimSpace(p.ATDocCodeID)
	if shipFrom == "" || shipTo == "" || moveAt == "" {
		return nil, fmt.Errorf("store: ship_from, ship_to, and movement_start_at required")
	}
	if atCode == "" {
		return nil, fmt.Errorf("store: at_transport_doc_code required")
	}
	cust, err := normalizeGRCustomer(p.Snapshot.Customer, p.RecipientIsVATPayer)
	if err != nil {
		return nil, err
	}
	payloadHash := grPayloadHash(p)
	businessKey := grBusinessKey(p.StoreID, p.Snapshot)

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
		FROM series WHERE store_id = ? AND document_type = 'GR' AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash)
	if err != nil {
		return nil, ErrGRSeriesMissing
	}
	if validationCode == "" {
		return nil, ErrGRSeriesMissing
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo("GR", seriesCode, seq)
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

	custID, err := d.ensureCustomerIDTx(tx, cust)
	if err != nil {
		return nil, err
	}
	qrTaxID := cust.TaxID
	if qrTaxID == "" {
		qrTaxID = "999999990"
	}
	qr, err := compliance.BuildQR(compliance.QRInput{
		IssuerNIF:              nif,
		CustomerTaxID:          qrTaxID,
		CustomerCountry:        cust.Country,
		DocumentType:           "GR",
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
		DocumentType:              "GR",
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
		ShipFromAddress:           shipFrom,
		ShipToAddress:             shipTo,
		MovementStartAt:           moveAt,
		ATDocCodeID:               atCode,
		RecipientIsVATPayer:       p.RecipientIsVATPayer,
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
	vatFlag := 0
	if p.RecipientIsVATPayer {
		vatFlag = 1
	}

	_, err = tx.Exec(`INSERT INTO invoices (
		id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
		atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
		invoice_date, system_entry_date, document_status, print_status,
		gross_total, net_total, tax_payable, customer_id, source_id,
		software_certificate_number, source_system, source_sale_id, scope_type, scope_id,
		fiscal_purpose, external_bill_id, display_meta_json, credited_gross_total, received_gross_total, created_at,
		fiscal_terminal_id, fiscal_terminal_label, status_reason, status_changed_at, valid_until,
		ship_from_address, ship_to_address, movement_start_at, at_transport_doc_code, recipient_is_vat_payer
	) VALUES (?, ?, 'GR', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N', 'PENDING',
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, '0.00', '0.00', ?, ?, ?, NULL, ?, NULL,
		?, ?, ?, ?, ?)`,
		docID, p.StoreID, seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, custID, p.OperatorID,
		cert, nullStr(sourceSystem), nullStr(p.Snapshot.SourceSaleID),
		nullStr(p.Snapshot.ScopeType), nullStr(p.Snapshot.ScopeID),
		"delivery_note", displayMetaJSON(p.Snapshot.DisplayMeta), nowRFC, termID, termLabel,
		systemEntry,
		shipFrom, shipTo, moveAt, atCode, vatFlag)
	if err != nil {
		return nil, fmt.Errorf("store: insert GR invoice: %w", err)
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
			return nil, fmt.Errorf("store: insert GR line: %w", err)
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
	) VALUES (?, ?, 'GR', 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
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
		DocumentType:   domain.DocumentGR,
		DocumentStatus: domain.WorkStatusNormal,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

func normalizeGRCustomer(c domain.CustomerInput, vatPayer bool) (domain.CustomerInput, error) {
	c.TaxID = strings.TrimSpace(c.TaxID)
	c.CompanyName = strings.TrimSpace(c.CompanyName)
	c.AddressDetail = strings.TrimSpace(c.AddressDetail)
	c.City = strings.TrimSpace(c.City)
	c.PostalCode = strings.TrimSpace(c.PostalCode)
	if c.Country == "" {
		c.Country = "PT"
	}
	if c.CompanyName == "" {
		return c, fmt.Errorf("store: GR customer name required")
	}
	if c.AddressDetail == "" || c.City == "" || c.PostalCode == "" {
		return c, fmt.Errorf("store: GR customer address (detail/city/postal) required")
	}
	if vatPayer {
		if c.TaxID == "" || c.TaxID == "999999990" {
			return c, fmt.Errorf("store: GR VAT payer requires real NIF")
		}
	} else {
		// Non-VAT recipient: keep empty TaxID on snapshot (no silent 999999990).
		// ensureCustomerIDTx maps empty → cust-final for FK only; QR uses sentinel separately.
		c.TaxID = ""
	}
	return c, nil
}

func grPayloadHash(p IssueGRParams) string {
	body := struct {
		Snapshot            domain.SaleSnapshot `json:"snapshot"`
		ShipFrom            string              `json:"ship_from"`
		ShipTo              string              `json:"ship_to"`
		MovementStartAt     string              `json:"movement_start_at"`
		RecipientIsVATPayer bool                `json:"recipient_is_vat_payer"`
		ATDocCodeID         string              `json:"at_doc_code_id"`
	}{p.Snapshot, p.ShipFromAddress, p.ShipToAddress, p.MovementStartAt, p.RecipientIsVATPayer, p.ATDocCodeID}
	return hashJSON(body)
}

func grBusinessKey(storeID string, snap domain.SaleSnapshot) string {
	sum := sha256.Sum256([]byte(storeID + "|GR|" + snap.SourceSystem + "|" + snap.SourceSaleID + "|" + snap.ScopeType + "|" + snap.ScopeID))
	return "gr:" + hex.EncodeToString(sum[:16])
}

// AnnulGRParams is input for AnnulGR.
type AnnulGRParams struct {
	StoreID    string
	RequestID  string
	DocumentID string
	OperatorID string
	Reason     string
	NowUTC     time.Time
}

// AnnulGR is the ONLY SQLite write path for GR annulment (N→A).
func (d *DB) AnnulGR(ctx context.Context, p AnnulGRParams) (*IssueRecord, error) {
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
	sum := sha256.Sum256([]byte(p.StoreID + "|ANNUL-GR|" + p.DocumentID))
	businessKey := "gr-annul:" + hex.EncodeToString(sum[:16])

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
	if docType != string(domain.DocumentGR) || docStatus != string(domain.WorkStatusNormal) {
		return nil, ErrDeliveryNotAllowed
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
		DocumentType:   domain.DocumentGR,
		DocumentStatus: domain.WorkStatusAnnulled,
		IssuedAt:       issuedAt,
		Hash:           hash,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

// linkDeliveryNotesInTx is the ONLY writer for invoice_delivery_references (inside IssueFT tx, FT only).
func linkDeliveryNotesInTx(tx *sql.Tx, storeID, saleInvoiceID string, deliveryIDs []string, statusChangedAt, operatorID string) error {
	_ = operatorID
	seen := map[string]struct{}{}
	for _, raw := range deliveryIDs {
		grID := strings.TrimSpace(raw)
		if grID == "" {
			continue
		}
		if _, ok := seen[grID]; ok {
			continue
		}
		seen[grID] = struct{}{}

		var docType, docStatus string
		err := tx.QueryRow(`SELECT document_type, document_status FROM invoices WHERE id = ? AND store_id = ?`,
			grID, storeID).Scan(&docType, &docStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if docType != string(domain.DocumentGR) {
			return ErrDeliveryNotAllowed
		}
		if !domain.IsLinkableDeliveryNoteStatus(docStatus) {
			return ErrDeliveryNotAllowed
		}
		_, err = tx.Exec(`INSERT INTO invoice_delivery_references (id, sale_invoice_id, delivery_invoice_id, created_at)
			VALUES (?, ?, ?, ?)`, uuid.NewString(), saleInvoiceID, grID, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return ErrDeliveryNotAllowed
			}
			return err
		}
		if docStatus == string(domain.WorkStatusNormal) {
			_, err = tx.Exec(`UPDATE invoices SET document_status = 'F', status_changed_at = ? WHERE id = ? AND document_status = 'N'`,
				statusChangedAt, grID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeliveryExtra is GR detail for Admin / API.
type DeliveryExtra struct {
	ShipFromAddress     string
	ShipToAddress       string
	MovementStartAt     string
	ATDocCodeID         string
	RecipientIsVATPayer bool
	StatusReason        string
	StatusChangedAt     string
	LinkedSaleNumbers   []string
}

// LoadDeliveryExtra loads GR movement fields and linked FT numbers.
func (d *DB) LoadDeliveryExtra(deliveryID string) (*DeliveryExtra, error) {
	var shipFrom, shipTo, moveAt, atCode, reason, changedAt sql.NullString
	var vatFlag sql.NullInt64
	err := d.SQL.QueryRow(`SELECT ship_from_address, ship_to_address, movement_start_at, at_transport_doc_code,
		recipient_is_vat_payer, status_reason, status_changed_at
		FROM invoices WHERE id = ? AND document_type = 'GR'`, deliveryID).
		Scan(&shipFrom, &shipTo, &moveAt, &atCode, &vatFlag, &reason, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := d.SQL.Query(`SELECT s.invoice_no FROM invoice_delivery_references r
		JOIN invoices s ON s.id = r.sale_invoice_id
		WHERE r.delivery_invoice_id = ?
		ORDER BY r.created_at`, deliveryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nos []string
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err != nil {
			return nil, err
		}
		nos = append(nos, no)
	}
	return &DeliveryExtra{
		ShipFromAddress:     shipFrom.String,
		ShipToAddress:       shipTo.String,
		MovementStartAt:     moveAt.String,
		ATDocCodeID:         atCode.String,
		RecipientIsVATPayer: vatFlag.Valid && vatFlag.Int64 != 0,
		StatusReason:        reason.String,
		StatusChangedAt:     changedAt.String,
		LinkedSaleNumbers:   nos,
	}, nil
}

// DeliveryRefsForSale returns GR invoice numbers linked to an FT.
func (d *DB) DeliveryRefsForSale(saleInvoiceID string) ([]string, error) {
	rows, err := d.SQL.Query(`SELECT g.invoice_no FROM invoice_delivery_references r
		JOIN invoices g ON g.id = r.delivery_invoice_id
		WHERE r.sale_invoice_id = ?
		ORDER BY r.created_at`, saleInvoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nos []string
	for rows.Next() {
		var no string
		if err := rows.Scan(&no); err != nil {
			return nil, err
		}
		nos = append(nos, no)
	}
	return nos, rows.Err()
}
