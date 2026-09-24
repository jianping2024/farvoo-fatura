package store

// ReceiptOriginalReference is the FT linked to an RG (detail read model).
type ReceiptOriginalReference struct {
	OriginalInvoiceID string `json:"original_invoice_id"`
	OriginalInvoiceNo string `json:"original_invoice_no"`
	Amount            string `json:"amount"`
}

// ReceiptOriginalForDocument is the ONLY reader for RG → original FT reference.
func (d *DB) ReceiptOriginalForDocument(receiptInvoiceID string) (*ReceiptOriginalReference, error) {
	var ref ReceiptOriginalReference
	err := d.SQL.QueryRow(`SELECT original_invoice_id, original_invoice_no, amount
		FROM invoice_receipt_references WHERE receipt_invoice_id = ?`, receiptInvoiceID).
		Scan(&ref.OriginalInvoiceID, &ref.OriginalInvoiceNo, &ref.Amount)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}
