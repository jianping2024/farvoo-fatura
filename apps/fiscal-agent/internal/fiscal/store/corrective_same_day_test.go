package store

import "testing"

func TestAssertCorrectiveSameInvoiceDate(t *testing.T) {
	if err := assertCorrectiveSameInvoiceDate("2026-08-20", "2026-08-20"); err != nil {
		t.Fatal(err)
	}
	if err := assertCorrectiveSameInvoiceDate("2026-08-20", "2026-08-21"); err == nil {
		t.Fatal("expected cross-day error")
	}
}
