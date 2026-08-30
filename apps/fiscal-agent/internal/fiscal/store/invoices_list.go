package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// InvoiceListItem is a row for GET /local/v1/fiscal-documents.
// List columns (Admin): clerk fields; technical fields in detail drawer only.
type InvoiceListItem struct {
	DocumentID      string `json:"document_id"`
	InvoiceNo       string `json:"invoice_no"`
	ATCUD           string `json:"atcud"`
	DocumentType    string `json:"document_type"`
	DocumentStatus  string `json:"document_status"`
	PrintStatus     string `json:"print_status"`
	GrossTotal      string `json:"gross_total"`
	SystemEntryDate string `json:"system_entry_date"`
	IssuedAt        string `json:"issued_at"` // same as system_entry_date for home "today" filter
	Hash            string `json:"hash"`
	PreviousHash    string `json:"previous_hash"`
	CustomerTaxID   string `json:"customer_tax_id,omitempty"`
	CustomerName    string `json:"customer_name,omitempty"`
	SourceSaleID    string `json:"source_sale_id,omitempty"`
	OrderLabel      string `json:"order_label,omitempty"`
}

// InvoiceListQuery filters GET /local/v1/fiscal-documents (invoice_date + search + pagination).
type InvoiceListQuery struct {
	StoreID  string
	Page     int
	PageSize int
	From     string // invoice_date YYYY-MM-DD inclusive
	To       string // invoice_date YYYY-MM-DD inclusive
	Q        string // invoice_no, customer, source
}

// InvoiceListResult is the ONLY paginated invoice list payload from store.
type InvoiceListResult struct {
	Items         []InvoiceListItem
	Total         int
	Page          int
	PageSize      int
	GrossTotalSum string
}

// InvoiceDetail extends IssueRecord with totals for GET /local/v1/fiscal-documents/{id}.
type InvoiceDetail struct {
	IssueRecord
	GrossTotal          string                `json:"gross_total"`
	NetTotal            string                `json:"net_total"`
	TaxPayable          string                `json:"tax_payable"`
	SourceSaleID        string                `json:"source_sale_id,omitempty"`
	OrderLabel          string                `json:"order_label,omitempty"`
	CreditedGrossTotal  string                `json:"credited_gross_total,omitempty"`
	RemainingGrossTotal string                `json:"remaining_gross_total,omitempty"`
	Lines               []CreditLineRemaining `json:"lines,omitempty"`
}

var allowedInvoicePageSizes = map[int]bool{10: true, 20: true}

func normalizeInvoiceListQuery(q InvoiceListQuery) (InvoiceListQuery, int, int) {
	storeID := q.StoreID
	if storeID == "" {
		storeID = "store-demo-001"
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if !allowedInvoicePageSizes[pageSize] {
		pageSize = 10
	}
	return InvoiceListQuery{
		StoreID:  storeID,
		Page:     page,
		PageSize: pageSize,
		From:     strings.TrimSpace(q.From),
		To:       strings.TrimSpace(q.To),
		Q:        strings.TrimSpace(q.Q),
	}, page, pageSize
}

func invoiceListWhere(q InvoiceListQuery) (string, []any) {
	query := `
		FROM invoices i
		LEFT JOIN invoice_customer_snapshots cs ON cs.invoice_id = i.id
		WHERE i.store_id = ?`
	args := []any{q.StoreID}
	if q.From != "" {
		query += ` AND i.invoice_date >= ?`
		args = append(args, q.From)
	}
	if q.To != "" {
		query += ` AND i.invoice_date <= ?`
		args = append(args, q.To)
	}
	if q.Q != "" {
		like := "%" + escapeLike(q.Q) + "%"
		query += ` AND (i.invoice_no LIKE ? ESCAPE '\'
			OR cs.customer_tax_id LIKE ? ESCAPE '\'
			OR cs.company_name LIKE ? ESCAPE '\'
			OR IFNULL(i.display_meta_json,'') LIKE ? ESCAPE '\'
			OR IFNULL(i.source_sale_id,'') LIKE ? ESCAPE '\')`
		args = append(args, like, like, like, like, like)
	}
	return query, args
}

// ListInvoices returns invoices newest first with server-side pagination.
// ONLY list reader for Admin invoice table (joins customer snapshot once here).
func (d *DB) ListInvoices(q InvoiceListQuery) (*InvoiceListResult, error) {
	q, page, pageSize := normalizeInvoiceListQuery(q)
	where, args := invoiceListWhere(q)

	var total int
	var grossSum float64
	countQuery := `SELECT COUNT(*), COALESCE(SUM(CAST(i.gross_total AS REAL)), 0)` + where
	if err := d.SQL.QueryRow(countQuery, args...).Scan(&total, &grossSum); err != nil {
		return nil, err
	}

	totalPages := int(math.Max(1, math.Ceil(float64(total)/float64(pageSize))))
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	selectQuery := `
		SELECT i.id, i.invoice_no, i.atcud, i.document_type, i.document_status, i.print_status,
			i.gross_total, i.system_entry_date, i.hash, IFNULL(i.previous_hash,''),
			IFNULL(cs.customer_tax_id,''), IFNULL(cs.company_name,''),
			IFNULL(i.source_sale_id,''), IFNULL(i.display_meta_json,'')` + where +
		` ORDER BY i.created_at DESC LIMIT ? OFFSET ?`
	selectArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := d.SQL.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []InvoiceListItem{}
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
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &InvoiceListResult{
		Items:         items,
		Total:         total,
		Page:          page,
		PageSize:      pageSize,
		GrossTotalSum: fmt.Sprintf("%.2f", grossSum),
	}, nil
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

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
