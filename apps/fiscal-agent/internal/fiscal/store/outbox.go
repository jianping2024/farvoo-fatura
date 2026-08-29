package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	OutboxPending = "PENDING"
	OutboxSent    = "SENT"
	OutboxFailed  = "FAILED"

	EventInvoiceIssued = "INVOICE_ISSUED"
)

// InvoiceIssuedPayload is the cloud copy body (no private keys). ONLY shape enqueued for INVOICE_ISSUED.
type InvoiceIssuedPayload struct {
	EventType      string `json:"event_type"`
	DocumentID     string `json:"document_id"`
	InvoiceNo      string `json:"invoice_no"`
	ATCUD          string `json:"atcud"`
	DocumentStatus string `json:"document_status"`
	StoreID        string `json:"store_id"`
	SourceSystem   string `json:"source_system"`
	SourceSaleID   string `json:"source_sale_id"`
	ScopeType      string `json:"scope_type"`
	ScopeID        string `json:"scope_id"`
	FiscalPurpose  string `json:"fiscal_purpose"`
	GrossTotal     string `json:"gross_total"`
	IssuedAt       string `json:"issued_at"`
	PrintJobID     string `json:"print_job_id"`
	PrintStatus    string `json:"print_status"`
}

// EnqueueInvoiceIssuedTx inserts sync_outbox INVOICE_ISSUED in the IssueFT transaction.
// ONLY outbox write path for signed invoices — call from IssueFT before COMMIT.
func EnqueueInvoiceIssuedTx(tx *sql.Tx, storeID string, p InvoiceIssuedPayload, nowUTC time.Time) error {
	if tx == nil {
		return fmt.Errorf("store: outbox tx nil")
	}
	p.EventType = EventInvoiceIssued
	if p.DocumentStatus == "" {
		p.DocumentStatus = "SIGNED"
	}
	if p.PrintStatus == "" {
		p.PrintStatus = "PENDING"
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	now := nowUTC.UTC().Format(time.RFC3339)
	_, err = tx.Exec(`INSERT INTO sync_outbox (
		id, store_id, event_type, payload_json, status, attempts, next_attempt_at, last_error, created_at, sent_at
	) VALUES (?, ?, ?, ?, ?, 0, ?, NULL, ?, NULL)`,
		uuid.NewString(), storeID, EventInvoiceIssued, string(raw), OutboxPending, now, now)
	if err != nil {
		return fmt.Errorf("store: enqueue outbox: %w", err)
	}
	return nil
}

// OutboxRow is a claimed pending sync event.
type OutboxRow struct {
	ID          string
	StoreID     string
	EventType   string
	PayloadJSON string
	Attempts    int
}

// ClaimNextOutbox claims one due PENDING row — ONLY claim path for sync worker.
// Single UPDATE…RETURNING so concurrent claimers cannot double-deliver.
func (d *DB) ClaimNextOutbox(nowUTC time.Time) (*OutboxRow, error) {
	now := nowUTC.UTC().Format(time.RFC3339)
	var row OutboxRow
	err := d.SQL.QueryRow(`UPDATE sync_outbox SET attempts = attempts + 1, last_error = NULL
		WHERE id = (
			SELECT id FROM sync_outbox
			WHERE status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
			ORDER BY created_at ASC LIMIT 1
		)
		RETURNING id, store_id, event_type, payload_json, attempts`, OutboxPending, now).
		Scan(&row.ID, &row.StoreID, &row.EventType, &row.PayloadJSON, &row.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkOutboxSent is the ONLY success write for sync_outbox delivery.
func (d *DB) MarkOutboxSent(id string, sentAt time.Time) error {
	_, err := d.SQL.Exec(`UPDATE sync_outbox SET status = ?, sent_at = ?, last_error = NULL, next_attempt_at = NULL
		WHERE id = ?`, OutboxSent, sentAt.UTC().Format(time.RFC3339), id)
	return err
}

// MarkOutboxRetry keeps PENDING with backoff; after maxAttempts marks FAILED.
// ONLY failure write path for sync worker.
func (d *DB) MarkOutboxRetry(id string, attempts int, errMsg string, nextAt time.Time, maxAttempts int) error {
	if attempts >= maxAttempts {
		_, err := d.SQL.Exec(`UPDATE sync_outbox SET status = ?, last_error = ?, next_attempt_at = NULL
			WHERE id = ?`, OutboxFailed, errMsg, id)
		return err
	}
	_, err := d.SQL.Exec(`UPDATE sync_outbox SET status = ?, last_error = ?, next_attempt_at = ?
		WHERE id = ?`, OutboxPending, errMsg, nextAt.UTC().Format(time.RFC3339), id)
	return err
}

// CountOutboxByStatus is for regression/asserts.
func (d *DB) CountOutboxByStatus(storeID, status string) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM sync_outbox WHERE store_id = ? AND status = ?`, storeID, status).Scan(&n)
	return n, err
}
