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

// ErrCreditNotAllowed indicates the original invoice cannot be credited.
var ErrCreditNotAllowed = errors.New("store: credit not allowed")

// ErrCreditAmountExceeded indicates cumulative credit would exceed original gross.
var ErrCreditAmountExceeded = errors.New("store: credit amount exceeded")

// ErrNCSeriesMissing indicates no ACTIVE NC series with validation_code.
var ErrNCSeriesMissing = errors.New("store: NC series missing")

// CreditLineInput is one partial credit line (validated by service).
type CreditLineInput struct {
	OriginalLineNumber int
	Quantity           string
	LineGross          string
}

// IssueNCParams is input for IssueNC (validated by service.IssueCreditNote).
type IssueNCParams struct {
	StoreID           string
	RequestID         string
	OriginalInvoiceID string
	OperatorID        string
	StationID         string
	Reason            string
	CreditFull        bool
	Lines             []CreditLineInput
	NowUTC            time.Time
}

type origInvoiceRow struct {
	ID, InvoiceNo, DocType, DocStatus, GrossTotal, CreditedGross string
	SourceSystem, SourceSaleID, ScopeType, ScopeID, FiscalPurpose  string
	DisplayMetaJSON                                              string
}

type origLineRow struct {
	ID, ProductCode, ProductDescription, DisplayName string
	LineNumber                                        int
	Quantity, UnitOfMeasure, UnitPriceGross, UnitPriceNet string
	LineGross, LineNet, LineTax, VATRate, TaxCode, ProductType string
}

type origCustomerRow struct {
	TaxID, CompanyName, AddressDetail, City, PostalCode, Country string
}

// IssueNC is the ONLY SQLite write path for NC credit notes.
func (d *DB) IssueNC(ctx context.Context, signer Signer, p IssueNCParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	payloadHash := creditPayloadHash(p)
	mode, fingerprint := creditModeFingerprint(p)
	businessKey := creditBusinessKey(p.StoreID, p.OriginalInvoiceID, mode, fingerprint)

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
	if err := validateCreditOriginal(orig); err != nil {
		return nil, err
	}

	origLines, err := loadOriginalLines(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}
	creditedByLine, err := loadCreditedGrossByLine(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}

	ncLines, ncGross, ncNet, ncTax, buckets, err := buildNCLines(origLines, creditedByLine, p)
	if err != nil {
		return nil, err
	}
	if len(ncLines) == 0 {
		return nil, fmt.Errorf("store: no credit lines")
	}

	origGross, _ := compliance.ParseDecimal(orig.GrossTotal)
	creditedTot, _ := compliance.ParseDecimal(orig.CreditedGross)
	newCredited := creditedTot.Add(ncGross)
	if newCredited.GreaterThan(origGross) {
		return nil, ErrCreditAmountExceeded
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
		FROM series WHERE store_id = ? AND document_type = 'NC' AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash)
	if err != nil {
		return nil, ErrNCSeriesMissing
	}
	if validationCode == "" {
		return nil, ErrNCSeriesMissing
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo("NC", seriesCode, seq)
	atcud := compliance.FormatATCUD(validationCode, seq)
	grossStr := compliance.Money2(ncGross)
	netStr := compliance.Money2(ncNet)
	taxStr := compliance.Money2(ncTax)

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
		DocumentType:           "NC",
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
		DocumentType:              "NC",
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
		Lines:                     toPrintLines(ncLinesToPrepared(ncLines)),
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
	fiscalPurpose := "credit"

	_, err = tx.Exec(`INSERT INTO invoices (
		id, store_id, document_type, series_id, series_code, sequence_number, invoice_no,
		atcud, hash, hash_control, signing_key_version, previous_hash, qr_content,
		invoice_date, system_entry_date, document_status, print_status,
		gross_total, net_total, tax_payable, customer_id, source_id,
		software_certificate_number, source_system, source_sale_id, scope_type, scope_id,
		fiscal_purpose, external_bill_id, display_meta_json, credited_gross_total, created_at
	) VALUES (?, ?, 'NC', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SIGNED', 'PENDING',
		?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, NULL, ?, '0.00', ?)`,
		docID, p.StoreID, seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, p.OperatorID,
		cert, nullStr(sourceSystem), nullStr(orig.SourceSaleID),
		nullStr(orig.ScopeType), nullStr(orig.ScopeID),
		fiscalPurpose, nullStr(orig.DisplayMetaJSON), nowRFC)
	if err != nil {
		return nil, fmt.Errorf("store: insert NC invoice: %w", err)
	}

	lineNoToOrig := map[int]origLineRow{}
	for _, ol := range origLines {
		lineNoToOrig[ol.LineNumber] = ol
	}

	for _, ln := range ncLines {
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
			return nil, fmt.Errorf("store: insert NC line: %w", err)
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

	newCreditedStr := compliance.Money2(newCredited)
	newStatus := string(domain.DocumentCreditedPartial)
	if newCredited.Equal(origGross) {
		newStatus = string(domain.DocumentCreditedFull)
	}
	_, err = tx.Exec(`UPDATE invoices SET credited_gross_total = ?, document_status = ? WHERE id = ?`,
		newCreditedStr, newStatus, p.OriginalInvoiceID)
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
	) VALUES (?, ?, 'NC', 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
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
		DocumentType:   domain.DocumentNC,
		DocumentStatus: domain.DocumentSigned,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

type ncPreparedLine struct {
	preparedLine
	OrigLineNumber int
}

func ncLinesToPrepared(in []ncPreparedLine) []preparedLine {
	out := make([]preparedLine, len(in))
	for i, ln := range in {
		out[i] = ln.preparedLine
	}
	return out
}

func validateCreditOriginal(orig *origInvoiceRow) error {
	switch orig.DocType {
	case string(domain.DocumentFT), string(domain.DocumentFS), string(domain.DocumentFR):
	default:
		return ErrCreditNotAllowed
	}
	switch orig.DocStatus {
	case string(domain.DocumentSigned), string(domain.DocumentCreditedPartial):
	default:
		return ErrCreditNotAllowed
	}
	return nil
}

func loadOriginalInvoice(tx *sql.Tx, storeID, invoiceID string) (*origInvoiceRow, error) {
	var o origInvoiceRow
	var displayMeta sql.NullString
	err := tx.QueryRow(`SELECT id, invoice_no, document_type, document_status, gross_total,
		COALESCE(credited_gross_total,'0.00'),
		COALESCE(source_system,''), COALESCE(source_sale_id,''), COALESCE(scope_type,''), COALESCE(scope_id,''),
		COALESCE(fiscal_purpose,''), display_meta_json
		FROM invoices WHERE id = ? AND store_id = ?`, invoiceID, storeID).
		Scan(&o.ID, &o.InvoiceNo, &o.DocType, &o.DocStatus, &o.GrossTotal, &o.CreditedGross,
			&o.SourceSystem, &o.SourceSaleID, &o.ScopeType, &o.ScopeID, &o.FiscalPurpose, &displayMeta)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if displayMeta.Valid {
		o.DisplayMetaJSON = displayMeta.String
	}
	return &o, nil
}

func loadOriginalLines(tx *sql.Tx, invoiceID string) ([]origLineRow, error) {
	rows, err := tx.Query(`SELECT id, line_number, product_code, product_description, COALESCE(display_name,''),
		quantity, unit_of_measure, unit_price_gross, unit_price_net, line_gross, line_net, line_tax,
		vat_rate, tax_code, product_type
		FROM invoice_lines WHERE invoice_id = ? ORDER BY line_number`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []origLineRow
	for rows.Next() {
		var l origLineRow
		if err := rows.Scan(&l.ID, &l.LineNumber, &l.ProductCode, &l.ProductDescription, &l.DisplayName,
			&l.Quantity, &l.UnitOfMeasure, &l.UnitPriceGross, &l.UnitPriceNet,
			&l.LineGross, &l.LineNet, &l.LineTax, &l.VATRate, &l.TaxCode, &l.ProductType); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func loadCreditedGrossByLine(tx *sql.Tx, originalInvoiceID string) (map[string]decimal.Decimal, error) {
	rows, err := tx.Query(`SELECT r.original_line_id, COALESCE(SUM(CAST(il.line_gross AS REAL)), 0)
		FROM invoice_line_references r
		JOIN invoice_lines il ON il.id = r.credit_line_id
		WHERE r.original_invoice_id = ?
		GROUP BY r.original_line_id`, originalInvoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var id string
		var sum float64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id], _ = compliance.ParseDecimal(fmt.Sprintf("%.2f", sum))
	}
	return out, rows.Err()
}

func loadOriginalCustomer(tx *sql.Tx, invoiceID string) (*origCustomerRow, error) {
	var c origCustomerRow
	err := tx.QueryRow(`SELECT customer_tax_id, company_name,
		COALESCE(address_detail,''), COALESCE(city,''), COALESCE(postal_code,''), country
		FROM invoice_customer_snapshots WHERE invoice_id = ?`, invoiceID).
		Scan(&c.TaxID, &c.CompanyName, &c.AddressDetail, &c.City, &c.PostalCode, &c.Country)
	if errors.Is(err, sql.ErrNoRows) {
		return &origCustomerRow{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"}, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func buildNCLines(origLines []origLineRow, creditedByLine map[string]decimal.Decimal, p IssueNCParams) (
	[]ncPreparedLine, decimal.Decimal, decimal.Decimal, decimal.Decimal, []compliance.TaxBucket, error,
) {
	lineByNo := map[int]origLineRow{}
	for _, ol := range origLines {
		lineByNo[ol.LineNumber] = ol
	}

	var inputs []CreditLineInput
	if p.CreditFull {
		for _, ol := range origLines {
			inputs = append(inputs, CreditLineInput{OriginalLineNumber: ol.LineNumber})
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
		origGross, _ := compliance.ParseDecimal(ol.LineGross)
		credited := creditedByLine[ol.ID]
		remaining := origGross.Sub(credited)
		if remaining.LessThanOrEqual(decimal.Zero) {
			continue
		}

		ln, err := creditLineFromInput(ol, remaining, in)
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, err
		}
		creditGross, _ := compliance.ParseDecimal(ln.LineGross)
		if creditGross.GreaterThan(remaining) {
			return nil, decimal.Zero, decimal.Zero, decimal.Zero, nil, ErrCreditAmountExceeded
		}
		if creditGross.LessThanOrEqual(decimal.Zero) {
			continue
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

func creditLineFromInput(ol origLineRow, remaining decimal.Decimal, in CreditLineInput) (preparedLine, error) {
	qtyStr := strings.TrimSpace(in.Quantity)
	grossStr := strings.TrimSpace(in.LineGross)

	if grossStr != "" && qtyStr != "" {
		return preparedLine{}, fmt.Errorf("store: specify quantity or line_gross, not both")
	}

	var lineGross, lineNet, lineTax, qty, unitNet string
	if grossStr != "" {
		g, err := compliance.ParseDecimal(grossStr)
		if err != nil {
			return preparedLine{}, err
		}
		lineGross, lineNet, lineTax, err = splitGrossAmount(compliance.Money2(g), ol.VATRate)
		if err != nil {
			return preparedLine{}, err
		}
		ug, _ := compliance.ParseDecimal(ol.UnitPriceGross)
		if ug.IsZero() {
			qty = "1"
		} else {
			qty = compliance.Money2(g.Div(ug))
		}
		unitNet = ol.UnitPriceNet
	} else {
		if qtyStr == "" {
			lineGross = compliance.Money2(remaining)
			var err error
			lineGross, lineNet, lineTax, err = splitGrossAmount(lineGross, ol.VATRate)
			if err != nil {
				return preparedLine{}, err
			}
			ug, _ := compliance.ParseDecimal(ol.UnitPriceGross)
			if ug.IsZero() {
				qty = "1"
			} else {
				qty = compliance.Money2(remaining.Div(ug))
			}
			unitNet = ol.UnitPriceNet
		} else {
			var err error
			lineGross, lineNet, lineTax, unitNet, err = compliance.LineFromGross(qtyStr, ol.UnitPriceGross, ol.VATRate)
			if err != nil {
				return preparedLine{}, err
			}
			qty = qtyStr
		}
	}

	pt := ol.ProductType
	if pt == "" {
		pt = "P"
	}
	uom := ol.UnitOfMeasure
	if uom == "" {
		uom = "UN"
	}

	return preparedLine{
		LineNumber:         0, // filled by caller
		ProductCode:        ol.ProductCode,
		ProductDescription: ol.ProductDescription,
		DisplayName:        ol.DisplayName,
		Quantity:           qty,
		UnitOfMeasure:      uom,
		UnitPriceGross:     ol.UnitPriceGross,
		UnitPriceNet:       unitNet,
		LineGross:          lineGross,
		LineNet:            lineNet,
		LineTax:            lineTax,
		VATRate:            ol.VATRate,
		TaxCode:            ol.TaxCode,
		ProductType:        pt,
	}, nil
}

func splitGrossAmount(gross, vatRate string) (lineGross, lineNet, lineTax string, err error) {
	g, err := compliance.ParseDecimal(gross)
	if err != nil {
		return "", "", "", err
	}
	rate, err := compliance.ParseDecimal(vatRate)
	if err != nil {
		return "", "", "", err
	}
	net := g.Div(decimal.NewFromInt(1).Add(rate))
	tax := g.Sub(net)
	return compliance.Money2(g), compliance.Money2(net), compliance.Money2(tax), nil
}

func creditPayloadHash(p IssueNCParams) string {
	type lineHash struct {
		OriginalLineNumber int    `json:"original_line_number"`
		Quantity           string `json:"quantity,omitempty"`
		LineGross          string `json:"line_gross,omitempty"`
	}
	body := struct {
		OriginalInvoiceID string     `json:"original_invoice_id"`
		Reason            string     `json:"reason"`
		CreditFull        bool       `json:"credit_full"`
		Lines             []lineHash `json:"lines,omitempty"`
	}{
		OriginalInvoiceID: p.OriginalInvoiceID,
		Reason:            p.Reason,
		CreditFull:        p.CreditFull,
	}
	if !p.CreditFull {
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

func creditModeFingerprint(p IssueNCParams) (mode, fingerprint string) {
	if p.CreditFull {
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

func creditBusinessKey(storeID, originalID, mode, fingerprint string) string {
	return fmt.Sprintf("%s|credit|%s|%s|%s", storeID, originalID, mode, fingerprint)
}

func tableNameFromMeta(displayMetaJSON string) string {
	if displayMetaJSON == "" {
		return ""
	}
	var meta struct {
		TableDisplayName string `json:"table_display_name"`
	}
	if json.Unmarshal([]byte(displayMetaJSON), &meta) == nil {
		return meta.TableDisplayName
	}
	return ""
}
