package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Bill draft status values (farvoo-fiscal-bill-sync-api).
// Runtime only open/discarded — invoiced rows are not kept; FT success hard-deletes.
const (
	BillDraftOpen      = "open"
	BillDraftDiscarded = "discarded"
)

// BillSyncDraft is a local copy of a Farvoo bill snapshot.
type BillSyncDraft struct {
	ID           string
	RequestID    string
	SourceSaleID string
	PayloadJSON  string
	Status       string
	CloudJobID   string
	LastError    string
	CreatedAt    string
	UpdatedAt    string
}

// ErrAlreadyInvoiced means re-sync is refused because a signed FT exists for the sale.
var ErrAlreadyInvoiced = errors.New("already_invoiced")

// GetBillDraftByID loads one draft by primary key.
func (d *DB) GetBillDraftByID(id string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts WHERE id = ?`, id)
	return scanBillDraft(row)
}

// GetBillDraftBySale returns the latest draft row for a sale (any status), or ErrNotFound.
func (d *DB) GetBillDraftBySale(sourceSaleID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts WHERE source_sale_id = ?
		ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, updated_at DESC
		LIMIT 1`, sourceSaleID)
	return scanBillDraft(row)
}

// GetOpenBillDraftBySale returns open draft for sale or ErrNotFound.
func (d *DB) GetOpenBillDraftBySale(sourceSaleID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts WHERE source_sale_id = ? AND status = ? LIMIT 1`,
		sourceSaleID, BillDraftOpen)
	return scanBillDraft(row)
}

// GetBillDraftByRequestID looks up by request_id (idempotent replay).
func (d *DB) GetBillDraftByRequestID(requestID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts WHERE request_id = ? LIMIT 1`, requestID)
	return scanBillDraft(row)
}

func scanBillDraft(row *sql.Row) (*BillSyncDraft, error) {
	var b BillSyncDraft
	err := row.Scan(&b.ID, &b.RequestID, &b.SourceSaleID, &b.PayloadJSON, &b.Status, &b.CloudJobID, &b.LastError, &b.CreatedAt, &b.UpdatedAt)
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
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BillSyncDraft
	for rows.Next() {
		var b BillSyncDraft
		if err := rows.Scan(&b.ID, &b.RequestID, &b.SourceSaleID, &b.PayloadJSON, &b.Status, &b.CloudJobID, &b.LastError, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpsertBillDraftOpen is the ONLY writer that creates/covers open bill sync drafts.
// Same request_id → idempotent (no double write). Signed FT for sale → ErrAlreadyInvoiced.
// Open/discarded/missing → replace with new open payload.
func (d *DB) UpsertBillDraftOpen(requestID, sourceSaleID, cloudJobID string, payload any) (*BillSyncDraft, error) {
	requestID = strings.TrimSpace(requestID)
	sourceSaleID = strings.TrimSpace(sourceSaleID)
	if requestID == "" || sourceSaleID == "" {
		return nil, fmt.Errorf("store: request_id and source_sale_id required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
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
		INSERT INTO bill_sync_drafts(id, request_id, source_sale_id, payload_json, status, cloud_job_id, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		id, requestID, sourceSaleID, string(raw), BillDraftOpen, nullIfEmpty(cloudJobID), now, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetBillDraftByRequestID(requestID)
}

// DeleteBillDraftsBySale is the ONLY post-issue cleanup: hard-delete all drafts for the sale.
func (d *DB) DeleteBillDraftsBySale(sourceSaleID string) error {
	sourceSaleID = strings.TrimSpace(sourceSaleID)
	if sourceSaleID == "" {
		return fmt.Errorf("store: source_sale_id required")
	}
	_, err := d.SQL.Exec(`DELETE FROM bill_sync_drafts WHERE source_sale_id = ?`, sourceSaleID)
	return err
}

// SignedFTScope is one signed FT business scope for a Farvoo sale.
type SignedFTScope struct {
	DocumentID string `json:"document_id"`
	InvoiceNo  string `json:"invoice_no"`
	ScopeType  string `json:"scope_type"`
	ScopeID    string `json:"scope_id"`
}

// HasSignedFTForSale is the ONLY already_invoiced gate for bill-sync re-ingest (tax DB, not draft status).
func (d *DB) HasSignedFTForSale(storeID, sourceSystem, sourceSaleID string) (bool, error) {
	scopes, err := d.ListSignedFTScopesForSale(storeID, sourceSystem, sourceSaleID)
	if err != nil {
		return false, err
	}
	return len(scopes) > 0, nil
}

// ListSignedFTScopesForSale lists signed FTs for a sale (workbench issued-scope marks + mutex).
// ONLY tax-DB reader for "which scopes of this sale already have FT".
func (d *DB) ListSignedFTScopesForSale(storeID, sourceSystem, sourceSaleID string) ([]SignedFTScope, error) {
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
		SELECT id, invoice_no, IFNULL(scope_type,''), IFNULL(scope_id,'')
		FROM invoices
		WHERE source_sale_id = ? AND IFNULL(source_system,'') = ?
		  AND document_type = 'FT'
		  AND document_status IN ('SIGNED','CREDITED_PARTIAL','CREDITED_FULL')`
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
	var out []SignedFTScope
	for rows.Next() {
		var s SignedFTScope
		if err := rows.Scan(&s.DocumentID, &s.InvoiceNo, &s.ScopeType, &s.ScopeID); err != nil {
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
