package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// GetByRequestID returns the issued document for a request_id.
func (d *DB) GetByRequestID(storeID, requestID string) (*IssueRecord, error) {
	var invID string
	err := d.SQL.QueryRow(`SELECT invoice_id FROM idempotency_keys WHERE store_id = ? AND request_id = ?`,
		storeID, requestID).Scan(&invID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return d.GetIssueRecordByID(invID)
}

// GetIssueRecordByID loads a committed issue by invoice id (draft workbench idempotent scope hit).
func (d *DB) GetIssueRecordByID(invoiceID string) (*IssueRecord, error) {
	tx, err := d.SQL.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rec, err := loadIssueRecord(tx, invoiceID)
	if err != nil {
		return nil, err
	}
	_ = tx.Commit()
	return rec, nil
}

// PrintJobView is a read model for GET /local/v1/print-jobs/{id}.
type PrintJobView struct {
	ID           string `json:"id"`
	InvoiceID    string `json:"invoice_id"`
	DocumentType string `json:"document_type"`
	PrintPurpose string `json:"print_purpose"`
	JobStatus    string `json:"job_status"`
	Attempts     int    `json:"attempts"`
	LastError    string `json:"last_error,omitempty"`
	CreatedAt    string `json:"created_at"`
	PrintedAt    string `json:"printed_at,omitempty"`
}

// GetPrintJob loads a print job by id.
func (d *DB) GetPrintJob(id string) (*PrintJobView, error) {
	var v PrintJobView
	var lastErr, printed sql.NullString
	err := d.SQL.QueryRow(`SELECT id, invoice_id, document_type, print_purpose, job_status, attempts,
		last_error, created_at, printed_at FROM local_print_jobs WHERE id = ?`, id).
		Scan(&v.ID, &v.InvoiceID, &v.DocumentType, &v.PrintPurpose, &v.JobStatus, &v.Attempts,
			&lastErr, &v.CreatedAt, &printed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if lastErr.Valid {
		v.LastError = lastErr.String
	}
	if printed.Valid {
		v.PrintedAt = printed.String
	}
	return &v, nil
}

// ClaimNextPrintJob claims one PENDING job (FOR UPDATE style via status flip).
func (d *DB) ClaimNextPrintJob() (jobID string, payloadJSON []byte, err error) {
	tx, err := d.SQL.Begin()
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id string
	var payload string
	err = tx.QueryRow(`SELECT id, payload_json FROM local_print_jobs
		WHERE job_status = 'PENDING' ORDER BY created_at LIMIT 1`).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Commit()
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.Exec(`UPDATE local_print_jobs SET job_status = 'PROCESSING', attempts = attempts + 1, updated_at = ?
		WHERE id = ? AND job_status = 'PENDING'`, now, id)
	if err != nil {
		return "", nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_ = tx.Commit()
		return "", nil, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return id, []byte(payload), nil
}

// CompletePrintJob marks success or failure and mirrors invoice.print_status.
func (d *DB) CompletePrintJob(jobID string, ok bool, errMsg string) error {
	tx, err := d.SQL.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var invoiceID string
	if err := tx.QueryRow(`SELECT invoice_id FROM local_print_jobs WHERE id = ?`, jobID).Scan(&invoiceID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	attemptID := jobID + "-a-" + now
	if ok {
		if _, err := tx.Exec(`UPDATE local_print_jobs SET job_status = 'PRINTED', printed_at = ?, updated_at = ?, last_error = NULL WHERE id = ?`,
			now, now, jobID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE invoices SET print_status = ? WHERE id = ?`, string(domain.PrintPrinted), invoiceID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO print_attempts (id, print_job_id, attempted_at, result, error_code, error_message, device_hint)
			VALUES (?, ?, ?, 'OK', NULL, NULL, 'fiscal-local')`, attemptID, jobID, now); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`UPDATE local_print_jobs SET job_status = 'FAILED', last_error = ?, updated_at = ? WHERE id = ?`,
			errMsg, now, jobID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE invoices SET print_status = ? WHERE id = ?`, string(domain.PrintFailed), invoiceID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO print_attempts (id, print_job_id, attempted_at, result, error_code, error_message, device_hint)
			VALUES (?, ?, ?, 'FAILED', 'PRINT', ?, 'fiscal-local')`, attemptID, jobID, now, errMsg); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DecodePayloadJSON is a tiny helper for workers.
func DecodePayloadJSON(b []byte, dest any) error {
	return json.Unmarshal(b, dest)
}
