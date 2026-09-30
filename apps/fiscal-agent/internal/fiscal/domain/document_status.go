package domain

import "strings"

// IssuedOriginalDocumentStatuses is the shared status set for reprint gate and bill-sync "already invoiced".
var IssuedOriginalDocumentStatuses = []DocumentStatus{
	DocumentSigned,
	DocumentCreditedPartial,
	DocumentCreditedFull,
	DocumentDebitedPartial,
	DocumentDebitedFull,
}

// IsReprintableDocumentStatus reports whether an ORIGINAL sale/NC/ND/RG may enqueue a REPRINT job.
// For PF use IsReprintableDocument (doc-type aware).
func IsReprintableDocumentStatus(s string) bool {
	st := DocumentStatus(s)
	for _, allowed := range IssuedOriginalDocumentStatuses {
		if st == allowed {
			return true
		}
	}
	return false
}

// IsReprintableDocument is the ONLY reprint status gate (sale statuses or PF N/F).
func IsReprintableDocument(docType, docStatus string) bool {
	if DocumentType(docType) == DocumentPF {
		switch DocumentStatus(docStatus) {
		case WorkStatusNormal, WorkStatusInvoiced:
			return true
		default:
			return false
		}
	}
	return IsReprintableDocumentStatus(docStatus)
}

// IssuedOriginalDocumentStatusSQLIn returns SQL IN literals for IssuedOriginalDocumentStatuses (single source).
func IssuedOriginalDocumentStatusSQLIn() string {
	parts := make([]string, len(IssuedOriginalDocumentStatuses))
	for i, st := range IssuedOriginalDocumentStatuses {
		parts[i] = "'" + string(st) + "'"
	}
	return strings.Join(parts, ",")
}
