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
