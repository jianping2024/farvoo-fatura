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
	"github.com/shopspring/decimal"
)

// ErrReceiptNotAllowed indicates the original invoice cannot receive an RG.
var ErrReceiptNotAllowed = errors.New("store: receipt not allowed")

// ErrReceiptAmountExceeded indicates RG amount exceeds remaining receivable.
var ErrReceiptAmountExceeded = errors.New("store: receipt amount exceeded")

// ErrRGSeriesMissing indicates no ACTIVE RG series with validation_code.
var ErrRGSeriesMissing = errors.New("store: RG series missing")

// IssueRGParams is input for IssueRG (validated by service.IssueReceipt).
type IssueRGParams struct {
	StoreID             string
	RequestID           string
	OriginalInvoiceID   string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Amount              string // empty when ReceiveFull
	ReceiveFull         bool
	PaymentMethod       string
	Reason              string
	InvoiceLocale       string
	NowUTC              time.Time
}

// IssueRG is the ONLY SQLite write path for RG receipts.
func (d *DB) IssueRG(ctx context.Context, signer Signer, p IssueRGParams) (*IssueRecord, error) {
	_ = ctx
	if p.NowUTC.IsZero() {
		p.NowUTC = time.Now().UTC()
	}
	payMethod := domain.NormalizePaymentMethod(p.PaymentMethod)
	if payMethod == domain.PaymentAccount {
		return nil, fmt.Errorf("store: receipt payment_method cannot be ACCOUNT")
	}
	if !domain.IsKnownPaymentMethod(payMethod) {
		return nil, fmt.Errorf("store: unknown payment_method %q", p.PaymentMethod)
	}
	reason := strings.TrimSpace(p.Reason)
	if reason == "" {
		reason = "Recibo"
	}

	payloadHash := receiptPayloadHash(p, payMethod, reason)
	businessKey := receiptBusinessKey(p.StoreID, p.OriginalInvoiceID, p.ReceiveFull, p.Amount, payMethod)

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
	if err := validateReceiptOriginal(orig); err != nil {
		return nil, err
	}

	rem, err := receiptRemaining(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}
	remainingDec, _ := compliance.ParseDecimal(rem.RemainingReceivableTotal)
	if remainingDec.IsZero() {
		return nil, ErrReceiptAmountExceeded
	}

	var rgGross decimal.Decimal
	if p.ReceiveFull {
		rgGross = remainingDec
	} else {
		rgGross, err = compliance.ParseDecimal(strings.TrimSpace(p.Amount))
		if err != nil || !rgGross.IsPositive() {
			return nil, fmt.Errorf("store: receipt amount must be positive")
		}
		if rgGross.GreaterThan(remainingDec) {
			return nil, ErrReceiptAmountExceeded
		}
	}
	grossStr := compliance.Money2(rgGross)
	netStr := grossStr // receipt: no VAT re-charge; tax already on FT
	taxStr := "0.00"

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
		FROM series WHERE store_id = ? AND document_type = 'RG' AND status = 'ACTIVE'
		ORDER BY fiscal_year DESC LIMIT 1`, p.StoreID).
		Scan(&seriesID, &seriesCode, &validationCode, &lastNumber, &lastHash)
	if err != nil {
		return nil, ErrRGSeriesMissing
	}
	if validationCode == "" {
		return nil, ErrRGSeriesMissing
	}

	seq := lastNumber + 1
	invoiceNo := compliance.FormatInvoiceNo("RG", seriesCode, seq)
	atcud := compliance.FormatATCUD(validationCode, seq)

	signPayload := compliance.BuildSignPayload(invoiceDate, systemEntry, invoiceNo, grossStr, lastHash)
	hashB64, hashControl, keyVersion, err := signer.Sign(signPayload)
	if err != nil {
		return nil, fmt.Errorf("store: sign: %w", err)
	}

	cust, err := loadOriginalCustomer(tx, p.OriginalInvoiceID)
	if err != nil {
		return nil, err
	}

	buckets := []compliance.TaxBucket{{
		Rate: "0.00", TaxBase: netStr, TaxAmount: taxStr,
	}}
	qr, err := compliance.BuildQR(compliance.QRInput{
		IssuerNIF:              nif,
		CustomerTaxID:          cust.TaxID,
		CustomerCountry:        cust.Country,
		DocumentType:           "RG",
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

	tableName := tableNameFromMeta(orig.DisplayMetaJSON)
	printPayload, payloadHashPrint, err := fiscalprint.BuildPayload(fiscalprint.BuildInput{
		DocumentID:                docID,
		DocumentType:              "RG",
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
		Lines: []fiscalprint.LineAmounts{{
			LineNumber:         1,
			ProductCode:        "RG-PAY",
			ProductDescription: "Recibo",
			DisplayName:        "Recibo",
			Quantity:           "1",
			UnitOfMeasure:      "UN",
			UnitPriceGross:     grossStr,
			UnitPriceNet:       netStr,
			LineGross:          grossStr,
			LineNet:            netStr,
			LineTax:            taxStr,
			VATRate:            "0.00",
			TaxCode:            "NS",
			ProductType:        "S",
		}},
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
		CreditReason:              reason,
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
	sourceSystem := orig.SourceSystem
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
		fiscal_terminal_id, fiscal_terminal_label
	) VALUES (?, ?, 'RG', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SIGNED', 'PENDING',
		?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, NULL, ?, '0.00', '0.00', ?, ?, ?)`,
		docID, p.StoreID, seriesID, seriesCode, seq, invoiceNo,
		atcud, hashB64, hashControl, keyVersion, lastHash, qr,
		invoiceDate, systemEntry,
		grossStr, netStr, taxStr, p.OperatorID,
		cert, nullStr(sourceSystem), nullStr(orig.SourceSaleID),
		nullStr(orig.ScopeType), nullStr(orig.ScopeID),
		"receipt", nullStr(orig.DisplayMetaJSON), nowRFC, termID, termLabel)
	if err != nil {
		return nil, fmt.Errorf("store: insert RG invoice: %w", err)
	}

	lineID := uuid.NewString()
	_, err = tx.Exec(`INSERT INTO invoice_lines (
		id, invoice_id, line_number, product_code, product_description, display_name,
		quantity, unit_of_measure, unit_price_gross, unit_price_net, line_gross, line_net, line_tax,
		vat_rate, tax_type, tax_country_region, tax_code, tax_exemption_code, tax_exemption_reason, product_type
	) VALUES (?, ?, 1, 'RG-PAY', 'Recibo', 'Recibo', '1', 'UN', ?, ?, ?, ?, ?, '0.00', 'IVA', ?, 'NS', 'M99', 'Não sujeito — recibo', 'S')`,
		lineID, docID, grossStr, netStr, grossStr, netStr, taxStr, taxRegion)
	if err != nil {
		return nil, fmt.Errorf("store: insert RG line: %w", err)
	}

	_, err = tx.Exec(`INSERT INTO invoice_receipt_references (
		id, receipt_invoice_id, original_invoice_id, original_invoice_no, amount
	) VALUES (?, ?, ?, ?, ?)`,
		uuid.NewString(), docID, p.OriginalInvoiceID, orig.InvoiceNo, grossStr)
	if err != nil {
		return nil, fmt.Errorf("store: insert receipt reference: %w", err)
	}

	_, err = tx.Exec(`INSERT INTO invoice_customer_snapshots (
		invoice_id, customer_tax_id, company_name, address_detail, city, postal_code, country, account_id, self_billing_indicator
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'Desconhecido', 0)`,
		docID, cust.TaxID, cust.CompanyName, cust.AddressDetail, cust.City, cust.PostalCode, cust.Country)
	if err != nil {
		return nil, err
	}

	if err = insertInvoicePayment(tx, docID, nowRFC, p.OperatorID, domain.PaymentInput{
		Method: payMethod, Amount: grossStr,
	}); err != nil {
		return nil, err
	}

	receivedDec, _ := compliance.ParseDecimal(rem.ReceivedGrossTotal)
	newReceived := compliance.Money2(receivedDec.Add(rgGross))
	_, err = tx.Exec(`UPDATE invoices SET received_gross_total = ? WHERE id = ?`,
		newReceived, p.OriginalInvoiceID)
	if err != nil {
		return nil, fmt.Errorf("store: update original received: %w", err)
	}

	payloadJSON, err := json.Marshal(printPayload)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`INSERT INTO local_print_jobs (
		id, invoice_id, document_type, print_purpose, job_status, logical_role,
		payload_json, payload_hash, attempts, last_error, created_at, updated_at, printed_at, created_by, station_id
	) VALUES (?, ?, 'RG', 'ORIGINAL', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
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
		DocumentType:   domain.DocumentRG,
		DocumentStatus: domain.DocumentSigned,
		PrintJobID:     printJobID,
		PrintStatus:    domain.PrintPending,
		IssuedAt:       p.NowUTC,
		Hash:           hashB64,
		QRContent:      qr,
		IdempotentHit:  false,
	}, nil
}

func validateReceiptOriginal(orig *origInvoiceRow) error {
	if orig.DocType != string(domain.DocumentFT) {
		return ErrReceiptNotAllowed
	}
	switch orig.DocStatus {
	case string(domain.DocumentSigned),
		string(domain.DocumentCreditedPartial),
		string(domain.DocumentDebitedPartial),
		string(domain.DocumentDebitedFull):
		return nil
	default:
		// CREDITED_FULL may still have receivable if somehow settled inconsistently;
		// still allow only when remaining > 0 (checked by caller). Reject obvious cancelled-like.
		if orig.DocStatus == string(domain.DocumentCreditedFull) {
			return nil
		}
		return ErrReceiptNotAllowed
	}
}

func receiptPayloadHash(p IssueRGParams, payMethod, reason string) string {
	body := struct {
		OriginalInvoiceID string `json:"original_invoice_id"`
		Amount            string `json:"amount,omitempty"`
		ReceiveFull       bool   `json:"receive_full"`
		PaymentMethod     string `json:"payment_method"`
		Reason            string `json:"reason"`
	}{
		OriginalInvoiceID: p.OriginalInvoiceID,
		Amount:            strings.TrimSpace(p.Amount),
		ReceiveFull:       p.ReceiveFull,
		PaymentMethod:     payMethod,
		Reason:            reason,
	}
	return hashJSON(body)
}

func receiptBusinessKey(storeID, originalID string, receiveFull bool, amount, payMethod string) string {
	mode := "partial:" + strings.TrimSpace(amount)
	if receiveFull {
		mode = "full"
	}
	sum := sha256.Sum256([]byte(storeID + "|" + originalID + "|RG|" + mode + "|" + payMethod))
	return "rg:" + hex.EncodeToString(sum[:16])
}

// Ensure tx implements receiptQuerier (*sql.Tx has Query/QueryRow).
var _ receiptQuerier = (*sql.Tx)(nil)
