package store

import (
	"time"
)

// AuditLogRetentionDays is the ONLY age-based retention window for audit_log
// (settings → 操作记录 keeps this many calendar days; older rows are purged).
const AuditLogRetentionDays = 365

// PurgeExpiredAuditLogs is the ONLY age-based audit_log DELETE path.
// Keeps rows with at >= now−AuditLogRetentionDays (UTC RFC3339, same as InsertAuditLog).
func (d *DB) PurgeExpiredAuditLogs() (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -AuditLogRetentionDays).Format(time.RFC3339)
	res, err := d.SQL.Exec(`DELETE FROM audit_log WHERE at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}
