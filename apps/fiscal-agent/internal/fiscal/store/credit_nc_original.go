package store

import (
	"database/sql"
	"errors"
)

// NCOriginalReference is the original invoice linked to an NC (detail read model).
type NCOriginalReference struct {
	OriginalInvoiceID string `json:"original_invoice_id"`
	OriginalInvoiceNo string `json:"original_invoice_no"`
	CreditReason      string `json:"credit_reason"`
}

// CreditOriginalForNC is the ONLY reader for NC → original invoice reference.
func (d *DB) CreditOriginalForNC(ncInvoiceID string) (*NCOriginalReference, error) {
	var ref NCOriginalReference
	err := d.SQL.QueryRow(`SELECT r.original_invoice_id, r.original_invoice_no, r.reason
		FROM invoice_line_references r
		JOIN invoice_lines il ON il.id = r.credit_line_id
		WHERE il.invoice_id = ?
		LIMIT 1`, ncInvoiceID).
		Scan(&ref.OriginalInvoiceID, &ref.OriginalInvoiceNo, &ref.CreditReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ref, nil
}
