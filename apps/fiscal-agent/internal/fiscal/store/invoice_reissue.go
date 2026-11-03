package store

import (
	"database/sql"
	"errors"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/vatpercent"
)

// InvoiceItemLine is one stored line in the manual-issue line shape (catalog.ManualLineInput
// JSON), so issuing an FT from a PF / GR sends the original quantity, price and IVA unchanged.
// Lines go as temp lines (no product_code): a catalog code would re-price from the current catalog.
type InvoiceItemLine struct {
	DisplayName    string `json:"display_name"`
	SaftName       string `json:"saft_name"`
	UnitPriceGross string `json:"unit_price_gross"`
	VATRatePercent string `json:"vat_rate_percent"`
	Quantity       string `json:"quantity"`
}

// LoadInvoiceItemLines is the ONLY reader of invoice_lines as re-issuable sale lines.
func (d *DB) LoadInvoiceItemLines(invoiceID string) ([]InvoiceItemLine, error) {
	rows, err := d.SQL.Query(`SELECT COALESCE(NULLIF(display_name,''), product_description), product_description,
			unit_price_gross, vat_rate, quantity
		FROM invoice_lines WHERE invoice_id = ? ORDER BY line_number`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InvoiceItemLine{}
	for rows.Next() {
		var ln InvoiceItemLine
		var rate string
		if err := rows.Scan(&ln.DisplayName, &ln.SaftName, &ln.UnitPriceGross, &rate, &ln.Quantity); err != nil {
			return nil, err
		}
		if ln.VATRatePercent, err = vatpercent.FromDecimal(rate); err != nil {
			return nil, err
		}
		out = append(out, ln)
	}
	return out, rows.Err()
}

// LoadInvoiceCustomer is the ONLY reader of an issued document's buyer snapshot.
// nil when the document has no snapshot row.
func (d *DB) LoadInvoiceCustomer(invoiceID string) (*domain.CustomerInput, error) {
	var c domain.CustomerInput
	err := d.SQL.QueryRow(`SELECT customer_tax_id, company_name, address_detail, city, postal_code, country
		FROM invoice_customer_snapshots WHERE invoice_id = ?`, invoiceID).
		Scan(&c.TaxID, &c.CompanyName, &c.AddressDetail, &c.City, &c.PostalCode, &c.Country)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
