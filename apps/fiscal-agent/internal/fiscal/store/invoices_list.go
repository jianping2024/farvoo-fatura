package store

import (
	"encoding/json"
	"errors"
)

// InvoiceListItem is a row for GET /local/v1/fiscal-documents.
// List columns (Admin): Hash inputs (minus invoice_date) + hash + ops fields; print_status kept in JSON only.
type InvoiceListItem struct {
	DocumentID       string `json:"document_id"`
	InvoiceNo        string `json:"invoice_no"`
	ATCUD            string `json:"atcud"`
	DocumentType     string `json:"document_type"`
	DocumentStatus   string `json:"document_status"`
	PrintStatus      string `json:"print_status"`
	GrossTotal       string `json:"gross_total"`
	SystemEntryDate  string `json:"system_entry_date"`
	IssuedAt         string `json:"issued_at"` // same as system_entry_date for home "today" filter
	Hash             string `json:"hash"`
	PreviousHash     string `json:"previous_hash"`
	CustomerTaxID    string `json:"customer_tax_id,omitempty"`
	CustomerName     string `json:"customer_name,omitempty"`
	SourceSaleID     string `json:"source_sale_id,omitempty"`
	OrderLabel       string `json:"order_label,omitempty"`
}

// InvoiceDetail extends IssueRecord with totals for GET /local/v1/fiscal-documents/{id}.
type InvoiceDetail struct {
	IssueRecord
	GrossTotal   string `json:"gross_total"`
	NetTotal     string `json:"net_total"`
	TaxPayable   string `json:"tax_payable"`
	SourceSaleID string `json:"source_sale_id,omitempty"`
	OrderLabel   string `json:"order_label,omitempty"`
}

// ListInvoices returns recent invoices newest first.
// ONLY list reader for Admin invoice table (joins customer snapshot once here).
func (d *DB) ListInvoices(storeID string, limit int) ([]InvoiceListItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if storeID == "" {
		storeID = "store-demo-001"
	}
	rows, err := d.SQL.Query(`
		SELECT i.id, i.invoice_no, i.atcud, i.document_type, i.document_status, i.print_status,
			i.gross_total, i.system_entry_date, i.hash, IFNULL(i.previous_hash,''),
			IFNULL(cs.customer_tax_id,''), IFNULL(cs.company_name,''),
			IFNULL(i.source_sale_id,''), IFNULL(i.display_meta_json,'')
		FROM invoices i
		LEFT JOIN invoice_customer_snapshots cs ON cs.invoice_id = i.id
		WHERE i.store_id = ?
		ORDER BY i.created_at DESC
		LIMIT ?`, storeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InvoiceListItem
	for rows.Next() {
		var it InvoiceListItem
		var displayMeta string
		if err := rows.Scan(
			&it.DocumentID, &it.InvoiceNo, &it.ATCUD, &it.DocumentType, &it.DocumentStatus, &it.PrintStatus,
			&it.GrossTotal, &it.SystemEntryDate, &it.Hash, &it.PreviousHash,
			&it.CustomerTaxID, &it.CustomerName,
			&it.SourceSaleID, &displayMeta,
		); err != nil {
			return nil, err
		}
		it.IssuedAt = it.SystemEntryDate
		it.OrderLabel = orderLabelFromMeta(it.SourceSaleID, displayMeta)
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetInvoiceDetail loads one invoice by id.
func (d *DB) GetInvoiceDetail(invoiceID string) (*InvoiceDetail, error) {
	rec, err := d.GetIssueRecordByID(invoiceID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var gross, net, tax, sourceID, displayMeta string
	err = d.SQL.QueryRow(`SELECT gross_total, net_total, tax_payable, IFNULL(source_sale_id,''), IFNULL(display_meta_json,'')
		FROM invoices WHERE id = ?`, invoiceID).
		Scan(&gross, &net, &tax, &sourceID, &displayMeta)
	if err != nil {
		return nil, err
	}
	return &InvoiceDetail{
		IssueRecord:  *rec,
		GrossTotal:   gross,
		NetTotal:     net,
		TaxPayable:   tax,
		SourceSaleID: sourceID,
		OrderLabel:   orderLabelFromMeta(sourceID, displayMeta),
	}, nil
}

func orderLabelFromMeta(sourceSaleID, displayMetaJSON string) string {
	var meta struct {
		TableDisplayName string `json:"table_display_name"`
		SplitName        string `json:"split_name"`
	}
	if displayMetaJSON != "" && json.Unmarshal([]byte(displayMetaJSON), &meta) == nil {
		if meta.TableDisplayName != "" {
			label := "桌 " + meta.TableDisplayName
			if meta.SplitName != "" {
				label += " · " + meta.SplitName
			}
			return label
		}
	}
	if sourceSaleID != "" {
		suffix := sourceSaleID
		if len(suffix) > 8 {
			suffix = suffix[len(suffix)-8:]
		}
		return "sale " + suffix
	}
	return ""
}
