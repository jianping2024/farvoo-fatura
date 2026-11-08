package store

import (
	"database/sql"
	"errors"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// ErrTransportSeqMoved: the GR/GT number communicated to AT is no longer the next one
// (another document took it). Fail closed — the AT registration does not match a local doc.
var ErrTransportSeqMoved = errors.New("store: transport series number moved since AT communication")

// TransportPrep is everything AT envioDocumentoTransporte needs from local state, read
// before communication: the real next document number plus issuer + line data.
type TransportPrep struct {
	Seq        int64
	InvoiceNo  string // e.g. "GR GR2026T02/1" — the number AT receives and the doc keeps
	ATCUD      string // validation_code-seq, sent with the number (AT validates both)
	IssuerNIF  string
	IssuerName string
	IssuerAddr string
	IssuerCity string
	IssuerPost string
	Lines      []TransportPrepLine
}

// TransportPrepLine is one AT Line (UnitPrice is net of VAT, as in SAF-T).
type TransportPrepLine struct {
	Description   string
	Quantity      string
	UnitOfMeasure string
	UnitPriceNet  string
}

// PrepareTransport is the ONLY pre-AT read for GR/GT: next number from the ACTIVE series
// of docType (GR|GT) and issuer/line data. Callers must serialize PrepareTransport →
// AT → IssueGR/IssueGT (ExpectedSeq) so the number cannot move in between.
func (d *DB) PrepareTransport(storeID, docType string, snap domain.SaleSnapshot) (*TransportPrep, error) {
	missing := ErrGRSeriesMissing
	if docType == "GT" {
		missing = ErrGTSeriesMissing
	} else if docType != "GR" {
		return nil, fmt.Errorf("store: transport doc type %q", docType)
	}
	var p TransportPrep
	var taxRegion string
	err := d.SQL.QueryRow(`SELECT tax_registration_number, legal_name, address_detail, city, postal_code, tax_country_region
		FROM taxpayer_settings WHERE store_id = ?`, storeID).
		Scan(&p.IssuerNIF, &p.IssuerName, &p.IssuerAddr, &p.IssuerCity, &p.IssuerPost, &taxRegion)
	if err != nil {
		return nil, fmt.Errorf("store: taxpayer_settings: %w", err)
	}
	var seriesCode, validationCode string
	var lastNumber int64
	err = d.SQL.QueryRow(`SELECT series_code, validation_code, last_number
		FROM series WHERE store_id = ? AND document_type = ? AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, storeID, docType).
		Scan(&seriesCode, &validationCode, &lastNumber)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && validationCode == "") {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	p.Seq = lastNumber + 1
	p.InvoiceNo = compliance.FormatInvoiceNo(docType, seriesCode, p.Seq)
	p.ATCUD = compliance.FormatATCUD(validationCode, p.Seq)

	lines, _, _, _, _, err := buildLines(snap.Lines, taxRegion)
	if err != nil {
		return nil, err
	}
	for _, ln := range lines {
		p.Lines = append(p.Lines, TransportPrepLine{
			Description: ln.ProductDescription, Quantity: ln.Quantity,
			UnitOfMeasure: ln.UnitOfMeasure, UnitPriceNet: ln.UnitPriceNet,
		})
	}
	return &p, nil
}

// IssuedByRequestID returns the document already issued for store+request_id (retry
// before AT: never communicate twice for one request).
func (d *DB) IssuedByRequestID(storeID, requestID string) (*IssueRecord, bool, error) {
	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var invID string
	err = tx.QueryRow(`SELECT invoice_id FROM idempotency_keys WHERE store_id = ? AND request_id = ?`,
		storeID, requestID).Scan(&invID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rec, err := loadIssueRecord(tx, invID)
	if err != nil {
		return nil, false, err
	}
	rec.IdempotentHit = true
	return rec, true, nil
}

// CommunicatedMovement is an issued GR/GT rebuilt for a second envioDocumentoTransporte
// (annulment, MovementStatus=A). Issuer name and address are the current store profile;
// the document number, ATCUD, parties, points, lines and start time come from the row.
type CommunicatedMovement struct {
	DocumentID     string
	DocumentType   string
	DocumentStatus string
	InvoiceNo      string
	ATCUD          string
	MovementStart  string
	Customer       domain.CustomerInput
	ShipFrom       domain.Address
	ShipTo         domain.Address
	IssuerNIF      string
	IssuerName     string
	IssuerAddr     string
	IssuerCity     string
	IssuerPost     string
	Lines          []TransportPrepLine
}

// LoadCommunicatedMovement is the ONLY pre-AT read of an already issued GR/GT.
// It does not allocate a series number.
func (d *DB) LoadCommunicatedMovement(storeID, documentID string) (*CommunicatedMovement, error) {
	var m CommunicatedMovement
	var from, to shipAddressCols
	var moveAt sql.NullString
	dest := []any{&m.DocumentType, &m.DocumentStatus, &m.InvoiceNo, &m.ATCUD, &moveAt}
	dest = append(dest, from.dest()...)
	dest = append(dest, to.dest()...)
	dest = append(dest,
		&m.Customer.TaxID, &m.Customer.CompanyName, &m.Customer.AddressDetail,
		&m.Customer.City, &m.Customer.PostalCode, &m.Customer.Country,
		&m.IssuerNIF, &m.IssuerName, &m.IssuerAddr, &m.IssuerCity, &m.IssuerPost,
	)
	err := d.SQL.QueryRow(`SELECT i.document_type, i.document_status, i.invoice_no, i.atcud, i.movement_start_at, `+
		shipAddressSelect("i.ship_from")+`, `+shipAddressSelect("i.ship_to")+`,
		COALESCE(cs.customer_tax_id,''), COALESCE(cs.company_name,''), COALESCE(cs.address_detail,''),
		COALESCE(cs.city,''), COALESCE(cs.postal_code,''), COALESCE(cs.country,''),
		COALESCE(t.tax_registration_number,''), COALESCE(t.legal_name,''), COALESCE(t.address_detail,''),
		COALESCE(t.city,''), COALESCE(t.postal_code,'')
		FROM invoices i
		LEFT JOIN invoice_customer_snapshots cs ON cs.invoice_id = i.id
		LEFT JOIN taxpayer_settings t ON t.store_id = i.store_id
		WHERE i.id = ? AND i.store_id = ?`, documentID, storeID).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.DocumentID = documentID
	m.MovementStart = moveAt.String
	m.ShipFrom = from.address()
	m.ShipTo = to.address()
	rows, err := d.SQL.Query(`SELECT product_description, quantity, unit_of_measure, unit_price_net
		FROM invoice_lines WHERE invoice_id = ? ORDER BY line_number`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ln TransportPrepLine
		if err := rows.Scan(&ln.Description, &ln.Quantity, &ln.UnitOfMeasure, &ln.UnitPriceNet); err != nil {
			return nil, err
		}
		m.Lines = append(m.Lines, ln)
	}
	return &m, rows.Err()
}
