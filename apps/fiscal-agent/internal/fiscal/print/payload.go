package print

// PayloadVersion is the frozen print snapshot schema version.
const PayloadVersion = 1

// Payload is the complete fiscal print snapshot written at issue time.
type Payload struct {
	Version       int    `json:"payload_version"`
	PayloadHash   string `json:"payload_hash"`
	DocumentID    string `json:"document_id"`
	DocumentType  string `json:"document_type"`
	PrintPurpose  string `json:"print_purpose"`
	InvoiceNo     string `json:"invoice_no"`
	IssuedAt      string `json:"issued_at"`
	Merchant      MerchantBlock `json:"merchant"`
	Customer      CustomerBlock `json:"customer"`
	Lines         []LineBlock   `json:"lines"`
	TaxSummary    []TaxSummaryRow `json:"tax_summary"`
	Totals        TotalsBlock   `json:"totals"`
	Payments      []PaymentBlock `json:"payments"`
	Compliance    ComplianceBlock `json:"compliance"`
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
	Method string `json:"method"`
	Amount string `json:"amount"`
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
