package service

import (
	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// ResolveSaleDocumentType is the ONLY parser for product sale document_type (FT/FS/FR).
func ResolveSaleDocumentType(s string) (domain.DocumentType, error) {
	return domain.ParseSaleDocumentType(s)
}

// ResolveBillSyncDocumentType is the ONLY parser for bill-draft issue document_type (FT/FS).
func ResolveBillSyncDocumentType(s string) (domain.DocumentType, error) {
	return domain.ParseBillSyncDocumentType(s)
}
