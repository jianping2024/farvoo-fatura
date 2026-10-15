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
	DocumentRG DocumentType = "RG"
	DocumentPF DocumentType = "PF"
	DocumentGR DocumentType = "GR"
	DocumentGT DocumentType = "GT"
)

// DocumentStatus is the lifecycle of a signed fiscal document (no DRAFT in DB).
// For document_type=PF or GR, invoices.document_status stores N / A / F (SAF-T work/movement status).
// For document_type=GT, only N / A (no F — GT does not link to FT).
type DocumentStatus string

const (
	DocumentSigned          DocumentStatus = "SIGNED"
	DocumentCreditedPartial DocumentStatus = "CREDITED_PARTIAL"
	DocumentCreditedFull    DocumentStatus = "CREDITED_FULL"
	DocumentDebitedPartial  DocumentStatus = "DEBITED_PARTIAL"
	DocumentDebitedFull     DocumentStatus = "DEBITED_FULL"

	// PF / GR / GT status — work/movement status letters.
	WorkStatusNormal   DocumentStatus = "N"
	WorkStatusAnnulled DocumentStatus = "A"
	WorkStatusInvoiced DocumentStatus = "F" // PF/GR only
)

// GT movement_purpose values (P0).
const (
	MovementPurposeTransfer = "transfer"
	MovementPurposeCustomer = "customer"
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
type SaleSnapshot struct {
	SourceSystem   string            `json:"source_system"`
	SourceSaleID   string            `json:"source_sale_id"`
	ScopeType      string            `json:"scope_type"`
	ScopeID        string            `json:"scope_id"`
	FiscalPurpose  string            `json:"fiscal_purpose"`
	Lines          []SaleLine        `json:"lines"`
	Customer       CustomerInput     `json:"customer"`
	Payments       []PaymentInput    `json:"payments"`
	ExternalBillID string            `json:"external_bill_id,omitempty"`
	DisplayMeta    map[string]string `json:"display_meta,omitempty"`
}

// SaleLine is one sellable row at issue time.
type SaleLine struct {
	ProductCode    string `json:"product_code"`
	DisplayName    string `json:"display_name"`
	SaftName       string `json:"saft_name"`
	Quantity       string `json:"quantity"`
	UnitPriceGross string `json:"unit_price_gross"`
	VATRate        string `json:"vat_rate"`
	ProductType    string `json:"product_type"`
	UnitOfMeasure  string `json:"unit_of_measure"`
}

// CustomerInput is the buyer at issue time.
type CustomerInput struct {
	TaxID         string `json:"tax_id"`
	CompanyName   string `json:"company_name"`
	Country       string `json:"country"`
	AddressDetail string `json:"address_detail"`
	City          string `json:"city"`
	PostalCode    string `json:"postal_code"`
}

// PaymentInput records how the sale was paid.
// Amount is the settlement amount (SAF-T PaymentAmount). Tendered/ChangeDue are
// cash-drawer operational fields only (CASH); empty when unused.
type PaymentInput struct {
	Method    string `json:"method"`
	Amount    string `json:"amount"`
	Tendered  string `json:"tendered,omitempty"`
	ChangeDue string `json:"change_due,omitempty"`
}

// CreditLineRequest is one partial credit line for IssueCreditNote.
type CreditLineRequest struct {
	OriginalLineNumber int    `json:"original_line_number"`
	Quantity           string `json:"quantity,omitempty"`
	LineGross          string `json:"line_gross,omitempty"`
}

// CreditNoteRequest wraps NC issuance input.
type CreditNoteRequest struct {
	StoreID             string
	RequestID           string
	OriginalInvoiceID   string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Reason              string
	CreditFull          bool
	Lines               []CreditLineRequest
}

// DebitNoteRequest wraps ND issuance input.
type DebitNoteRequest struct {
	StoreID             string
	RequestID           string
	OriginalInvoiceID   string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Reason              string
	DebitFull           bool
	Lines               []CreditLineRequest
}

// ReceiptRequest wraps RG (recibo) issuance against an FT.
type ReceiptRequest struct {
	StoreID             string
	RequestID           string
	OriginalInvoiceID   string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Amount              string // required unless ReceiveFull
	ReceiveFull         bool
	PaymentMethod       string
	Reason              string
}

// IssueRequest wraps a sale snapshot with idempotency keys.
type IssueRequest struct {
	StoreID             string
	RequestID           string
	OperatorID          string
	StationID           string // station_printers key for ORIGINAL fiscal print
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Snapshot            SaleSnapshot
	ProformaID          string   // optional; sale → PF soft link (max 1)
	DeliveryNoteIDs     []string // optional; FT → 1..N GR soft links
}

// ProformaRequest wraps PF (pró-forma) issuance.
type ProformaRequest struct {
	StoreID             string
	RequestID           string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Snapshot            SaleSnapshot
	ValidUntil          string // optional YYYY-MM-DD
}

// AnnulProformaRequest wraps PF annulment (N→A only).
type AnnulProformaRequest struct {
	StoreID             string
	RequestID           string
	DocumentID          string
	OperatorID          string
	Reason              string
}

// DeliveryNoteRequest wraps GR (guia de remessa) issuance.
type DeliveryNoteRequest struct {
	StoreID              string
	RequestID            string
	OperatorID           string
	StationID            string
	FiscalTerminalID     string
	FiscalTerminalLabel  string
	Snapshot             SaleSnapshot
	ShipFromAddress      string
	ShipToAddress        string
	MovementStartAt      string // local wall YYYY-MM-DDTHH:MM:SS or RFC3339
	RecipientIsVATPayer  bool
	ATDocCodeID          string // from TransportClient; required before store write
}

// AnnulDeliveryNoteRequest wraps GR annulment (N→A only).
type AnnulDeliveryNoteRequest struct {
	StoreID    string
	RequestID  string
	DocumentID string
	OperatorID string
	Reason     string
}

// TransportNoteRequest wraps GT (guia de transporte) issuance.
type TransportNoteRequest struct {
	StoreID             string
	RequestID           string
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
	Snapshot            SaleSnapshot
	MovementPurpose     string // transfer | customer
	ShipFromAddress     string
	ShipToAddress       string
	MovementStartAt     string
	RecipientIsVATPayer bool
}

// AnnulTransportNoteRequest wraps GT annulment (N→A only).
type AnnulTransportNoteRequest struct {
	StoreID    string
	RequestID  string
	DocumentID string
	OperatorID string
	Reason     string
}

// IssueResult is returned after a successful FT issuance.
type IssueResult struct {
	DocumentID      string         `json:"DocumentID"`
	InvoiceNo       string         `json:"InvoiceNo"`
	ATCUD           string         `json:"ATCUD"`
	DocumentType    DocumentType   `json:"DocumentType"`
	DocumentStatus  DocumentStatus `json:"DocumentStatus"`
	PrintJobID      string         `json:"PrintJobID"`
	PrintStatus     PrintStatus    `json:"PrintStatus"`
	IssuedAt        time.Time      `json:"IssuedAt"`
	IdempotentHit   bool           `json:"IdempotentHit"`
	CleanupPending  bool           `json:"cleanup_pending,omitempty"`
}
