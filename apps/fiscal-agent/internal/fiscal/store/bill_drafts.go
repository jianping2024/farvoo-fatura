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
const (
	BillDraftOpen      = "open"
	BillDraftInvoiced  = "invoiced"
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

// ErrAlreadyInvoiced means an open re-sync is refused because a draft is already invoiced.
var ErrAlreadyInvoiced = errors.New("already_invoiced")

// GetBillDraftBySale returns the latest draft row for a sale (any status), or ErrNotFound.
func (d *DB) GetBillDraftBySale(sourceSaleID string) (*BillSyncDraft, error) {
	row := d.SQL.QueryRow(`
		SELECT id, request_id, source_sale_id, payload_json, status, IFNULL(cloud_job_id,''), IFNULL(last_error,''), created_at, updated_at
		FROM bill_sync_drafts WHERE source_sale_id = ?
		ORDER BY CASE status WHEN 'invoiced' THEN 0 WHEN 'open' THEN 1 ELSE 2 END, updated_at DESC
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
// Same request_id → idempotent (no double write). Same source_sale_id with invoiced → ErrAlreadyInvoiced.
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

	if inv, err := d.GetBillDraftBySale(sourceSaleID); err == nil && inv.Status == BillDraftInvoiced {
		return nil, ErrAlreadyInvoiced
	} else if err != nil && !errors.Is(err, ErrNotFound) {
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

// MarkBillDraftInvoiced sets status invoiced for the open draft of a sale (ONLY after FT issue).
func (d *DB) MarkBillDraftInvoiced(sourceSaleID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE bill_sync_drafts SET status = ?, updated_at = ? WHERE source_sale_id = ? AND status = ?`,
		BillDraftInvoiced, now, sourceSaleID, BillDraftOpen)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
