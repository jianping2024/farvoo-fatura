package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ErrDebitNotAllowed indicates the original invoice cannot be debited.
var ErrDebitNotAllowed = errors.New("store: debit not allowed")

// ErrDebitAmountExceeded indicates debit line_gross is missing or not positive (not an original-gross cap).
var ErrDebitAmountExceeded = errors.New("store: debit amount invalid")

// ErrNDSeriesMissing indicates no ACTIVE ND series with validation_code.
var ErrNDSeriesMissing = errors.New("store: ND series missing")

// IssueNDParams is input for IssueND (validated by service.IssueDebitNote).
type IssueNDParams struct {
	StoreID           string
	RequestID         string
	OriginalInvoiceID string
	OperatorID        string
	StationID         string
	Reason            string
	DebitFull         bool
	Lines             []CreditLineInput
	NowUTC            time.Time
}

// IssueND is the ONLY SQLite write path for ND debit notes.
func (d *DB) IssueND(ctx context.Context, signer Signer, p IssueNDParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	payloadHash := debitPayloadHash(p)
	mode, fingerprint := debitModeFingerprint(p)
	businessKey := debitBusinessKey(p.StoreID, p.OriginalInvoiceID, mode, fingerprint)

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

	orig, err := loadOriginalInvoice(tx, p.StoreID, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}
	if err := validateDebitOriginal(orig); err != nil {
		return nil, err
	}

	origLines, err := loadOriginalLines(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}

	ndLines, ndGross, ndNet, ndTax, buckets, err := buildNDLines(origLines, p)
	if err != nil {
		return nil, err
	}
	if len(ndLines) == 0 {
		return nil, fmt.Errorf("store: no debit lines")
	}

	debitedTot, _ := compliance.ParseDecimal(orig.DebitedGross)
	newDebited := debitedTot.Add(ndGross)

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
		FROM series WHERE store_id = ? AND document_type = 'ND' AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash)
	if err != nil {
		return nil, ErrNDSeriesMissing
	}
	if validationCode == "" {
		return nil, ErrNDSeriesMissing
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo("ND", seriesCode, seq)
	atcud := compliance.FormatATCUD(validationCode, seq)
	grossStr := compliance.Money2(ndGross)
	netStr := compliance.Money2(ndNet)
	taxStr := compliance.Money2(ndTax)

	signPayload := compliance.BuildSignPayload(invoiceDate, systemEntry, invoiceNo, grossStr, lastHash)
	hashB64, hashControl, keyVersion, err := signer.Sign(signPayload)
	if err != nil {
		return nil, fmt.Errorf("store: sign: %w", err)
	}

	cust, err := loadOriginalCustomer(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}

	qr, err := compliance.BuildQR(compliance.QRInput{
		IssuerNIF:              nif,
		CustomerTaxID:          cust.TaxID,
		CustomerCountry:        cust.Country,
		DocumentType:           "ND",
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

	payMethod := "CASH"
	var payMethodScan sql.NullString
	_ = tx.QueryRow(`SELECT method FROM invoice_payments WHERE invoice_id = ? ORDER BY rowid LIMIT 1`, p.OriginalInvoiceID).
		Scan(&payMethodScan)
	if payMethodScan.Valid && payMethodScan.String != "" {
		payMethod = payMethodScan.String
	}

	docID := uuid.NewString()
	printJobID := uuid.NewString()
	idemID := uuid.NewString()
	nowRFC := p.NowUTC.Format(time.RFC3339)

	tableName := tableNameFromMeta(orig.DisplayMetaJSON)
	printPayload, payloadHashPrint, err := fiscalprint.BuildPayload(fiscalprint.BuildInput{
		DocumentID:                docID,
		DocumentType:              "ND",
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
		Lines:                     toPrintLines(ncLinesToPrepared(ndLines)),
		Buckets:                   buckets,
		NetTotal:                  netStr,
		TaxPayable:                taxStr,
		GrossTotal:                grossStr,
		Payments:                  []domain.PaymentInput{{Method: payMethod, Amount: grossStr}},
		ATCUD:                     atcud,
		QRContent:                 qr,
		Hash:                      hashB64,
		HashControl:               hashControl,
		OriginalInvoiceNo:         orig.InvoiceNo,
		CreditReason:              p.Reason,
	})
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`UPDATE series SET last_number = ?, last_hash = ?, updated_at = ? WHERE id = ?`,
		seq, hashB64, nowRFC, seriesID); err != nil {
		return nil, err
	}

	sourceSystem := orig.SourceSystem
	if sourceSystem == "" {
		sourceSystem = "LOCAL"
	}
	fiscalPurpose := "debit"

	_, err = tx.Exec(`INSERT INTO invoices (
		id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
		atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
		invoice_date, system_entry_date, document_status, print_status,
		gross_total, net_total, tax_payable, customer_id, source_id,
		software_certificate_number, source_system, source_sale_id, scope_type, scope_id,
		fiscal_purpose, external_bill_id, display_meta_json, debited_gross_total, created_at
	) VALUES (?, ?, 'ND', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SIGNED', 'PENDING',
		?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, NULL, ?, '0.00', ?)`,
		docID, p.StoreID, seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, p.OperatorID,
		cert, nullStr(sourceSystem), nullStr(orig.SourceSaleID),
		nullStr(orig.ScopeType), nullStr(orig.ScopeID),
		fiscalPurpose, nullStr(orig.DisplayMetaJSON), nowRFC)
	if err != nil {
		return nil, fmt.Errorf("store: insert ND invoice: %w", err)
	}

	lineNoToOrig := map[int]origLineRow{}
	for _, ol := range origLines {
		lineNoToOrig[ol.LineNumber] = ol
	}

	for _, ln := range ndLines {
		lineID := uuid.NewString()
		ol := lineNoToOrig[ln.OrigLineNumber]
		_, err = tx.Exec(`INSERT INTO invoice_lines (
			id, invoice_id, line_number, product_code, product_description, display_name,
			quantity, unit_of_measure, unit_price_gross, unit_price_net, line_gross, line_net, line_tax,
			vat_rate, tax_type, tax_country_region, tax_code, tax_exemption_code, tax_exemption_reason, product_type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'IVA', ?, ?, NULL, NULL, ?)`,
			lineID, docID, ln.LineNumber, ln.ProductCode, ln.ProductDescription, nullStr(ln.DisplayName),
			ln.Quantity, ln.UnitOfMeasure, ln.UnitPriceGross, ln.UnitPriceNet, ln.LineGross, ln.LineNet, ln.LineTax,
			ln.VATRate, taxRegion, ln.TaxCode, ln.ProductType)
		if err != nil {
			return nil, fmt.Errorf("store: insert ND line: %w", err)
		}
		_, err = tx.Exec(`INSERT INTO invoice_line_references (
			id, credit_line_id, original_invoice_id, original_invoice_no, original_line_id, original_line_number, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), lineID, p.OriginalInvoiceID, orig.InvoiceNo, ol.ID, ol.LineNumber, p.Reason)
		if err != nil {
			return nil, fmt.Errorf("store: insert line reference: %w", err)
		}
	}

	_, err = tx.Exec(`INSERT INTO invoice_customer_snapshots (
		invoice_id, customer_tax_id, company_name, address_detail, city, postal_code, country, account_id, self_billing_indicator
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0)`,
		docID, cust.TaxID, cust.CompanyName, cust.AddressDetail, cust.City, cust.PostalCode, cust.Country)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`INSERT INTO invoice_payments (id, invoice_id, method, amount, paid_at, operator_id)
		VALUES (?, ?, ?, ?, ?, ?)`, uuid.NewString(), docID, payMethod, grossStr, nowRFC, p.OperatorID)
	if err != nil {
		return nil, err
	}

	newDebitedStr := compliance.Money2(newDebited)
	// ND is a correction delta — never cap at original gross; always DEBITED_PARTIAL after any ND.
	_, err = tx.Exec(`UPDATE invoices SET debited_gross_total = ?, document_status = ? WHERE id = ?`,
		newDebitedStr, string(domain.DocumentDebitedPartial), p.OriginalInvoiceID)
	if err != nil {
		return nil, fmt.Errorf("store: update original: %w", err)
	}

	payloadJSON, err := json.Marshal(printPayload)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO local_print_jobs (
		id, invoice_id, document_type, print_purpose, job_status, logical_role,
		payload_json, payload_hash, attempts, last_error, created_at, updated_at, printed_at, created_by, station_id
	) VALUES (?, ?, 'ND', 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
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
		DocumentType:   domain.DocumentND,
		DocumentStatus: domain.DocumentSigned,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

func validateDebitOriginal(orig *origInvoiceRow) error {
	switch orig.DocType {
	case string(domain.DocumentFT), string(domain.DocumentFS), string(domain.DocumentFR):
	default:
		return ErrDebitNotAllowed
	}
	switch orig.DocStatus {
	case string(domain.DocumentSigned), string(domain.DocumentDebitedPartial), string(domain.DocumentDebitedFull):
		// DEBITED_FULL kept for legacy rows; further ND still allowed (no original-gross ceiling).
	default:
		return ErrDebitNotAllowed
	}
	return nil
}

// buildNDLines is the ONLY ND line builder (correction delta; no cap vs original line/gross).
func buildNDLines(origLines []origLineRow, p IssueNDParams) (
	[]ncPreparedLine, decimal.Decimal, decimal.Decimal, decimal.Decimal, []compliance.TaxBucket, error,
) {
	lineByNo := map[int]origLineRow{}
	for _, ol := range origLines {
		lineByNo[ol.LineNumber] = ol
	}

	var inputs []CreditLineInput
	if p.DebitFull {
		for _, ol := range origLines {
			inputs = append(inputs, CreditLineInput{
				OriginalLineNumber: ol.LineNumber,
				LineGross:          ol.LineGross,
			})
		}
	} else {
		inputs = p.Lines
	}

	bucketMap := map[string]*compliance.TaxBucket{}
	var lines []ncPreparedLine
	var grossTot, netTot, taxTot decimal.Decimal
	lineNo := 0

	for _, in := range inputs {
		ol, ok := lineByNo[in.OriginalLineNumber]
		if !ok {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, fmt.Errorf("store: unknown line %d", in.OriginalLineNumber)
		}
		grossStr := strings.TrimSpace(in.LineGross)
		qtyStr := strings.TrimSpace(in.Quantity)
		if grossStr == "" && qtyStr == "" {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, ErrDebitAmountExceeded
		}
		// No remaining ceiling: use a large sentinel only so creditLineFromInput can fill from qty.
		origGross, _ := compliance.ParseDecimal(ol.LineGross)
		sentinel := origGross
		if strings.TrimSpace(in.LineGross) != "" {
			g, err := compliance.ParseDecimal(in.LineGross)
			if err != nil {
				return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, err
			}
			if g.LessThanOrEqual(decimal.Zero) {
				return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, ErrDebitAmountExceeded
			}
			sentinel = g
		}
		ln, err := creditLineFromInput(ol, sentinel, in)
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, err
		}
		debitGross, _ := compliance.ParseDecimal(ln.LineGross)
		if debitGross.LessThanOrEqual(decimal.Zero) {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, ErrDebitAmountExceeded
		}

		lineNo++
		ln.LineNumber = lineNo
		lines = append(lines, ncPreparedLine{preparedLine: ln, OrigLineNumber: ol.LineNumber})
		g, _ := compliance.ParseDecimal(ln.LineGross)
		n, _ := compliance.ParseDecimal(ln.LineNet)
		t, _ := compliance.ParseDecimal(ln.LineTax)
		grossTot = grossTot.Add(g)
		netTot = netTot.Add(n)
		taxTot = taxTot.Add(t)
		rate := ln.VATRate
		if b, ok := bucketMap[rate]; ok {
			bb, _ := compliance.ParseDecimal(b.TaxBase)
			bt, _ := compliance.ParseDecimal(b.TaxAmount)
			b.TaxBase = compliance.Money2(bb.Add(n))
			b.TaxAmount = compliance.Money2(bt.Add(t))
		} else {
			bucketMap[rate] = &compliance.TaxBucket{Rate: rate, TaxBase: ln.LineNet, TaxAmount: ln.LineTax}
		}
	}

	var buckets []compliance.TaxBucket
	for _, b := range bucketMap {
		buckets = append(buckets, *b)
	}
	return lines, grossTot.Round(2), netTot.Round(2), taxTot.Round(2), buckets, nil
}

func debitPayloadHash(p IssueNDParams) string {
	type lineHash struct {
		OriginalLineNumber int    `json:"original_line_number"`
		Quantity           string `json:"quantity,omitempty"`
		LineGross          string `json:"line_gross,omitempty"`
	}
	body := struct {
		OriginalInvoiceID string     `json:"original_invoice_id"`
		Reason            string     `json:"reason"`
		DebitFull         bool       `json:"debit_full"`
		Lines             []lineHash `json:"lines,omitempty"`
	}{
		OriginalInvoiceID: p.OriginalInvoiceID,
		Reason:            p.Reason,
		DebitFull:           p.DebitFull,
	}
	if !p.DebitFull {
		for _, ln := range p.Lines {
			body.Lines = append(body.Lines, lineHash{
				OriginalLineNumber: ln.OriginalLineNumber,
				Quantity:           strings.TrimSpace(ln.Quantity),
				LineGross:          strings.TrimSpace(ln.LineGross),
			})
		}
	}
	return hashJSON(body)
}

func debitModeFingerprint(p IssueNDParams) (mode, fingerprint string) {
	if p.DebitFull {
		return "full", "full"
	}
	type lineFP struct {
		OriginalLineNumber int    `json:"original_line_number"`
		Quantity           string `json:"quantity,omitempty"`
		LineGross          string `json:"line_gross,omitempty"`
	}
	var fps []lineFP
	for _, ln := range p.Lines {
		fps = append(fps, lineFP{
			OriginalLineNumber: ln.OriginalLineNumber,
			Quantity:           strings.TrimSpace(ln.Quantity),
			LineGross:          strings.TrimSpace(ln.LineGross),
		})
	}
	sort.Slice(fps, func(i, j int) bool { return fps[i].OriginalLineNumber < fps[j].OriginalLineNumber })
	b, _ := json.Marshal(fps)
	sum := sha256.Sum256(b)
	return "partial", hex.EncodeToString(sum[:])
}

func debitBusinessKey(storeID, originalID, mode, fingerprint string) string {
	return fmt.Sprintf("%s|debit|%s|%s|%s", storeID, originalID, mode, fingerprint)
}

