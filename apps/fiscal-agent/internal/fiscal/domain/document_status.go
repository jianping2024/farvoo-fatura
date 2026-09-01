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

// IsReprintableDocumentStatus reports whether an ORIGINAL invoice may enqueue a REPRINT job.
func IsReprintableDocumentStatus(s string) bool {
	st := DocumentStatus(s)
	for _, allowed := range IssuedOriginalDocumentStatuses {
		if st == allowed {
			return true
		}
	}
	return false
}

// IssuedOriginalDocumentStatusSQLIn returns SQL IN literals for IssuedOriginalDocumentStatuses (single source).
func IssuedOriginalDocumentStatusSQLIn() string {
	parts := make([]string, len(IssuedOriginalDocumentStatuses))
	for i, st := range IssuedOriginalDocumentStatuses {
		parts[i] = "'" + string(st) + "'"
	}
	return strings.Join(parts, ",")
}
