package store

// SAFTLine is one invoice line for SAF-T export.
type SAFTLine struct {
	LineNumber               int
	ProductCode              string
	ProductDescription       string
	Quantity                 string
	UnitOfMeasure            string
	UnitPriceNet             string
	LineNet                  string
	LineTax                  string
	LineGross                string
	VATRate                  string
	TaxType                  string
	TaxCountryRegion         string
	TaxCode                  string
	TaxExemptionCode         string
	TaxExemptionReason       string
	ProductType              string
	ReferenceOriginalInvoice string
	ReferenceReason          string
}

// SAFTCustomer is customer snapshot for SAF-T.
type SAFTCustomer struct {
	CustomerTaxID       string
	CompanyName         string
	AddressDetail       string
	City                string
	PostalCode          string
	Country             string
	AccountID           string
	SelfBillingIndicator int
}

// SAFTInvoice is one signed document for SAF-T export.
type SAFTInvoice struct {
	ID                        string
	DocumentType              string
	InvoiceNo                 string
	ATCUD                     string
	InvoiceDate               string
	SystemEntryDate           string
	Hash                      string
	PreviousHash              string
	HashControl               int
	GrossTotal                string
	NetTotal                  string
	TaxPayable                string
	SourceID                  string
	SoftwareCertificateNumber string
	Customer                  SAFTCustomer
	Lines                     []SAFTLine
}

// LoadSAFTInvoicesForPeriod is the ONLY reader for SAF-T source documents in a month.
func (d *DB) LoadSAFTInvoicesForPeriod(storeID, startDate, endDate string) ([]SAFTInvoice, error) {
	rows, err := d.SQL.Query(`SELECT i.id, i.document_type, i.invoice_no, i.atcud, i.invoice_date, i.system_entry_date,
		i.hash, IFNULL(i.previous_hash,''), i.hash_control, i.gross_total, i.net_total, i.tax_payable,
		i.source_id, i.software_certificate_number,
		cs.customer_tax_id, cs.company_name, cs.address_detail, cs.city, cs.postal_code, cs.country,
		cs.account_id, cs.self_billing_indicator
		FROM invoices i
		JOIN invoice_customer_snapshots cs ON cs.invoice_id = i.id
		WHERE i.store_id = ? AND i.invoice_date >= ? AND i.invoice_date <= ?
		AND i.document_type IN ('FT','NC','FS','FR')
		ORDER BY i.invoice_date, i.created_at`, storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SAFTInvoice
	var ids []string
	for rows.Next() {
		var inv SAFTInvoice
		if err := rows.Scan(
			&inv.ID, &inv.DocumentType, &inv.InvoiceNo, &inv.ATCUD, &inv.InvoiceDate, &inv.SystemEntryDate,
			&inv.Hash, &inv.PreviousHash, &inv.HashControl, &inv.GrossTotal, &inv.NetTotal, &inv.TaxPayable,
			&inv.SourceID, &inv.SoftwareCertificateNumber,
			&inv.Customer.CustomerTaxID, &inv.Customer.CompanyName, &inv.Customer.AddressDetail,
			&inv.Customer.City, &inv.Customer.PostalCode, &inv.Customer.Country,
			&inv.Customer.AccountID, &inv.Customer.SelfBillingIndicator,
		); err != nil {
			return nil, err
		}
		out = append(out, inv)
		ids = append(ids, inv.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for i, invoiceID := range ids {
		lines, err := d.loadSAFTLines(invoiceID)
		if err != nil {
			return nil, err
		}
		if out[i].DocumentType == "NC" {
			refs, err := d.loadNCReferences(invoiceID)
			if err != nil {
				return nil, err
			}
			for j := range lines {
				if ref, ok := refs[lines[j].LineNumber]; ok {
					lines[j].ReferenceOriginalInvoice = ref.OrigNo
					lines[j].ReferenceReason = ref.Reason
				}
			}
		}
		out[i].Lines = lines
	}
	return out, nil
}

func (d *DB) loadSAFTLines(invoiceID string) ([]SAFTLine, error) {
	rows, err := d.SQL.Query(`SELECT il.line_number, il.product_code, il.product_description, il.quantity,
		il.unit_of_measure, il.unit_price_net, il.line_net, il.line_tax, il.line_gross,
		il.vat_rate, il.tax_type, il.tax_country_region, il.tax_code,
		COALESCE(il.tax_exemption_code,''), COALESCE(il.tax_exemption_reason,''), il.product_type
		FROM invoice_lines il WHERE il.invoice_id = ? ORDER BY il.line_number`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []SAFTLine
	for rows.Next() {
		var ln SAFTLine
		if err := rows.Scan(
			&ln.LineNumber, &ln.ProductCode, &ln.ProductDescription, &ln.Quantity,
			&ln.UnitOfMeasure, &ln.UnitPriceNet, &ln.LineNet, &ln.LineTax, &ln.LineGross,
			&ln.VATRate, &ln.TaxType, &ln.TaxCountryRegion, &ln.TaxCode,
			&ln.TaxExemptionCode, &ln.TaxExemptionReason, &ln.ProductType,
		); err != nil {
			return nil, err
		}
		lines = append(lines, ln)
	}
	return lines, rows.Err()
}

type ncLineRef struct {
	OrigNo string
	Reason string
}

func (d *DB) loadNCReferences(invoiceID string) (map[int]ncLineRef, error) {
	rows, err := d.SQL.Query(`SELECT il.line_number, r.original_invoice_no, r.reason
		FROM invoice_line_references r
		JOIN invoice_lines il ON il.id = r.credit_line_id
		WHERE il.invoice_id = ?`, invoiceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]ncLineRef{}
	for rows.Next() {
		var ln int
		var ref ncLineRef
		if err := rows.Scan(&ln, &ref.OrigNo, &ref.Reason); err != nil {
			return nil, err
		}
		out[ln] = ref
	}
	return out, rows.Err()
}

// SAFTExportRow is one saft_exports record.
type SAFTExportRow struct {
	ID               string `json:"id"`
	StoreID          string `json:"store_id"`
	TaxpayerNIF      string `json:"taxpayer_nif"`
	PeriodYear       int    `json:"period_year"`
	PeriodMonth      int    `json:"period_month"`
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	FileName         string `json:"file_name"`
	FilePath         string `json:"file_path,omitempty"`
	FileSHA256       string `json:"file_sha256,omitempty"`
	InvoiceCount     int    `json:"invoice_count"`
	TotalNet         string `json:"total_net,omitempty"`
	TotalTax         string `json:"total_tax,omitempty"`
	TotalGross       string `json:"total_gross,omitempty"`
	ValidationStatus string `json:"validation_status"`
	ValidationErrors string `json:"validation_errors,omitempty"`
	CreatedBy        string `json:"created_by,omitempty"`
	CreatedAt        string `json:"created_at"`
}
