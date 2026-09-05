package store_test

import (
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/store"
	"github.com/google/uuid"
)

func TestPurgeExpiredAuditLogsKeepsOneYear(t *testing.T) {
	db, _ := seedAuditTestDB(t)

	oldID := uuid.NewString()
	keepID := uuid.NewString()
	oldAt := time.Now().UTC().AddDate(0, 0, -(store.AuditLogRetentionDays + 1)).Format(time.RFC3339)
	keepAt := time.Now().UTC().AddDate(0, 0, -(store.AuditLogRetentionDays - 1)).Format(time.RFC3339)

	if _, err := db.SQL.Exec(`INSERT INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
		VALUES (?, ?, 'op-admin', 'LOGIN', 'operator', 'op-admin', '{}')`, oldID, oldAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.Exec(`INSERT INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
		VALUES (?, ?, 'op-admin', 'LOGIN', 'operator', 'op-admin', '{}')`, keepID, keepAt); err != nil {
		t.Fatal(err)
	}

	n, err := db.PurgeExpiredAuditLogs()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 purged, got %d", n)
	}

	var oldLeft, keepLeft int
	_ = db.SQL.QueryRow(`SELECT COUNT(1) FROM audit_log WHERE id=?`, oldID).Scan(&oldLeft)
	_ = db.SQL.QueryRow(`SELECT COUNT(1) FROM audit_log WHERE id=?`, keepID).Scan(&keepLeft)
	if oldLeft != 0 {
		t.Fatal("row older than retention must be deleted")
	}
	if keepLeft != 1 {
		t.Fatal("row within retention must be kept")
	}
}

func TestListAuditLogDefaultPageSize(t *testing.T) {
	db, _ := seedAuditTestDB(t)
	for i := 0; i < 15; i++ {
		if err := db.InsertAuditLog("op-admin", "LOGIN", "operator", "op-admin", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	res, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res.PageSize != store.AuditLogDefaultPageSize {
		t.Fatalf("default page_size: got %d want %d", res.PageSize, store.AuditLogDefaultPageSize)
	}
	if len(res.Items) != store.AuditLogDefaultPageSize {
		t.Fatalf("items: got %d want %d", len(res.Items), store.AuditLogDefaultPageSize)
	}
}
