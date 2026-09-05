package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/ptnif"
	"github.com/shopspring/decimal"
)

// DayFSRow is one FS invoice on a business date (ordered by sequence).
type DayFSRow struct {
	ID         string
	InvoiceNo  string
	SeriesID   string
	SeriesCode string
	Seq        int64
	Gross      string
	PayMethod  string
	TaxID      string
	HasNCOrND  bool // original referenced by any NC/ND line
	Protected  bool
}

// SettlePlanInput feeds BuildSettlePlan (唯一计划入口).
type SettlePlanInput struct {
	BusinessDate string
	KeepTarget   string
	DBPath       string     // normalized by BuildSettlePlan
	State        *ToolState // settlements live in exe-adjacent JSON, not fiscal.db
}

// SettlePlan is the ONLY preview/apply payload for day settlement.
type SettlePlan struct {
	BusinessDate     string
	SeriesID         string
	SeriesCode       string
	KeepTarget       string
	KeepActual       string
	Shortfall        bool
	AnchorInvoiceNo  string
	AnchorSeq        int64
	CutoffInvoiceNo  string
	CutoffSeq        int64
	DeleteIDs        []string
	DeleteInvoiceNos []string
	DeleteGrossTotal string
	KeepIDs          []string
}

// IsProtectedInvoice is the ONLY protected-ticket rule:
// non-CASH OR buyer NIF ≠ Consumidor Final OR has NC/ND as original.
func IsProtectedInvoice(payMethod, taxID string, hasNCOrND bool) bool {
	if hasNCOrND {
		return true
	}
	taxID = strings.TrimSpace(taxID)
	if taxID != "" && taxID != ptnif.FinalConsumer {
		return true
	}
	return domain.NormalizePaymentMethod(payMethod) != domain.PaymentCash
}

// BuildSettlePlan is the ONLY settlement plan builder (no DB writes).
func BuildSettlePlan(ctx context.Context, db *sql.DB, in SettlePlanInput) (*SettlePlan, error) {
	biz := strings.TrimSpace(in.BusinessDate)
	if biz == "" {
		return nil, fmt.Errorf("business date required")
	}
	target, err := decimal.NewFromString(strings.TrimSpace(in.KeepTarget))
	if err != nil || target.IsNegative() {
		return nil, fmt.Errorf("invalid keep amount %q", in.KeepTarget)
	}
	dbPath, err := NormalizeDBPath(in.DBPath)
	if err != nil {
		return nil, err
	}
	if in.State == nil {
		return nil, fmt.Errorf("tool state required")
	}
	if IsDateSettledInState(in.State, dbPath, biz) {
		return nil, fmt.Errorf("营业日 %s 已结算", biz)
	}

	rows, err := loadDayFS(ctx, db, biz)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("营业日 %s 没有 FS 发票", biz)
	}
	seriesID := rows[0].SeriesID
	seriesCode := rows[0].SeriesCode
	for _, r := range rows {
		if r.SeriesID != seriesID {
			return nil, fmt.Errorf("营业日 %s 存在多个 FS 系列，拒绝结算", biz)
		}
	}
	if err := assertDayIsSeriesTip(db, seriesID, rows[len(rows)-1].Seq); err != nil {
		return nil, err
	}

	var anchorIdx = -1
	for i, r := range rows {
		if r.Protected {
			anchorIdx = i
		}
	}

	accumStart := 0
	anchorNo := ""
	var anchorSeq int64
	if anchorIdx >= 0 {
		accumStart = anchorIdx + 1
		anchorNo = rows[anchorIdx].InvoiceNo
		anchorSeq = rows[anchorIdx].Seq
	}

	keepActual := decimal.Zero
	cutoffIdx := accumStart - 1 // if nothing after anchor, cutoff is anchor (or -1 if no rows before accum)
	if anchorIdx >= 0 {
		cutoffIdx = anchorIdx
	} else if len(rows) > 0 {
		cutoffIdx = -1 // will advance while accumulating from 0
	}

	for i := accumStart; i < len(rows); i++ {
		g, err := decimal.NewFromString(rows[i].Gross)
		if err != nil {
			return nil, fmt.Errorf("gross %s: %w", rows[i].InvoiceNo, err)
		}
		keepActual = keepActual.Add(g)
		cutoffIdx = i
		if keepActual.GreaterThanOrEqual(target) && target.Sign() > 0 {
			break
		}
		if target.Sign() == 0 {
			// keep 0 → cutoff stays at anchor (or before first); delete all after anchor
			cutoffIdx = accumStart - 1
			if anchorIdx >= 0 {
				cutoffIdx = anchorIdx
			} else {
				cutoffIdx = -1
			}
			keepActual = decimal.Zero
			break
		}
	}

	// target > 0 but nothing to accumulate → cutoff at anchor / before first
	if accumStart >= len(rows) && target.Sign() > 0 {
		if anchorIdx >= 0 {
			cutoffIdx = anchorIdx
		} else {
			cutoffIdx = -1
		}
		keepActual = decimal.Zero
	}

	shortfall := target.Sign() > 0 && keepActual.LessThan(target)

	var keepIDs, delIDs, delNos []string
	delGross := decimal.Zero
	cutoffNo := ""
	var cutoffSeq int64
	if cutoffIdx >= 0 {
		cutoffNo = rows[cutoffIdx].InvoiceNo
		cutoffSeq = rows[cutoffIdx].Seq
	}
	for i, r := range rows {
		if i <= cutoffIdx {
			keepIDs = append(keepIDs, r.ID)
			continue
		}
		delIDs = append(delIDs, r.ID)
		delNos = append(delNos, r.InvoiceNo)
		g, _ := decimal.NewFromString(r.Gross)
		delGross = delGross.Add(g)
	}

	return &SettlePlan{
		BusinessDate:     biz,
		SeriesID:         seriesID,
		SeriesCode:       seriesCode,
		KeepTarget:       target.StringFixed(2),
		KeepActual:       keepActual.StringFixed(2),
		Shortfall:        shortfall,
		AnchorInvoiceNo:  anchorNo,
		AnchorSeq:        anchorSeq,
		CutoffInvoiceNo:  cutoffNo,
		CutoffSeq:        cutoffSeq,
		DeleteIDs:        delIDs,
		DeleteInvoiceNos: delNos,
		DeleteGrossTotal: delGross.StringFixed(2),
		KeepIDs:          keepIDs,
	}, nil
}

func loadDayFS(ctx context.Context, db *sql.DB, businessDate string) ([]DayFSRow, error) {
	// HasNCOrND: ONLY via invoice_line_references.original_invoice_id (any NC/ND credit line).
	q := `
SELECT i.id, i.invoice_no, i.series_id, i.series_code, i.sequence_number, i.gross_total,
       IFNULL((SELECT p.method FROM invoice_payments p WHERE p.invoice_id=i.id ORDER BY p.rowid LIMIT 1), 'CASH'),
       IFNULL((SELECT s.customer_tax_id FROM invoice_customer_snapshots s WHERE s.invoice_id=i.id), ''),
       EXISTS(SELECT 1 FROM invoice_line_references r WHERE r.original_invoice_id=i.id)
FROM invoices i
WHERE i.document_type='FS' AND i.invoice_date=?
ORDER BY i.sequence_number ASC`
	rs, err := db.QueryContext(ctx, q, businessDate)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []DayFSRow
	for rs.Next() {
		var r DayFSRow
		var hasCorrective int
		if err := rs.Scan(&r.ID, &r.InvoiceNo, &r.SeriesID, &r.SeriesCode, &r.Seq, &r.Gross, &r.PayMethod, &r.TaxID, &hasCorrective); err != nil {
			return nil, err
		}
		r.HasNCOrND = hasCorrective != 0
		r.Protected = IsProtectedInvoice(r.PayMethod, r.TaxID, r.HasNCOrND)
		out = append(out, r)
	}
	return out, rs.Err()
}

func assertDayIsSeriesTip(db *sql.DB, seriesID string, dayMaxSeq int64) error {
	var lastNum int64
	err := db.QueryRow(`SELECT last_number FROM series WHERE id=?`, seriesID).Scan(&lastNum)
	if err != nil {
		return err
	}
	if dayMaxSeq != lastNum {
		return fmt.Errorf("该日不是 FS 系列 tip（日末序号=%d，系列 last_number=%d）；结算代表当日营业结束，拒绝", dayMaxSeq, lastNum)
	}
	return nil
}

// LoadTaxpayerTimezone is the ONLY timezone reader for settle defaults.
func LoadTaxpayerTimezone(db *sql.DB) (string, error) {
	var tz string
	err := db.QueryRow(`SELECT IFNULL(timezone,'Europe/Lisbon') FROM taxpayer_settings LIMIT 1`).Scan(&tz)
	if err == sql.ErrNoRows {
		return "Europe/Lisbon", nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(tz) == "" {
		return "Europe/Lisbon", nil
	}
	return tz, nil
}

// ListEligibleSettleDates returns unsettled FS tip-capable business dates (desc).
func ListEligibleSettleDates(db *sql.DB, st *ToolState, dbPath, tzName string) ([]string, error) {
	_ = tzName
	norm, err := NormalizeDBPath(dbPath)
	if err != nil {
		return nil, err
	}
	rs, err := db.Query(`
SELECT DISTINCT i.invoice_date
FROM invoices i
WHERE i.document_type='FS'
ORDER BY i.invoice_date DESC`)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for rs.Next() {
		var d string
		if err := rs.Scan(&d); err != nil {
			rs.Close()
			return nil, err
		}
		candidates = append(candidates, d)
	}
	if err := rs.Err(); err != nil {
		rs.Close()
		return nil, err
	}
	rs.Close()

	var dates []string
	for _, d := range candidates {
		if IsDateSettledInState(st, norm, d) {
			continue
		}
		rows, err := loadDayFS(context.Background(), db, d)
		if err != nil || len(rows) == 0 {
			continue
		}
		if err := assertDayIsSeriesTip(db, rows[0].SeriesID, rows[len(rows)-1].Seq); err != nil {
			continue
		}
		dates = append(dates, d)
	}
	return dates, nil
}

// DefaultSettleDate prefers today (store tz) if eligible, else latest eligible.
func DefaultSettleDate(eligible []string, tzName string) string {
	if len(eligible) == 0 {
		return ""
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.FixedZone("Lisbon", 0)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	for _, d := range eligible {
		if d == today {
			return today
		}
	}
	return eligible[0]
}

// ValidateSettleDate ensures user pick is in eligible set.
func ValidateSettleDate(date string, eligible []string) error {
	for _, d := range eligible {
		if d == date {
			return nil
		}
	}
	return fmt.Errorf("日期 %s 不可结算（须为有 FS、未结算、且为系列 tip 的营业日）；可选: %s", date, strings.Join(eligible, ", "))
}
