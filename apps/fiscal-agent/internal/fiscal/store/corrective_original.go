package store

import (
	"database/sql"
	"errors"
)

// CorrectiveOriginalReference is the original invoice linked to an NC or ND (detail read model).
type CorrectiveOriginalReference struct {
	OriginalInvoiceID string `json:"original_invoice_id"`
	OriginalInvoiceNo string `json:"original_invoice_no"`
	CreditReason      string `json:"credit_reason"` // NC/ND reason from invoice_line_references.reason
}

// CorrectiveOriginalForDocument is the ONLY reader for NC/ND → original invoice reference.
func (d *DB) CorrectiveOriginalForDocument(correctiveInvoiceID string) (*CorrectiveOriginalReference, error) {
	var ref CorrectiveOriginalReference
	err := d.SQL.QueryRow(`SELECT r.original_invoice_id, r.original_invoice_no, r.reason
		FROM invoice_line_references r
		JOIN invoice_lines il ON il.id = r.credit_line_id
		WHERE il.invoice_id = ?
		LIMIT 1`, correctiveInvoiceID).
		Scan(&ref.OriginalInvoiceID, &ref.OriginalInvoiceNo, &ref.CreditReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ref, nil
}
