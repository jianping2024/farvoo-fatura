package store

import (
	"fmt"
	"math"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/audit"
)

// AuditLogQuery filters paginated audit_log reads (ONLY list query type).
type AuditLogQuery struct {
	Page        int
	PageSize    int
	Action      string
	OperatorID  string
	From        string
	To          string
	OwnerFilter bool
}

// AuditLogRow is one audit_log row with operator display name.
type AuditLogRow struct {
	ID                  string
	At                  string
	OperatorID          string
	OperatorDisplayName string
	Action              string
	EntityType          string
	EntityID            string
	DetailJSON          string
}

// AuditLogListResult is paginated audit_log list.
type AuditLogListResult struct {
	Items    []AuditLogRow
	Total    int
	Page     int
	PageSize int
}

var allowedAuditPageSizes = map[int]bool{10: true, 20: true, 50: true, 100: true}

func normalizeAuditLogQuery(q AuditLogQuery) (AuditLogQuery, int, int) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if !allowedAuditPageSizes[pageSize] {
		pageSize = 50
	}
	return AuditLogQuery{
		Page:        page,
		PageSize:    pageSize,
		Action:      strings.TrimSpace(q.Action),
		OperatorID:  strings.TrimSpace(q.OperatorID),
		From:        strings.TrimSpace(q.From),
		To:          strings.TrimSpace(q.To),
		OwnerFilter: q.OwnerFilter,
	}, page, pageSize
}

// ListAuditLog is the ONLY audit_log read path.
func (d *DB) ListAuditLog(q AuditLogQuery) (*AuditLogListResult, error) {
	q, page, pageSize := normalizeAuditLogQuery(q)
	where := `FROM audit_log a LEFT JOIN operators o ON o.id = a.operator_id WHERE 1=1`
	args := []any{}

	if q.OwnerFilter {
		ownerActs := audit.OwnerActionList()
		placeholders := strings.Repeat("?,", len(ownerActs))
		placeholders = placeholders[:len(placeholders)-1]
		where += ` AND a.action IN (` + placeholders + `)`
		for _, act := range ownerActs {
			args = append(args, act)
		}
	}
	if q.Action != "" {
		where += ` AND a.action = ?`
		args = append(args, q.Action)
	}
	if q.OperatorID != "" {
		where += ` AND a.operator_id = ?`
		args = append(args, q.OperatorID)
	}
	if q.From != "" {
		where += ` AND a.at >= ?`
		args = append(args, q.From)
	}
	if q.To != "" {
		where += ` AND a.at <= ?`
		args = append(args, q.To)
	}

	var total int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	totalPages := int(math.Max(1, math.Ceil(float64(total)/float64(pageSize))))
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	selectQuery := `
		SELECT a.id, a.at, IFNULL(a.operator_id,''), IFNULL(o.display_name,''),
			a.action, IFNULL(a.entity_type,''), IFNULL(a.entity_id,''), IFNULL(a.detail_json,'') ` +
		where + ` ORDER BY a.at DESC LIMIT ? OFFSET ?`
	selectArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := d.SQL.Query(selectQuery, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AuditLogRow, 0, pageSize)
	for rows.Next() {
		var r AuditLogRow
		if err := rows.Scan(&r.ID, &r.At, &r.OperatorID, &r.OperatorDisplayName,
			&r.Action, &r.EntityType, &r.EntityID, &r.DetailJSON); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &AuditLogListResult{
		Items: items, Total: total, Page: page, PageSize: pageSize,
	}, nil
}

// InsertLoginFailureAudit records a failed login attempt (ONLY LOGIN_FAILED write path).
func (d *DB) InsertLoginFailureAudit(operatorID, entityKey string) error {
	return d.InsertAuditLog(operatorID, "LOGIN_FAILED", "operator", entityKey, "{}")
}

// LoginFailureEntityKey is the ONLY entity_id format for LOGIN_FAILED rows.
func LoginFailureEntityKey(storeID, operatorID string) string {
	return fmt.Sprintf("login_fail:%s:%s", storeID, operatorID)
}
