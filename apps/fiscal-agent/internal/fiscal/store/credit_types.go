package store

import "farvoo-fiscal-agent/internal/fiscal/domain"

// IsCreditableOriginalDocumentType reports whether document_type can be credited (FT/FS/FR).
func IsCreditableOriginalDocumentType(docType domain.DocumentType) bool {
	switch docType {
	case domain.DocumentFT, domain.DocumentFS, domain.DocumentFR:
		return true
	default:
		return false
	}
}
