package store

import (
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// insertInvoicePayment is the ONLY INSERT writer for invoice_payments
// (including tendered / change_due).
func insertInvoicePayment(tx *sql.Tx, invoiceID, paidAt, operatorID string, pay domain.PaymentInput) error {
	tendered := nullEmpty(pay.Tendered)
	changeDue := nullEmpty(pay.ChangeDue)
	_, err := tx.Exec(`INSERT INTO invoice_payments (
		id, invoice_id, method, amount, paid_at, operator_id, tendered, change_due
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), invoiceID, pay.Method, pay.Amount, paidAt, nullStr(operatorID), tendered, changeDue)
	return err
}

func nullEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
