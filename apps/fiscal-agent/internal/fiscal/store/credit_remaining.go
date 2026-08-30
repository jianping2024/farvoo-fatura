package store

import (
	"database/sql"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/compliance"

	"github.com/shopspring/decimal"
)

// CreditLineRemaining is one original line with remaining credit gross.
type CreditLineRemaining struct {
	LineID             string `json:"line_id"`
	LineNumber         int    `json:"line_number"`
	Description        string `json:"description"`
	LineGross          string `json:"line_gross"`
	RemainingLineGross string `json:"remaining_line_gross"`
}

// CreditInvoiceRemaining aggregates credit balances for an invoice.
type CreditInvoiceRemaining struct {
	CreditedGrossTotal  string                `json:"credited_gross_total"`
	RemainingGrossTotal string                `json:"remaining_gross_total"`
	Lines               []CreditLineRemaining `json:"lines"`
}

type creditLineQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// loadCreditedGrossByLine aggregates credited gross per original line id.
func loadCreditedGrossByLine(q creditLineQuerier, originalInvoiceID string) (map[string]decimal.Decimal, error) {
	rows, err := q.Query(`SELECT r.original_line_id, COALESCE(SUM(CAST(il.line_gross AS REAL)), 0)
		FROM invoice_line_references r
		JOIN invoice_lines il ON il.id = r.credit_line_id
		WHERE r.original_invoice_id = ?
		GROUP BY r.original_line_id`, originalInvoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]decimal.Decimal{}
	for rows.Next() {
		var id string
		var sum float64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id], _ = compliance.ParseDecimal(fmt.Sprintf("%.2f", sum))
	}
	return out, rows.Err()
}

// CreditRemainingForInvoice is the ONLY reader for per-line remaining credit gross.
func (d *DB) CreditRemainingForInvoice(invoiceID string) (*CreditInvoiceRemaining, error) {
	var gross, credited string
	err := d.SQL.QueryRow(`SELECT gross_total, COALESCE(credited_gross_total,'0.00') FROM invoices WHERE id = ?`, invoiceID).
		Scan(&gross, &credited)
	if err != nil {
		return nil, err
	}
	grossDec, _ := compliance.ParseDecimal(gross)
	creditedDec, _ := compliance.ParseDecimal(credited)
	remainingTot := grossDec.Sub(creditedDec)
	if remainingTot.IsNegative() {
		remainingTot = decimal.Zero
	}

	creditedByLine, err := loadCreditedGrossByLine(d.SQL, invoiceID)
	if err != nil {
		return nil, err
	}

	rows, err := d.SQL.Query(`SELECT id, line_number, COALESCE(display_name,''), product_description, line_gross
		FROM invoice_lines WHERE invoice_id = ? ORDER BY line_number`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []CreditLineRemaining
	for rows.Next() {
		var l CreditLineRemaining
		var desc, display string
		if err := rows.Scan(&l.LineID, &l.LineNumber, &display, &desc, &l.LineGross); err != nil {
			return nil, err
		}
		if display != "" {
			l.Description = display
		} else {
			l.Description = desc
		}
		origGross, _ := compliance.ParseDecimal(l.LineGross)
		creditedLine := creditedByLine[l.LineID]
		rem := origGross.Sub(creditedLine)
		if rem.IsNegative() {
			rem = decimal.Zero
		}
		l.RemainingLineGross = compliance.Money2(rem)
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &CreditInvoiceRemaining{
		CreditedGrossTotal:  credited,
		RemainingGrossTotal: compliance.Money2(remainingTot),
		Lines:               lines,
	}, nil
}
