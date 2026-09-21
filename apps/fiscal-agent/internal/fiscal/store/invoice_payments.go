package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// insertInvoicePayment is the ONLY INSERT writer for invoice_payments
// (including tendered / change_due). Method must be a known code (empty → CASH).
func insertInvoicePayment(tx *sql.Tx, invoiceID, paidAt, operatorID string, pay domain.PaymentInput) error {
	method := domain.NormalizePaymentMethod(pay.Method)
	if !domain.IsKnownPaymentMethod(method) {
		return fmt.Errorf("store: unknown payment_method %q", pay.Method)
	}
	amount := strings.TrimSpace(pay.Amount)
	if amount == "" {
		return fmt.Errorf("store: payment amount required")
	}
	tendered := nullEmpty(pay.Tendered)
	changeDue := nullEmpty(pay.ChangeDue)
	_, err := tx.Exec(`INSERT INTO invoice_payments (
		id, invoice_id, method, amount, paid_at, operator_id, tendered, change_due
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), invoiceID, method, amount, paidAt, nullStr(operatorID), tendered, changeDue)
	return err
}

func nullEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
