package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SettleResult is returned after ApplySettlement.
type SettleResult struct {
	BusinessDate string
	DeletedCount int
	KeepTarget   string
	KeepActual   string
}

// ApplySettlement is the ONLY writer that deletes FS invoices for day settle
// and marks settlement in the exe-adjacent state file (not fiscal.db).
func ApplySettlement(ctx context.Context, db *sql.DB, statePath string, toolState *ToolState, dbPath string, plan *SettlePlan) (*SettleResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("nil plan")
	}
	if toolState == nil {
		return nil, fmt.Errorf("nil tool state")
	}
	norm, err := NormalizeDBPath(dbPath)
	if err != nil {
		return nil, err
	}
	if IsDateSettledInState(toolState, norm, plan.BusinessDate) {
		return nil, fmt.Errorf("营业日 %s 已结算", plan.BusinessDate)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var lastNum int64
	if err := tx.QueryRow(`SELECT last_number FROM series WHERE id=?`, plan.SeriesID).Scan(&lastNum); err != nil {
		return nil, err
	}
	var dayMax sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(sequence_number) FROM invoices WHERE series_id=? AND invoice_date=? AND document_type='FS'`,
		plan.SeriesID, plan.BusinessDate).Scan(&dayMax); err != nil {
		return nil, err
	}
	if !dayMax.Valid || dayMax.Int64 != lastNum {
		return nil, fmt.Errorf("结算时系列 tip 已变化，拒绝")
	}

	if len(plan.DeleteIDs) > 0 {
		if err := refuseIfOriginalReferencedOutsideTx(tx, plan.DeleteIDs); err != nil {
			return nil, err
		}
		if err := cascadeDeleteInvoicesTx(tx, plan.DeleteIDs); err != nil {
			return nil, err
		}
		newNum, newHash, err := tipAfterDeleteTx(tx, plan.SeriesID)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.Exec(`UPDATE series SET last_number=?, last_hash=?, updated_at=? WHERE id=?`,
			newNum, newHash, now, plan.SeriesID); err != nil {
			return nil, err
		}
	}

	detail, _ := json.Marshal(map[string]any{
		"keep_target": plan.KeepTarget, "keep_actual": plan.KeepActual,
		"shortfall": plan.Shortfall, "delete_ids": plan.DeleteIDs,
		"delete_nos": plan.DeleteInvoiceNos, "cutoff_seq": plan.CutoffSeq,
		"anchor_no": plan.AnchorInvoiceNo,
	})
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = tx.Exec(`INSERT INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
		VALUES (?,?,?,?,?,?,?)`,
		uuid.NewString(), now, "devtool", "dev_day_settle", "business_date", plan.BusinessDate, string(detail))

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if err := MarkSettledInState(toolState, SettlementEntry{
		BusinessDate: plan.BusinessDate,
		DBPath:       norm,
		SettledAt:    now,
		KeepTarget:   plan.KeepTarget,
		KeepActual:   plan.KeepActual,
		DeletedCount: len(plan.DeleteIDs),
		CutoffSeq:    plan.CutoffSeq,
		SeriesID:     plan.SeriesID,
		DetailJSON:   string(detail),
	}); err != nil {
		return nil, fmt.Errorf("票库已提交但结算标记失败: %w", err)
	}
	if err := SaveToolState(statePath, toolState); err != nil {
		return nil, fmt.Errorf("票库已提交但状态文件写入失败: %w", err)
	}

	return &SettleResult{
		BusinessDate: plan.BusinessDate,
		DeletedCount: len(plan.DeleteIDs),
		KeepTarget:   plan.KeepTarget,
		KeepActual:   plan.KeepActual,
	}, nil
}

func refuseIfOriginalReferencedOutsideTx(tx *sql.Tx, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	ph, args := placeholders(ids)
	q := fmt.Sprintf(`
		SELECT COUNT(1) FROM invoice_line_references r
		INNER JOIN invoice_lines l ON l.id = r.credit_line_id
		WHERE r.original_invoice_id IN (%s)
		  AND l.invoice_id NOT IN (%s)`, ph, ph)
	args = append(append([]any{}, args...), args...)
	var n int
	if err := tx.QueryRow(q, args...).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("拒绝删除：仍有 NC/ND 引用这些原票（%d）", n)
	}
	return nil
}

func cascadeDeleteInvoicesTx(tx *sql.Tx, ids []string) error {
	ph, args := placeholders(ids)
	steps := []string{
		fmt.Sprintf(`DELETE FROM print_attempts WHERE print_job_id IN (SELECT id FROM local_print_jobs WHERE invoice_id IN (%s))`, ph),
		fmt.Sprintf(`DELETE FROM local_print_jobs WHERE invoice_id IN (%s)`, ph),
		fmt.Sprintf(`DELETE FROM invoice_line_references WHERE credit_line_id IN (SELECT id FROM invoice_lines WHERE invoice_id IN (%s))`, ph),
		fmt.Sprintf(`DELETE FROM idempotency_keys WHERE invoice_id IN (%s)`, ph),
		fmt.Sprintf(`DELETE FROM invoice_payments WHERE invoice_id IN (%s)`, ph),
		fmt.Sprintf(`DELETE FROM invoice_customer_snapshots WHERE invoice_id IN (%s)`, ph),
		fmt.Sprintf(`DELETE FROM invoice_lines WHERE invoice_id IN (%s)`, ph),
		fmt.Sprintf(`DELETE FROM invoices WHERE id IN (%s)`, ph),
	}
	for _, q := range steps {
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

func tipAfterDeleteTx(tx *sql.Tx, seriesID string) (num int64, hash string, err error) {
	var max sql.NullInt64
	if err = tx.QueryRow(`SELECT MAX(sequence_number) FROM invoices WHERE series_id=?`, seriesID).Scan(&max); err != nil {
		return 0, "", err
	}
	if !max.Valid || max.Int64 == 0 {
		return 0, "", nil
	}
	num = max.Int64
	err = tx.QueryRow(`SELECT hash FROM invoices WHERE series_id=? AND sequence_number=?`, seriesID, num).Scan(&hash)
	return num, hash, err
}

func placeholders(ids []string) (string, []any) {
	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i] = "?"
		args[i] = id
	}
	return strings.Join(parts, ","), args
}
