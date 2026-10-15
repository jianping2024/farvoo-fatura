package print

// PayloadVersion is the frozen print snapshot schema version.
const PayloadVersion = 1

// Payload is the complete fiscal print snapshot written at issue time.
type Payload struct {
	Version          int    `json:"payload_version"`
	PayloadHash      string `json:"payload_hash"`
	DocumentID       string `json:"document_id"`
	DocumentType     string `json:"document_type"`
	PrintPurpose     string `json:"print_purpose"`
	InvoiceNo        string `json:"invoice_no"`
	IssuedAt         string `json:"issued_at"`
	Locale           string `json:"locale,omitempty"` // invoice chrome: en | pt (scheme A); empty → pt at render
	TableDisplayName string `json:"table_display_name,omitempty"`
	Merchant         MerchantBlock  `json:"merchant"`
	Customer         CustomerBlock  `json:"customer"`
	Lines            []LineBlock    `json:"lines"`
	TaxSummary       []TaxSummaryRow `json:"tax_summary"`
	Totals           TotalsBlock    `json:"totals"`
	Payments         []PaymentBlock  `json:"payments"`
	Compliance       ComplianceBlock `json:"compliance"`
	Transport        TransportBlock  `json:"transport,omitempty"` // GR/GT movement fields
}

// TransportBlock holds GR/GT ship addresses and AT transport doc code.
type TransportBlock struct {
	ShipFromAddress     string `json:"ship_from_address,omitempty"`
	ShipToAddress       string `json:"ship_to_address,omitempty"`
	MovementStartAt     string `json:"movement_start_at,omitempty"`
	ATDocCodeID         string `json:"at_doc_code_id,omitempty"`
	RecipientIsVATPayer bool   `json:"recipient_is_vat_payer,omitempty"`
	MovementPurpose     string `json:"movement_purpose,omitempty"` // GT: transfer | customer
}

type MerchantBlock struct {
	LegalName                 string `json:"legal_name"`
	BusinessName              string `json:"business_name,omitempty"`
	TaxRegistrationNumber     string `json:"tax_registration_number"`
	Address                   string `json:"address"`
	SoftwareCertificateNumber string `json:"software_certificate_number"`
}

type CustomerBlock struct {
	TaxID       string `json:"tax_id"`
	CompanyName string `json:"company_name"`
	Country     string `json:"country"`
}

type LineBlock struct {
	Description    string `json:"description"`
	DisplayName    string `json:"display_name,omitempty"`
	Quantity       string `json:"quantity"`
	UnitPriceGross string `json:"unit_price_gross"`
	VATRate        string `json:"vat_rate"`
	LineGross      string `json:"line_gross"`
	LineNet        string `json:"line_net"`
	LineTax        string `json:"line_tax"`
}

type TaxSummaryRow struct {
	VATRate   string `json:"vat_rate"`
	TaxBase   string `json:"tax_base"`
	TaxAmount string `json:"tax_amount"`
	Gross     string `json:"gross"`
}

type TotalsBlock struct {
	NetTotal   string `json:"net_total"`
	TaxPayable string `json:"tax_payable"`
	GrossTotal string `json:"gross_total"`
}

type PaymentBlock struct {
	Method    string `json:"method"`
	Amount    string `json:"amount"`
	Tendered  string `json:"tendered,omitempty"`
	ChangeDue string `json:"change_due,omitempty"`
}

type ComplianceBlock struct {
	ATCUD              string `json:"atcud"`
	QR                 QRBlock `json:"qr"`
	HashControlChars   string `json:"hash_control_chars"`
	CertificationLine  string `json:"certification_line"`
	OriginalInvoiceNo  string `json:"original_invoice_no,omitempty"`
	CreditReason       string `json:"credit_reason,omitempty"`
}

type QRBlock struct {
	Content string `json:"content"`
}
