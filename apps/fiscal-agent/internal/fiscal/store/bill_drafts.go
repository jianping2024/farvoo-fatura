package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
)

// Bill draft status values (farvoo-fiscal-bill-sync-api).
// Runtime only open/discarded — invoiced rows are not kept; FT success hard-deletes.
const (
	BillDraftOpen      = "open"
	BillDraftDiscarded = "discarded"
)

// BillSyncDraft is a local copy of a Farvoo bill snapshot + local allocation.
type BillSyncDraft struct {
	ID                 string `json:"id"`
	RequestID          string `json:"request_id"`
	SourceSaleID       string `json:"source_sale_id"`
	PayloadJSON        string `json:"payload_json"`
	AllocationJSON     string `json:"allocation_json"`
	AllocationRevision int64  `json:"allocation_revision"`
	Status             string `json:"status"`
	CloudJobID         string `json:"cloud_job_id"`
	LastError          string `json:"last_error"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// ErrAlreadyInvoiced means re-sync is refused because a signed FT exists for the sale.
var ErrAlreadyInvoiced = errors.New("already_invoiced")

// ErrAllocationConflict means OCC revision mismatch on SaveBillDraftAllocation.
var ErrAllocationConflict = errors.New("allocation_conflict")

const billDraftSelectCols = `id, request_id, source_sale_id, payload_json,
		IFNULL(allocation_json,'{}'), IFNULL(allocation_revision,0),
		status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at`

// GetBillDraftByID loads one draft by primary key.
func (d *DB) GetBillDraftByID(id string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`SELECT `+billDraftSelectCols+` FROM bill_sync_drafts WHERE id = ?`, id)
	return scanBillDraft(row)
}

// GetBillDraftBySale returns the latest draft row for a sale (any status), or ErrNotFound.
func (d *DB) GetBillDraftBySale(sourceSaleID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT `+billDraftSelectCols+` FROM bill_sync_drafts WHERE source_sale_id = ?
		ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, updated_at DESC
		LIMIT 1`, sourceSaleID)
	return scanBillDraft(row)
}

// GetOpenBillDraftBySale returns open draft for sale or ErrNotFound.
func (d *DB) GetOpenBillDraftBySale(sourceSaleID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT `+billDraftSelectCols+` FROM bill_sync_drafts WHERE source_sale_id = ? AND status = ? LIMIT 1`,
		sourceSaleID, BillDraftOpen)
	return scanBillDraft(row)
}

// GetBillDraftByRequestID looks up by request_id (idempotent replay).
func (d *DB) GetBillDraftByRequestID(requestID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT `+billDraftSelectCols+` FROM bill_sync_drafts WHERE request_id = ? LIMIT 1`, requestID)
	return scanBillDraft(row)
}

func scanBillDraft(row *sql.Row) (*BillSyncDraft, error) {
	var b BillSyncDraft
	err := row.Scan(&b.ID, &b.RequestID, &b.SourceSaleID, &b.PayloadJSON,
		&b.AllocationJSON, &b.AllocationRevision,
		&b.Status, &b.CloudJobID, &b.LastError, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBillDrafts returns recent drafts (newest first).
func (d *DB) ListBillDrafts(limit int) ([]BillSyncDraft, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.SQL.Query(`
		SELECT `+billDraftSelectCols+` FROM bill_sync_drafts ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BillSyncDraft
	for rows.Next() {
		var b BillSyncDraft
		if err := rows.Scan(&b.ID, &b.RequestID, &b.SourceSaleID, &b.PayloadJSON,
			&b.AllocationJSON, &b.AllocationRevision,
			&b.Status, &b.CloudJobID, &b.LastError, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CountOpenBillDrafts returns how many status=open drafts exist.
func (d *DB) CountOpenBillDrafts() (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(*) FROM bill_sync_drafts WHERE status = ?`, BillDraftOpen).Scan(&n)
	return n, err
}

func (d *DB) fireBillDraftsChanged(tableHint, kind string) {
	if d == nil || d.OnBillDraftsChanged == nil {
		return
	}
	n, err := d.CountOpenBillDrafts()
	if err != nil {
		n = 0
	}
	d.OnBillDraftsChanged(n, strings.TrimSpace(tableHint), kind)
}

func tableHintFromPayload(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["table_display_name"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// UpsertBillDraftOpen is the ONLY writer that creates/covers open bill sync drafts (payload + seed allocation).
// Same request_id → idempotent (no double write). Open/discarded/missing → replace with new open payload.
// allocationJSON seed + allocationRevision are set only on INSERT; later edits MUST use SaveBillDraftAllocation.
func (d *DB) UpsertBillDraftOpen(requestID, sourceSaleID, cloudJobID string, payload any, allocationJSON string, allocationRevision int64) (*BillSyncDraft, error) {
	requestID = strings.TrimSpace(requestID)
	sourceSaleID = strings.TrimSpace(sourceSaleID)
	if requestID == "" || sourceSaleID == "" {
		return nil, fmt.Errorf("store: request_id and source_sale_id required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	allocationJSON = strings.TrimSpace(allocationJSON)
	if allocationJSON == "" {
		allocationJSON = "{}"
	}
	if allocationRevision < 0 {
		allocationRevision = 0
	}

	if existing, err := d.GetBillDraftByRequestID(requestID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`UPDATE bill_sync_drafts SET status = ?, updated_at = ? WHERE source_sale_id = ? AND status = ?`,
		BillDraftDiscarded, now, sourceSaleID, BillDraftOpen); err != nil {
		return nil, err
	}

	id := uuid.NewString()
	if _, err := tx.Exec(`
		INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, allocation_json, allocation_revision, status, cloud_job_id, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		id, requestID, sourceSaleID, string(raw), allocationJSON, allocationRevision, BillDraftOpen, nullIfEmpty(cloudJobID), now, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	d.fireBillDraftsChanged(tableHintFromPayload(payload), "upsert")
	return d.GetBillDraftByRequestID(requestID)
}

// SaveBillDraftAllocation is the ONLY writer that updates allocation_json after draft create (OCC).
// expectedRevision must match current allocation_revision; on success revision becomes expected+1.
func (d *DB) SaveBillDraftAllocation(draftID string, expectedRevision int64, allocationJSON string) (*BillSyncDraft, error) {
	draftID = strings.TrimSpace(draftID)
	if draftID == "" {
		return nil, fmt.Errorf("store: draft id required")
	}
	allocationJSON = strings.TrimSpace(allocationJSON)
	if allocationJSON == "" {
		allocationJSON = "{}"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`
		UPDATE bill_sync_drafts
		SET allocation_json = ?, allocation_revision = ?, updated_at = ?
		WHERE id = ? AND status = ? AND allocation_revision = ?`,
		allocationJSON, expectedRevision+1, now, draftID, BillDraftOpen, expectedRevision)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		cur, err := d.GetBillDraftByID(draftID)
		if err != nil {
			return nil, err
		}
		if cur.Status != BillDraftOpen {
			return nil, fmt.Errorf("store: draft not open")
		}
		return nil, ErrAllocationConflict
	}
	return d.GetBillDraftByID(draftID)
}

// DeleteBillDraftsBySale is the ONLY post-issue cleanup: hard-delete all drafts for the sale.
func (d *DB) DeleteBillDraftsBySale(sourceSaleID string) error {
	sourceSaleID = strings.TrimSpace(sourceSaleID)
	if sourceSaleID == "" {
		return fmt.Errorf("store: source_sale_id required")
	}
	_, err := d.SQL.Exec(`DELETE FROM bill_sync_drafts WHERE source_sale_id = ?`, sourceSaleID)
	if err != nil {
		return err
	}
	d.fireBillDraftsChanged("", "delete")
	return nil
}

// SignedSaleScope is one signed FT/FS business scope for a Farvoo sale.
type SignedSaleScope struct {
	DocumentID   string `json:"document_id"`
	InvoiceNo    string `json:"invoice_no"`
	DocumentType string `json:"document_type"`
	ScopeType    string `json:"scope_type"`
	ScopeID      string `json:"scope_id"`
}

// HasSignedSaleForSale is the ONLY already_invoiced gate for bill-sync re-ingest (tax DB, not draft status).
func (d *DB) HasSignedSaleForSale(storeID, sourceSystem, sourceSaleID string) (bool, error) {
	scopes, err := d.ListSignedSaleScopesForSale(storeID, sourceSystem, sourceSaleID)
	if err != nil {
		return false, err
	}
	return len(scopes) > 0, nil
}

// ListSignedSaleScopesForSale lists signed FT/FS for a sale (workbench issued-scope marks + mutex).
func (d *DB) ListSignedSaleScopesForSale(storeID, sourceSystem, sourceSaleID string) ([]SignedSaleScope, error) {
	storeID = strings.TrimSpace(storeID)
	sourceSystem = strings.TrimSpace(sourceSystem)
	sourceSaleID = strings.TrimSpace(sourceSaleID)
	if sourceSaleID == "" {
		return nil, fmt.Errorf("store: source_sale_id required")
	}
	if sourceSystem == "" {
		sourceSystem = "farvoo"
	}
	q := `
		SELECT id, invoice_no, document_type, IFNULL(scope_type,''), IFNULL(scope_id,'')
		FROM invoices
		WHERE source_sale_id = ? AND IFNULL(source_system,'') = ?
		  AND document_type IN ('FT','FS')
		  AND document_status IN (` + domain.IssuedOriginalDocumentStatusSQLIn() + `)`
	args := []any{sourceSaleID, sourceSystem}
	if storeID != "" {
		q += ` AND store_id = ?`
		args = append(args, storeID)
	}
	rows, err := d.SQL.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SignedSaleScope
	for rows.Next() {
		var s SignedSaleScope
		if err := rows.Scan(&s.DocumentID, &s.InvoiceNo, &s.DocumentType, &s.ScopeType, &s.ScopeID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// InvoiceCustomerTaxID returns the customer snapshot tax id for an invoice (NIF checks).
func (d *DB) InvoiceCustomerTaxID(invoiceID string) (string, error) {
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return "", fmt.Errorf("store: invoice id required")
	}
	var tax string
	err := d.SQL.QueryRow(`SELECT customer_tax_id FROM invoice_customer_snapshots WHERE invoice_id = ?`, invoiceID).Scan(&tax)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return tax, err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
