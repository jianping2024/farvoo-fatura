package domain

import (
	"fmt"
	"strings"
)

// DefaultSaleDocumentType is the product default when document_type is omitted.
const DefaultSaleDocumentType = DocumentFS

// SaleDocumentTypes are issuable sale invoices in restaurant product (FT + FS + FR).
var SaleDocumentTypes = []DocumentType{DocumentFT, DocumentFS, DocumentFR}

// IsSaleDocumentType reports whether dt is an issuable product sale type (manual + Local API).
func IsSaleDocumentType(dt DocumentType) bool {
	for _, t := range SaleDocumentTypes {
		if dt == t {
			return true
		}
	}
	return false
}

// IsSaleScopeDocumentType reports FT/FS used for bill-sync scope mutex (not FR).
func IsSaleScopeDocumentType(dt DocumentType) bool {
	switch dt {
	case DocumentFT, DocumentFS:
		return true
	default:
		return false
	}
}

// IsAdjustableOriginalDocumentType reports originals eligible for NC/ND in product (FT + FS + FR).
func IsAdjustableOriginalDocumentType(dt DocumentType) bool {
	return IsSaleDocumentType(dt)
}

// AdminInvoiceListDocumentTypes is the tab order on Admin invoice list (FT/FS/FR/NC/ND).
var AdminInvoiceListDocumentTypes = []DocumentType{
	DocumentFT, DocumentFS, DocumentFR, DocumentNC, DocumentND,
}

// ParseInvoiceListDocumentType validates document_type for GET /local/v1/fiscal-documents.
func ParseInvoiceListDocumentType(s string) (DocumentType, error) {
	dt := DocumentType(strings.ToUpper(strings.TrimSpace(s)))
	if dt == "" {
		return "", fmt.Errorf("document_type required")
	}
	for _, allowed := range AdminInvoiceListDocumentTypes {
		if dt == allowed {
			return dt, nil
		}
	}
	return "", fmt.Errorf("document_type must be FT, FS, FR, NC, or ND")
}

// ParseSaleDocumentType resolves product sale document type; empty → DefaultSaleDocumentType.
func ParseSaleDocumentType(s string) (DocumentType, error) {
	dt := DocumentType(strings.ToUpper(strings.TrimSpace(s)))
	if dt == "" {
		return DefaultSaleDocumentType, nil
	}
	if !IsSaleDocumentType(dt) {
		return "", fmt.Errorf("document_type must be FT, FS, or FR")
	}
	return dt, nil
}

// ParseBillSyncDocumentType resolves bill-draft issue type; empty → DefaultSaleDocumentType.
// Product rule: bill sync stays FT/FS only (FR is manual/API sale path).
func ParseBillSyncDocumentType(s string) (DocumentType, error) {
	dt := DocumentType(strings.ToUpper(strings.TrimSpace(s)))
	if dt == "" {
		return DefaultSaleDocumentType, nil
	}
	if !IsSaleScopeDocumentType(dt) {
		return "", fmt.Errorf("document_type must be FT or FS")
	}
	return dt, nil
}
