package store

import (
	"database/sql"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/shopspring/decimal"
)

// ReceiptInvoiceRemaining is FT receivable balance for RG.
type ReceiptInvoiceRemaining struct {
	GrossTotal               string `json:"gross_total"`
	CreditedGrossTotal       string `json:"credited_gross_total"`
	SettledAtIssueTotal      string `json:"settled_at_issue_total"`
	ReceivedGrossTotal       string `json:"received_gross_total"`
	RemainingReceivableTotal string `json:"remaining_receivable_total"`
}

// ReceiptRemainingForInvoice is the ONLY reader/calculator for FT receivable (RG ceiling).
func (d *DB) ReceiptRemainingForInvoice(invoiceID string) (*ReceiptInvoiceRemaining, error) {
	return receiptRemaining(d.SQL, invoiceID)
}

type receiptQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

func receiptRemaining(q receiptQuerier, invoiceID string) (*ReceiptInvoiceRemaining, error) {
	var gross, credited, received string
	err := q.QueryRow(`SELECT gross_total,
		COALESCE(credited_gross_total,'0.00'),
		COALESCE(received_gross_total,'0.00')
		FROM invoices WHERE id = ?`, invoiceID).
		Scan(&gross, &credited, &received)
	if err != nil {
		return nil, err
	}
	settled, err := settledAtIssueTotal(q, invoiceID)
	if err != nil {
		return nil, err
	}
	grossDec, _ := compliance.ParseDecimal(gross)
	creditedDec, _ := compliance.ParseDecimal(credited)
	receivedDec, _ := compliance.ParseDecimal(received)
	remaining := grossDec.Sub(creditedDec).Sub(settled).Sub(receivedDec)
	if remaining.IsNegative() {
		remaining = decimal.Zero
	}
	return &ReceiptInvoiceRemaining{
		GrossTotal:               compliance.Money2(grossDec),
		CreditedGrossTotal:       compliance.Money2(creditedDec),
		SettledAtIssueTotal:      compliance.Money2(settled),
		ReceivedGrossTotal:       compliance.Money2(receivedDec),
		RemainingReceivableTotal: compliance.Money2(remaining),
	}, nil
}

// settledAtIssueTotal sums invoice_payments that settle at issue (excludes ACCOUNT).
func settledAtIssueTotal(q receiptQuerier, invoiceID string) (decimal.Decimal, error) {
	rows, err := q.Query(`SELECT method, amount FROM invoice_payments WHERE invoice_id = ?`, invoiceID)
	if err != nil {
		return decimal.Zero, err
	}
	defer rows.Close()
	sum := decimal.Zero
	for rows.Next() {
		var method, amount string
		if err := rows.Scan(&method, &amount); err != nil {
			return decimal.Zero, err
		}
		if !domain.IsSettlingPaymentMethod(method) {
			continue
		}
		amt, err := compliance.ParseDecimal(amount)
		if err != nil {
			return decimal.Zero, fmt.Errorf("store: payment amount: %w", err)
		}
		sum = sum.Add(amt)
	}
	return sum, rows.Err()
}
