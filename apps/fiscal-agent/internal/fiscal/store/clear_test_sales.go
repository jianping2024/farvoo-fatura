package store

import (
	"fmt"
	"strings"
)

// ClearTestFiscalSalesData wipes signed sales documents + drafts for pre-delivery handoff.
// Keeps taxpayer / series registry / operators / terminals / signing keys / product catalog.
// Resets series counters so the next FT starts clean after test wipe.
// Sole Agent entry for Farvoo fiscal_clear_jobs hang-queue.
func (d *DB) ClearTestFiscalSalesData() error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("store: db not open")
	}
	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`DELETE FROM print_attempts`,
		`DELETE FROM local_print_jobs`,
		`DELETE FROM invoice_line_references`,
		`DELETE FROM invoice_receipt_references`,
		`DELETE FROM invoice_proforma_references`,
		`DELETE FROM invoice_delivery_references`,
		`DELETE FROM invoice_payments`,
		`DELETE FROM invoice_customer_snapshots`,
		`DELETE FROM invoice_lines`,
		`DELETE FROM invoices`,
		`DELETE FROM bill_sync_drafts`,
		`DELETE FROM saft_exports`,
		`DELETE FROM idempotency_keys`,
		`UPDATE series SET last_number = 0, last_hash = ''`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			if strings.Contains(err.Error(), "no such table") {
				continue
			}
			return fmt.Errorf("clear test sales: %s: %w", q, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
