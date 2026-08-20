package domain

import "time"

// DocumentType is the AT invoice family code.
type DocumentType string

const (
	DocumentFT DocumentType = "FT"
	DocumentFS DocumentType = "FS"
	DocumentFR DocumentType = "FR"
	DocumentNC DocumentType = "NC"
	DocumentND DocumentType = "ND"
)

// DocumentStatus is the business lifecycle of a signed fiscal document.
type DocumentStatus string

const (
	DocumentDraft          DocumentStatus = "DRAFT"
	DocumentSigned         DocumentStatus = "SIGNED"
	DocumentCreditedPartial DocumentStatus = "CREDITED_PARTIAL"
	DocumentCreditedFull   DocumentStatus = "CREDITED_FULL"
)

// PrintStatus tracks physical output separately from tax status.
type PrintStatus string

const (
	PrintNotPrinted PrintStatus = "NOT_PRINTED"
	PrintPending    PrintStatus = "PENDING"
	PrintProcessing PrintStatus = "PROCESSING"
	PrintPrinted    PrintStatus = "PRINTED"
	PrintFailed     PrintStatus = "PRINT_FAILED"
	PrintReprinted  PrintStatus = "REPRINTED"
)

// PrintPurpose distinguishes first issue from reprint jobs.
type PrintPurpose string

const (
	PrintOriginal PrintPurpose = "ORIGINAL"
	PrintReprint  PrintPurpose = "REPRINT"
)

// SaleSnapshot is the only business input Fiscal Core accepts for issuance.
// POS / manual UI assemble this; Agent never reads live menu or inventory tables.
type SaleSnapshot struct {
	SourceSystem string
	SourceSaleID string
	ScopeType    string
	ScopeID      string
	FiscalPurpose string
	Lines        []SaleLine
	Customer     CustomerInput
	Payments     []PaymentInput
	ExternalBillID string
	DisplayMeta  map[string]string // e.g. table display name; not in SAF-T
}

// SaleLine is one sellable row at issue time.
type SaleLine struct {
	ProductCode     string
	DisplayName     string
	SaftName        string
	Quantity        string // decimal string
	UnitPriceGross  string
	VATRate         string // e.g. "0.06", "0.13", "0.23"
	ProductType     string // P, S, O
	UnitOfMeasure   string
}

// CustomerInput is the buyer at issue time.
type CustomerInput struct {
	TaxID       string
	CompanyName string
	Country     string
	AddressDetail string
	City        string
	PostalCode  string
}

// PaymentInput records how the sale was paid (local only; not SAF-T Payment node in MVP).
type PaymentInput struct {
	Method string
	Amount string
}

// IssueRequest wraps a sale snapshot with idempotency keys.
type IssueRequest struct {
	StoreID   string
	RequestID string
	Snapshot  SaleSnapshot
}

// IssueResult is returned after a successful FT (or FS) issuance.
type IssueResult struct {
	DocumentID    string
	InvoiceNo     string
	ATCUD         string
	DocumentType  DocumentType
	DocumentStatus DocumentStatus
	PrintJobID    string
	PrintStatus   PrintStatus
	IssuedAt      time.Time
}

// SignedDocument is the immutable tax record after transaction commit.
type SignedDocument struct {
	ID              string
	DocumentType    DocumentType
	SeriesCode      string
	SequenceNumber  int64
	InvoiceNo       string
	ATCUD           string
	Hash            string
	HashControl     int
	SigningKeyVersion int
	PreviousHash    string
	InvoiceDate     time.Time
	SystemEntryDate time.Time
	GrossTotal      string
	NetTotal        string
	TaxPayable      string
	QRContent       string
	CustomerTaxID   string
	SourceID        string // operator
}
