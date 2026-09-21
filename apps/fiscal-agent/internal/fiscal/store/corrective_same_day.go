package store

import (
	"errors"
	"fmt"
	"strings"
)

// ErrCorrectiveCrossDay is returned when NC/ND invoice_date ≠ original invoice_date.
var ErrCorrectiveCrossDay = errors.New("store: corrective must be same invoice_date as original")

// assertCorrectiveSameInvoiceDate is the ONLY same-day gate for NC/ND vs original.
func assertCorrectiveSameInvoiceDate(origInvoiceDate, correctiveInvoiceDate string) error {
	o := strings.TrimSpace(origInvoiceDate)
	c := strings.TrimSpace(correctiveInvoiceDate)
	if o == "" || c == "" {
		return fmt.Errorf("store: missing invoice_date for corrective check")
	}
	if o != c {
		return ErrCorrectiveCrossDay
	}
	return nil
}
