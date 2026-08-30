package store

import (
	"time"

	"github.com/google/uuid"
)

// InsertAuditLog is the ONLY audit_log write path.
func (d *DB) InsertAuditLog(operatorID, action, entityType, entityID, detailJSON string) error {
	_, err := d.SQL.Exec(`INSERT INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), time.Now().UTC().Format(time.RFC3339),
		nullStr(operatorID), action, nullStr(entityType), nullStr(entityID), nullStr(detailJSON))
	return err
}
