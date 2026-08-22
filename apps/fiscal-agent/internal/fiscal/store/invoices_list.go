package store

import (
	"encoding/json"
	"errors"
)

// InvoiceListItem is a row for GET /local/v1/fiscal-documents.
type InvoiceListItem struct {
	DocumentID     string `json:"document_id"`
	InvoiceNo      string `json:"invoice_no"`
	ATCUD          string `json:"atcud"`
	DocumentType   string `json:"document_type"`
	DocumentStatus string `json:"document_status"`
	PrintStatus    string `json:"print_status"`
	GrossTotal     string `json:"gross_total"`
	IssuedAt       string `json:"issued_at"`
	SourceSaleID   string `json:"source_sale_id,omitempty"`
	OrderLabel     string `json:"order_label,omitempty"`
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
func (d *DB) ListInvoices(storeID string, limit int) ([]InvoiceListItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if storeID == "" {
		storeID = "store-demo-001"
	}
	rows, err := d.SQL.Query(`SELECT i.id, i.invoice_no, i.atcud, i.document_type, i.document_status, i.print_status,
		i.gross_total, i.created_at, IFNULL(i.source_id,''), IFNULL(i.display_meta_json,'')
		FROM invoices i WHERE i.store_id = ? ORDER BY i.created_at DESC LIMIT ?`, storeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InvoiceListItem
	for rows.Next() {
		var it InvoiceListItem
		var created string
		var sourceID, displayMeta string
		if err := rows.Scan(&it.DocumentID, &it.InvoiceNo, &it.ATCUD, &it.DocumentType, &it.DocumentStatus,
			&it.PrintStatus, &it.GrossTotal, &created, &sourceID, &displayMeta); err != nil {
			return nil, err
		}
		it.IssuedAt = created
		it.SourceSaleID = sourceID
		it.OrderLabel = orderLabelFromMeta(sourceID, displayMeta)
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
	err = d.SQL.QueryRow(`SELECT gross_total, net_total, tax_payable, IFNULL(source_id,''), IFNULL(display_meta_json,'')
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

func orderLabelFromMeta(sourceID, displayMetaJSON string) string {
	if sourceID != "" {
		suffix := sourceID
		if len(suffix) > 8 {
			suffix = suffix[len(suffix)-8:]
		}
		return "ORD-" + suffix
	}
	var meta struct {
		TableDisplayName string `json:"table_display_name"`
	}
	if displayMetaJSON != "" && json.Unmarshal([]byte(displayMetaJSON), &meta) == nil && meta.TableDisplayName != "" {
		return "桌 " + meta.TableDisplayName
	}
	return ""
}
