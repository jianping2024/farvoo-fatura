package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ErrConflict indicates idempotency payload mismatch.
var ErrConflict = errors.New("store: idempotency conflict")

// ErrNotFound is returned when a lookup misses.
var ErrNotFound = errors.New("store: not found")

// Signer signs the Hash payload (injected; PEMSigner in P0).
type Signer interface {
	Sign(payload string) (hashBase64 string, hashControl int, keyVersion int, err error)
}

// IssueParams is input already validated by service.
type IssueParams struct {
	StoreID    string
	RequestID  string
	DocType    domain.DocumentType
	Snapshot   domain.SaleSnapshot
	OperatorID string
	StationID  string // Agent station_printers key for ORIGINAL print job
	NowUTC     time.Time // injectable for tests
}

// IssueRecord is the committed fiscal document + ORIGINAL print job.
type IssueRecord struct {
	DocumentID     string
	InvoiceNo      string
	ATCUD          string
	DocumentType   domain.DocumentType
	DocumentStatus domain.DocumentStatus
	PrintJobID     string
	PrintStatus    domain.PrintStatus
	IssuedAt       time.Time
	Hash           string
	QRContent      string
	IdempotentHit  bool
}

// IssueFT is the ONLY SQLite write path for signed sale documents (FT / FS / FR).
// NC and ND use IssueNC / IssueND.
func (d *DB) IssueFT(ctx context.Context, signer Signer, p IssueParams) (*IssueRecord, error) {
	_ = ctx
	switch p.DocType {
	case domain.DocumentFT, domain.DocumentFS, domain.DocumentFR:
	default:
		return nil, fmt.Errorf("store: unsupported sale document type %s (use IssueNC/IssueND)", p.DocType)
	}
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	payloadHash := hashJSON(p.Snapshot)
	businessKey := businessIdempotencyKey(p.StoreID, p.Snapshot)

	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// BEGIN IMMEDIATE equivalent: first write locks
	if _, err := tx.Exec(`UPDATE schema_migrations SET version = version WHERE 0`); err != nil {
		// ignore — ensure write lock via series update below
		_ = err
	}

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
	var status string
	err = tx.QueryRow(`SELECT id, series_code, validation_code, last_number, last_hash, status
		FROM series WHERE store_id = ? AND document_type = ? AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID, string(p.DocType)).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash, &status)
	if err != nil {
		return nil, fmt.Errorf("store: active %s series: %w", p.DocType, err)
	}
	if validationCode == "" {
		return nil, fmt.Errorf("store: series %s missing validation_code", seriesCode)
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo(string(p.DocType), seriesCode, seq)
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
		DocumentType:           string(p.DocType),
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
		DocumentType:              string(p.DocType),
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
		Payments:                  p.Snapshot.Payments,
		ATCUD:                     atcud,
		QRContent:                 qr,
		Hash:                      hashB64,
		HashControl:               hashControl,
	})
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`UPDATE series SET last_number = ?, last_hash = ?, updated_at = ? WHERE id = ?`,
		seq, hashB64, nowRFC, seriesID); err != nil {
		return nil, err
	}

	opID := p.OperatorID
	_, err = tx.Exec(`INSERT INTO invoices (
		id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
		atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
		invoice_date, system_entry_date, document_status, print_status,
		gross_total, net_total, tax_payable, customer_id, source_id,
		software_certificate_number, source_system, source_sale_id, scope_type, scope_id,
		fiscal_purpose, external_bill_id, display_meta_json, credited_gross_total, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SIGNED', 'PENDING',
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '0.00', ?)`,
		docID, p.StoreID, string(p.DocType), seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, custID, opID,
		cert, nullStr(p.Snapshot.SourceSystem), nullStr(p.Snapshot.SourceSaleID),
		nullStr(p.Snapshot.ScopeType), nullStr(p.Snapshot.ScopeID),
		nullStr(p.Snapshot.FiscalPurpose), nullStr(p.Snapshot.ExternalBillID),
		displayMetaJSON(p.Snapshot.DisplayMeta), nowRFC)
	if err != nil {
		return nil, fmt.Errorf("store: insert invoice: %w", err)
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
			return nil, fmt.Errorf("store: insert line: %w", err)
		}
	}

	_, err = tx.Exec(`INSERT INTO invoice_customer_snapshots (
		invoice_id, customer_tax_id, company_name, address_detail, city, postal_code, country, account_id, self_billing_indicator
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0)`,
		docID, cust.TaxID, cust.CompanyName, cust.AddressDetail, cust.City, cust.PostalCode, cust.Country)
	if err != nil {
		return nil, err
	}

	payments := p.Snapshot.Payments
	if len(payments) == 0 {
		payments = []domain.PaymentInput{{Method: "CASH", Amount: grossStr}}
	}
	for _, pay := range payments {
		_, err = tx.Exec(`INSERT INTO invoice_payments (id, invoice_id, method, amount, paid_at, operator_id)
			VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), docID, pay.Method, pay.Amount, nowRFC, opID)
		if err != nil {
			return nil, err
		}
	}

	payloadJSON, err := json.Marshal(printPayload)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO local_print_jobs (
		id, invoice_id, document_type, print_purpose, job_status, logical_role,
		payload_json, payload_hash, attempts, last_error, created_at, updated_at, printed_at, created_by, station_id
	) VALUES (?, ?, ?, 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
		printJobID, docID, string(p.DocType), string(payloadJSON), payloadHashPrint, nowRFC, nowRFC, opID, nullStr(p.StationID))
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
		DocumentType:   p.DocType,
		DocumentStatus: domain.DocumentSigned,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

type preparedLine struct {
	LineNumber         int
	ProductCode        string
	ProductDescription string
	DisplayName        string
	Quantity           string
	UnitOfMeasure      string
	UnitPriceGross     string
	UnitPriceNet       string
	LineGross          string
	LineNet            string
	LineTax            string
	VATRate            string
	TaxCode            string
	ProductType        string
}

func toPrintLines(lines []preparedLine) []fiscalprint.LineAmounts {
	out := make([]fiscalprint.LineAmounts, 0, len(lines))
	for _, ln := range lines {
		out = append(out, fiscalprint.LineAmounts{
			LineNumber: ln.LineNumber, ProductCode: ln.ProductCode, ProductDescription: ln.ProductDescription,
			DisplayName: ln.DisplayName, Quantity: ln.Quantity, UnitOfMeasure: ln.UnitOfMeasure,
			UnitPriceGross: ln.UnitPriceGross, UnitPriceNet: ln.UnitPriceNet,
			LineGross: ln.LineGross, LineNet: ln.LineNet, LineTax: ln.LineTax,
			VATRate: ln.VATRate, TaxCode: ln.TaxCode, ProductType: ln.ProductType,
		})
	}
	return out
}

func buildLines(in []domain.SaleLine, _ string) ([]preparedLine, []compliance.TaxBucket, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	if len(in) == 0 {
		return nil, nil, decimal.Zero, decimal.Zero, decimal.Zero, fmt.Errorf("store: no lines")
	}
	bucketMap := map[string]*compliance.TaxBucket{}
	var lines []preparedLine
	var grossTot, netTot, taxTot decimal.Decimal
	for i, sl := range in {
		lg, ln, lt, un, err := compliance.LineFromGross(sl.Quantity, sl.UnitPriceGross, sl.VATRate)
		if err != nil {
			return nil, nil, decimal.Zero, decimal.Zero, decimal.Zero, err
		}
		saftName := sl.SaftName
		if saftName == "" {
			saftName = sl.DisplayName
		}
		uom := sl.UnitOfMeasure
		if uom == "" {
			uom = "UN"
		}
		pt := sl.ProductType
		if pt == "" {
			pt = "P"
		}
		code := sl.ProductCode
		if code == "" {
			code = fmt.Sprintf("LINE%d", i+1)
		}
		rate := sl.VATRate
		lines = append(lines, preparedLine{
			LineNumber: i + 1, ProductCode: code, ProductDescription: saftName, DisplayName: sl.DisplayName,
			Quantity: sl.Quantity, UnitOfMeasure: uom, UnitPriceGross: sl.UnitPriceGross, UnitPriceNet: un,
			LineGross: lg, LineNet: ln, LineTax: lt, VATRate: rate, TaxCode: compliance.TaxCodeForRate(rate), ProductType: pt,
		})
		g, _ := compliance.ParseDecimal(lg)
		n, _ := compliance.ParseDecimal(ln)
		t, _ := compliance.ParseDecimal(lt)
		grossTot = grossTot.Add(g)
		netTot = netTot.Add(n)
		taxTot = taxTot.Add(t)
		if b, ok := bucketMap[rate]; ok {
			bb, _ := compliance.ParseDecimal(b.TaxBase)
			bt, _ := compliance.ParseDecimal(b.TaxAmount)
			b.TaxBase = compliance.Money2(bb.Add(n))
			b.TaxAmount = compliance.Money2(bt.Add(t))
		} else {
			bucketMap[rate] = &compliance.TaxBucket{Rate: rate, TaxBase: ln, TaxAmount: lt}
		}
	}
	var buckets []compliance.TaxBucket
	for _, b := range bucketMap {
		buckets = append(buckets, *b)
	}
	return lines, buckets, grossTot.Round(2), netTot.Round(2), taxTot.Round(2), nil
}

func normalizeCustomer(c domain.CustomerInput) domain.CustomerInput {
	if c.TaxID == "" {
		c.TaxID = "999999990"
	}
	if c.CompanyName == "" {
		c.CompanyName = "Consumidor Final"
	}
	if c.Country == "" {
		c.Country = "PT"
	}
	if c.AddressDetail == "" {
		c.AddressDetail = "Desconhecido"
	}
	if c.City == "" {
		c.City = "Desconhecido"
	}
	if c.PostalCode == "" {
		c.PostalCode = "Desconhecido"
	}
	return c
}

func businessIdempotencyKey(storeID string, s domain.SaleSnapshot) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", storeID, s.SourceSystem, s.SourceSaleID, s.ScopeType, s.ScopeID, s.FiscalPurpose)
}

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func displayMetaJSON(m map[string]string) any {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func (d *DB) lookupIdempotency(tx *sql.Tx, storeID, requestID, businessKey, payloadHash string) (*IssueRecord, bool, error) {
	var invID, reqHash, biz string
	err := tx.QueryRow(`SELECT invoice_id, request_payload_hash, business_key FROM idempotency_keys
		WHERE store_id = ? AND request_id = ?`, storeID, requestID).Scan(&invID, &reqHash, &biz)
	if err == nil {
		if reqHash != payloadHash {
			return nil, false, ErrConflict
		}
		rec, err := loadIssueRecord(tx, invID)
		if err != nil {
			return nil, false, err
		}
		rec.IdempotentHit = true
		return rec, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	err = tx.QueryRow(`SELECT invoice_id FROM idempotency_keys WHERE store_id = ? AND business_key = ?`,
		storeID, businessKey).Scan(&invID)
	if err == nil {
		rec, err := loadIssueRecord(tx, invID)
		if err != nil {
			return nil, false, err
		}
		rec.IdempotentHit = true
		return rec, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	return nil, false, nil
}

func loadIssueRecord(tx *sql.Tx, invoiceID string) (*IssueRecord, error) {
	var rec IssueRecord
	var docType, docStatus, printStatus string
	var created string
	err := tx.QueryRow(`SELECT id, invoice_no, atcud, document_type, document_status, print_status, hash, qr_content, created_at
		FROM invoices WHERE id = ?`, invoiceID).
		Scan(&rec.DocumentID, &rec.InvoiceNo, &rec.ATCUD, &docType, &docStatus, &printStatus, &rec.Hash, &rec.QRContent, &created)
	if err != nil {
		return nil, err
	}
	rec.DocumentType = domain.DocumentType(docType)
	rec.DocumentStatus = domain.DocumentStatus(docStatus)
	rec.PrintStatus = domain.PrintStatus(printStatus)
	rec.IssuedAt, _ = time.Parse(time.RFC3339, created)
	err = tx.QueryRow(`SELECT id FROM local_print_jobs WHERE invoice_id = ? AND print_purpose = 'ORIGINAL' ORDER BY created_at LIMIT 1`,
		invoiceID).Scan(&rec.PrintJobID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return &rec, nil
}
