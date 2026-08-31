package service

import (
	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// ResolveSaleDocumentType is the ONLY parser for product sale document_type (FT/FS).
func ResolveSaleDocumentType(s string) (domain.DocumentType, error) {
	return domain.ParseSaleDocumentType(s)
}
