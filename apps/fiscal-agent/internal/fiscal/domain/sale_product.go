package domain

import (
	"fmt"
	"strings"
)

// DefaultSaleDocumentType is the product default when document_type is omitted.
const DefaultSaleDocumentType = DocumentFS

// SaleDocumentTypes are issuable sale invoices in restaurant product (FT + FS).
var SaleDocumentTypes = []DocumentType{DocumentFT, DocumentFS}

// IsSaleScopeDocumentType reports FT/FS used for bill-sync scope mutex and product UI.
func IsSaleScopeDocumentType(dt DocumentType) bool {
	switch dt {
	case DocumentFT, DocumentFS:
		return true
	default:
		return false
	}
}

// IsAdjustableOriginalDocumentType reports originals eligible for NC/ND in product (FT + FS).
func IsAdjustableOriginalDocumentType(dt DocumentType) bool {
	return IsSaleScopeDocumentType(dt)
}

// AdminInvoiceListDocumentTypes is the tab order on Admin invoice list (FT/FS/NC/ND).
var AdminInvoiceListDocumentTypes = []DocumentType{
	DocumentFT, DocumentFS, DocumentNC, DocumentND,
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
	return "", fmt.Errorf("document_type must be FT, FS, NC, or ND")
}

// ParseSaleDocumentType resolves product sale document type; empty → DefaultSaleDocumentType.
func ParseSaleDocumentType(s string) (DocumentType, error) {
	dt := DocumentType(strings.ToUpper(strings.TrimSpace(s)))
	if dt == "" {
		return DefaultSaleDocumentType, nil
	}
	if !IsSaleScopeDocumentType(dt) {
		return "", fmt.Errorf("document_type must be FT or FS")
	}
	return dt, nil
}
