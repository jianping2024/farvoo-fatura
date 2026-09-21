package store

import (
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// InvoiceRevenueSummaryQuery filters hub revenue stats (date + optional terminal only).
type InvoiceRevenueSummaryQuery struct {
	StoreID          string
	From             string // invoice_date YYYY-MM-DD inclusive
	To               string
	FiscalTerminalID string // empty = all terminals; domain.LoopbackFiscalTerminalID = Agent host
}

// InvoiceRevenueTerminalOption is one PC option for the hub terminal dropdown.
type InvoiceRevenueTerminalOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// InvoiceRevenueSummary is the ONLY hub net-revenue payload (cash / non-cash buckets).
type InvoiceRevenueSummary struct {
	InvoiceCount    int
	GrossNetSum     string // FT+FS+ND − NC
	CashGrossSum    string
	NonCashGrossSum string
	Terminals       []InvoiceRevenueTerminalOption
}

// InvoiceRevenueSummary is the ONLY reader for Admin hub net sales / cash / non-cash.
// Document type and search must NOT affect this query (product P0).
func (d *DB) InvoiceRevenueSummary(q InvoiceRevenueSummaryQuery) (*InvoiceRevenueSummary, error) {
	storeID := strings.TrimSpace(q.StoreID)
	if storeID == "" {
		storeID = "store-demo-001"
	}
	from := strings.TrimSpace(q.From)
	to := strings.TrimSpace(q.To)
	termID := strings.TrimSpace(q.FiscalTerminalID)

	where := `FROM invoices i WHERE i.store_id = ?`
	args := []any{storeID}
	if from != "" {
		where += ` AND i.invoice_date >= ?`
		args = append(args, from)
	}
	if to != "" {
		where += ` AND i.invoice_date <= ?`
		args = append(args, to)
	}
	if termID != "" {
		where += ` AND IFNULL(i.fiscal_terminal_id,'') = ?`
		args = append(args, termID)
	}

	payMethod := `UPPER(IFNULL(` + primaryPaymentMethodSubquery + `, ''))`
	// Missing payment row is data corruption: exclude from cash/non-cash (do not invent CASH).
	// Net identity holds for rows that have a known primary payment.
	cashExpr := `CASE
		WHEN ` + payMethod + ` = '' THEN 0
		WHEN ` + payMethod + ` = 'CASH' AND i.document_type IN ('FT','FS','FR','ND') THEN CAST(i.gross_total AS REAL)
		WHEN ` + payMethod + ` = 'CASH' AND i.document_type = 'NC' THEN -CAST(i.gross_total AS REAL)
		ELSE 0 END`
	nonCashExpr := `CASE
		WHEN ` + payMethod + ` = '' THEN 0
		WHEN ` + payMethod + ` != 'CASH' AND i.document_type IN ('FT','FS','FR','ND') THEN CAST(i.gross_total AS REAL)
		WHEN ` + payMethod + ` != 'CASH' AND i.document_type = 'NC' THEN -CAST(i.gross_total AS REAL)
		ELSE 0 END`

	var count int
	var cash, nonCash float64
	sumSQL := `SELECT COUNT(*),
		COALESCE(SUM(` + cashExpr + `), 0),
		COALESCE(SUM(` + nonCashExpr + `), 0) ` + where
	if err := d.SQL.QueryRow(sumSQL, args...).Scan(&count, &cash, &nonCash); err != nil {
		return nil, err
	}
	net := cash + nonCash

	terminals, err := d.listRevenueTerminalOptions(storeID, from, to)
	if err != nil {
		return nil, err
	}

	return &InvoiceRevenueSummary{
		InvoiceCount:    count,
		GrossNetSum:     fmt.Sprintf("%.2f", net),
		CashGrossSum:    fmt.Sprintf("%.2f", cash),
		NonCashGrossSum: fmt.Sprintf("%.2f", nonCash),
		Terminals:       terminals,
	}, nil
}

// listRevenueTerminalOptions: active terminals + loopback if used + inactive that issued in range.
func (d *DB) listRevenueTerminalOptions(storeID, from, to string) ([]InvoiceRevenueTerminalOption, error) {
	seen := map[string]InvoiceRevenueTerminalOption{}

	// Always offer loopback (Agent host).
	seen[domain.LoopbackFiscalTerminalID] = InvoiceRevenueTerminalOption{
		ID:    domain.LoopbackFiscalTerminalID,
		Label: domain.FiscalTerminalDisplayName("", "127.0.0.1"),
	}

	rows, err := d.ListFiscalTerminals(storeID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !row.Active {
			continue
		}
		seen[row.ID] = InvoiceRevenueTerminalOption{
			ID:    row.ID,
			Label: domain.FiscalTerminalDisplayName(row.Label, row.LastSeenIP),
		}
	}

	// Inactive / unknown ids that appear on invoices in the date window.
	histWhere := `FROM invoices WHERE store_id = ? AND IFNULL(fiscal_terminal_id,'') != ''`
	histArgs := []any{storeID}
	if from != "" {
		histWhere += ` AND invoice_date >= ?`
		histArgs = append(histArgs, from)
	}
	if to != "" {
		histWhere += ` AND invoice_date <= ?`
		histArgs = append(histArgs, to)
	}
	histSQL := `SELECT DISTINCT fiscal_terminal_id, IFNULL(fiscal_terminal_label,'') ` + histWhere
	hrows, err := d.SQL.Query(histSQL, histArgs...)
	if err != nil {
		return nil, err
	}
	defer hrows.Close()
	for hrows.Next() {
		var id, label string
		if err := hrows.Scan(&id, &label); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			// Prefer live label for active; keep frozen label only for missing ids.
			continue
		}
		seen[id] = InvoiceRevenueTerminalOption{
			ID:    id,
			Label: domain.FiscalTerminalDisplayName(label, ""),
		}
	}
	if err := hrows.Err(); err != nil {
		return nil, err
	}

	// Prefer frozen loopback label from history when present.
	var lbLabel string
	_ = d.SQL.QueryRow(`SELECT fiscal_terminal_label FROM invoices
		WHERE store_id=? AND fiscal_terminal_id=? AND IFNULL(fiscal_terminal_label,'')!=''
		ORDER BY created_at DESC LIMIT 1`, storeID, domain.LoopbackFiscalTerminalID).Scan(&lbLabel)
	if strings.TrimSpace(lbLabel) != "" {
		seen[domain.LoopbackFiscalTerminalID] = InvoiceRevenueTerminalOption{
			ID: domain.LoopbackFiscalTerminalID, Label: strings.TrimSpace(lbLabel),
		}
	}

	out := make([]InvoiceRevenueTerminalOption, 0, len(seen))
	// Stable-ish order: loopback first, then by label.
	if opt, ok := seen[domain.LoopbackFiscalTerminalID]; ok {
		out = append(out, opt)
		delete(seen, domain.LoopbackFiscalTerminalID)
	}
	rest := make([]InvoiceRevenueTerminalOption, 0, len(seen))
	for _, opt := range seen {
		rest = append(rest, opt)
	}
	for i := 0; i < len(rest); i++ {
		for j := i + 1; j < len(rest); j++ {
			if rest[j].Label < rest[i].Label {
				rest[i], rest[j] = rest[j], rest[i]
			}
		}
	}
	out = append(out, rest...)
	return out, nil
}
