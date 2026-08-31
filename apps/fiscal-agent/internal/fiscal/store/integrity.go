package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SeriesIntegrityRow is one series vs invoices expectation.
type SeriesIntegrityRow struct {
	SeriesID       string `json:"series_id"`
	SeriesCode     string `json:"series_code"`
	DocumentType   string `json:"document_type"`
	Status         string `json:"status"`
	LastNumber     int64  `json:"last_number"`
	LastHash       string `json:"last_hash"`
	ExpectedNumber int64  `json:"expected_number"`
	ExpectedHash   string `json:"expected_hash"`
	OK             bool   `json:"ok"`
	Blocked        bool   `json:"blocked,omitempty"`
	Healed         bool   `json:"healed,omitempty"`
}

// SeriesIntegrityReport is the verify result.
type SeriesIntegrityReport struct {
	OK      bool                 `json:"ok"`
	Checked int                  `json:"checked"`
	Failed  int                  `json:"failed"`
	Blocked int                  `json:"blocked"`
	Healed  int                  `json:"healed"`
	Series  []SeriesIntegrityRow `json:"series"`
}

// VerifySeriesIntegrityOptions controls side effects after compare.
type VerifySeriesIntegrityOptions struct {
	BlockOnFail bool // ACTIVE + mismatch → FAILED
	HealOnPass  bool // FAILED + match + validation_code → ACTIVE
	OperatorID  string
}

type seriesScanRow struct {
	id, code, docType, status, lastHash, validation string
	lastNumber                                      int64
}

// VerifySeriesIntegrity is the ONLY series last_number/last_hash audit path (D6.3).
// Loads series first then queries invoices (MaxOpenConns=1 forbids nested queries on open rows).
func (d *DB) VerifySeriesIntegrity(opts VerifySeriesIntegrityOptions) (*SeriesIntegrityReport, error) {
	rows, err := d.SQL.Query(`
		SELECT id, series_code, document_type, status, last_number, IFNULL(last_hash,''), IFNULL(validation_code,'')
		FROM series ORDER BY document_type, series_code`)
	if err != nil {
		return nil, err
	}
	var list []seriesScanRow
	for rows.Next() {
		var s seriesScanRow
		if err := rows.Scan(&s.id, &s.code, &s.docType, &s.status, &s.lastNumber, &s.lastHash, &s.validation); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rep := &SeriesIntegrityReport{OK: true}
	for _, s := range list {
		r := SeriesIntegrityRow{
			SeriesID: s.id, SeriesCode: s.code, DocumentType: s.docType,
			Status: s.status, LastNumber: s.lastNumber, LastHash: s.lastHash,
		}
		expN, expH, err := d.expectedSeriesTip(s.id)
		if err != nil {
			return nil, err
		}
		r.ExpectedNumber = expN
		r.ExpectedHash = expH
		r.OK = r.LastNumber == expN && r.LastHash == expH
		if !r.OK {
			rep.OK = false
			rep.Failed++
			if opts.BlockOnFail && r.Status == "ACTIVE" {
				if err := d.setSeriesStatus(r.SeriesID, "FAILED"); err != nil {
					return nil, err
				}
				r.Status = "FAILED"
				r.Blocked = true
				rep.Blocked++
				detail, _ := json.Marshal(map[string]any{
					"series_code": r.SeriesCode, "last_number": r.LastNumber, "expected_number": expN,
					"last_hash_len": len(r.LastHash), "expected_hash_len": len(expH),
				})
				_ = d.InsertAuditLog(opts.OperatorID, "series_integrity_failed", "series", r.SeriesID, string(detail))
			}
		} else if opts.HealOnPass && r.Status == "FAILED" && strings.TrimSpace(s.validation) != "" {
			if err := d.setSeriesStatus(r.SeriesID, "ACTIVE"); err != nil {
				return nil, err
			}
			r.Status = "ACTIVE"
			r.Healed = true
			rep.Healed++
			_ = d.InsertAuditLog(opts.OperatorID, "series_integrity_healed", "series", r.SeriesID,
				fmt.Sprintf(`{"series_code":%q}`, r.SeriesCode))
		}
		rep.Checked++
		rep.Series = append(rep.Series, r)
	}
	return rep, nil
}

func (d *DB) expectedSeriesTip(seriesID string) (num int64, hash string, err error) {
	var max sql.NullInt64
	err = d.SQL.QueryRow(`SELECT MAX(sequence_number) FROM invoices WHERE series_id = ?`, seriesID).Scan(&max)
	if err != nil {
		return 0, "", err
	}
	if !max.Valid || max.Int64 == 0 {
		return 0, "", nil
	}
	num = max.Int64
	err = d.SQL.QueryRow(`SELECT hash FROM invoices WHERE series_id = ? AND sequence_number = ?`, seriesID, num).Scan(&hash)
	if err != nil {
		return 0, "", err
	}
	return num, hash, nil
}

func (d *DB) setSeriesStatus(seriesID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.SQL.Exec(`UPDATE series SET status = ?, updated_at = ? WHERE id = ?`, status, now, seriesID)
	return err
}
