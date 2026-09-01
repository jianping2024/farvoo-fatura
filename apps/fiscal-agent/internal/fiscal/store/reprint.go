package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"

	"github.com/google/uuid"
)

// ErrReprintNotAllowed is returned when document_status blocks REPRINT (see domain.IsReprintableDocumentStatus).
var ErrReprintNotAllowed = errors.New("store: reprint not allowed")

// ErrReprintOriginalMissing is returned when no ORIGINAL local_print_jobs row exists for the invoice.
var ErrReprintOriginalMissing = errors.New("store: original print job not found")

// ReprintResult is the outcome of CreateReprintPrintJob.
type ReprintResult struct {
	PrintJobID  string
	InvoiceID   string
	PrintStatus string
}

// CreateReprintPrintJob clones ORIGINAL payload_json into a new REPRINT job — ONLY reprint write path.
func (d *DB) CreateReprintPrintJob(invoiceID, operatorID, stationID string) (*ReprintResult, error) {
	if invoiceID == "" {
		return nil, fmt.Errorf("store: invoice id required")
	}
	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var docStatus string
	err = tx.QueryRow(`SELECT document_status FROM invoices WHERE id = ?`, invoiceID).Scan(&docStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !domain.IsReprintableDocumentStatus(docStatus) {
		return nil, ErrReprintNotAllowed
	}

	var origPayload string
	var docType string
	err = tx.QueryRow(`SELECT payload_json, document_type FROM local_print_jobs
		WHERE invoice_id = ? AND print_purpose = 'ORIGINAL' ORDER BY created_at LIMIT 1`, invoiceID).
		Scan(&origPayload, &docType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReprintOriginalMissing
	}
	if err != nil {
		return nil, err
	}

	payloadJSON, payloadHash, err := fiscalprint.ClonePayloadForReprint([]byte(origPayload))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	jobID := uuid.NewString()
	_, err = tx.Exec(`INSERT INTO local_print_jobs (
		id, invoice_id, document_type, print_purpose, job_status, logical_role,
		payload_json, payload_hash, attempts, last_error, created_at, updated_at, printed_at, created_by, station_id
	) VALUES (?, ?, ?, 'REPRINT', 'PENDING', 'fiscal_receipt_printer', ?, ?, 0, NULL, ?, ?, NULL, ?, ?)`,
		jobID, invoiceID, docType, string(payloadJSON), payloadHash, now, now, nullStr(operatorID), nullStr(stationID))
	if err != nil {
		return nil, fmt.Errorf("store: insert reprint job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rec, err := d.GetIssueRecordByID(invoiceID)
	if err != nil {
		return &ReprintResult{PrintJobID: jobID, InvoiceID: invoiceID}, nil
	}
	return &ReprintResult{
		PrintJobID:  jobID,
		InvoiceID:   invoiceID,
		PrintStatus: string(rec.PrintStatus),
	}, nil
}
