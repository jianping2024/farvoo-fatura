package compliance

import (
	"fmt"
	"strconv"
)

// FormatSequence is the ONLY sequence formatter for InvoiceNo, ATCUD, Hash, QR, print, SAF-T.
// Internal DB stores int; this is the sole display/string form (v0.17 §2.7.4).
func FormatSequence(n int64) string {
	return strconv.FormatInt(n, 10)
}

// FormatInvoiceNo builds "FT SERIES/n" (space after type).
func FormatInvoiceNo(docType, seriesCode string, sequence int64) string {
	return fmt.Sprintf("%s %s/%s", docType, seriesCode, FormatSequence(sequence))
}

// FormatATCUD builds "VALIDATION-n" without "ATCUD:" prefix.
func FormatATCUD(validationCode string, sequence int64) string {
	return fmt.Sprintf("%s-%s", validationCode, FormatSequence(sequence))
}
