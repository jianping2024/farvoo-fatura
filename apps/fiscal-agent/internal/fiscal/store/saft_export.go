package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// InsertSAFTExportInput is the ONLY write payload for saft_exports.
type InsertSAFTExportInput struct {
	StoreID          string
	TaxpayerNIF      string
	PeriodYear       int
	PeriodMonth      int
	StartDate        string
	EndDate          string
	FileName         string
	FilePath         string
	FileSHA256       string
	InvoiceCount     int
	TotalNet         string
	TotalTax         string
	TotalGross       string
	ValidationStatus string
	ValidationErrors string
	CreatedBy        string
}

// InsertSAFTExport appends one saft_exports row (never updates prior exports).
func (d *DB) InsertSAFTExport(in InsertSAFTExportInput) (*SAFTExportRow, error) {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.SQL.Exec(`INSERT INTO saft_exports (
		id, store_id, taxpayer_nif, period_year, period_month, start_date, end_date,
		file_name, file_path, file_sha256, invoice_count, total_net, total_tax, total_gross,
		validation_status, validation_errors, created_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.StoreID, in.TaxpayerNIF, in.PeriodYear, in.PeriodMonth, in.StartDate, in.EndDate,
		in.FileName, in.FilePath, in.FileSHA256, in.InvoiceCount, in.TotalNet, in.TotalTax, in.TotalGross,
		in.ValidationStatus, nullStr(in.ValidationErrors), nullStr(in.CreatedBy), now)
	if err != nil {
		return nil, err
	}
	return &SAFTExportRow{
		ID: id, StoreID: in.StoreID, TaxpayerNIF: in.TaxpayerNIF,
		PeriodYear: in.PeriodYear, PeriodMonth: in.PeriodMonth,
		StartDate: in.StartDate, EndDate: in.EndDate,
		FileName: in.FileName, FilePath: in.FilePath, FileSHA256: in.FileSHA256,
		InvoiceCount: in.InvoiceCount, TotalNet: in.TotalNet, TotalTax: in.TotalTax, TotalGross: in.TotalGross,
		ValidationStatus: in.ValidationStatus, ValidationErrors: in.ValidationErrors,
		CreatedBy: in.CreatedBy, CreatedAt: now,
	}, nil
}

// ListSAFTExports lists export rows newest first.
func (d *DB) ListSAFTExports(storeID string, year, month int) ([]SAFTExportRow, error) {
	q := `SELECT id, store_id, taxpayer_nif, period_year, period_month, start_date, end_date,
		file_name, IFNULL(file_path,''), IFNULL(file_sha256,''), invoice_count,
		IFNULL(total_net,''), IFNULL(total_tax,''), IFNULL(total_gross,''),
		validation_status, IFNULL(validation_errors,''), IFNULL(created_by,''), created_at
		FROM saft_exports WHERE store_id = ?`
	args := []any{storeID}
	if year > 0 {
		q += ` AND period_year = ?`
		args = append(args, year)
	}
	if month > 0 {
		q += ` AND period_month = ?`
		args = append(args, month)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := d.SQL.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SAFTExportRow
	for rows.Next() {
		var r SAFTExportRow
		if err := rows.Scan(
			&r.ID, &r.StoreID, &r.TaxpayerNIF, &r.PeriodYear, &r.PeriodMonth, &r.StartDate, &r.EndDate,
			&r.FileName, &r.FilePath, &r.FileSHA256, &r.InvoiceCount,
			&r.TotalNet, &r.TotalTax, &r.TotalGross,
			&r.ValidationStatus, &r.ValidationErrors, &r.CreatedBy, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetSAFTExport loads one export row by id.
func (d *DB) GetSAFTExport(exportID string) (*SAFTExportRow, error) {
	var r SAFTExportRow
	err := d.SQL.QueryRow(`SELECT id, store_id, taxpayer_nif, period_year, period_month, start_date, end_date,
		file_name, IFNULL(file_path,''), IFNULL(file_sha256,''), invoice_count,
		IFNULL(total_net,''), IFNULL(total_tax,''), IFNULL(total_gross,''),
		validation_status, IFNULL(validation_errors,''), IFNULL(created_by,''), created_at
		FROM saft_exports WHERE id = ?`, exportID).
		Scan(
			&r.ID, &r.StoreID, &r.TaxpayerNIF, &r.PeriodYear, &r.PeriodMonth, &r.StartDate, &r.EndDate,
			&r.FileName, &r.FilePath, &r.FileSHA256, &r.InvoiceCount,
			&r.TotalNet, &r.TotalTax, &r.TotalGross,
			&r.ValidationStatus, &r.ValidationErrors, &r.CreatedBy, &r.CreatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
