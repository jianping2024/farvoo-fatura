package store_test

import (
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func seedAuditTestDB(t *testing.T) (*store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	storeID := "store-audit-001"
	if err := db.UpsertOperator("op-admin", storeID, "admin", "Admin", "local-op-admin"); err != nil {
		t.Fatal(err)
	}
	return db, storeID
}

func TestListAuditLogOwnerFilter(t *testing.T) {
	db, _ := seedAuditTestDB(t)
	defer db.Close()

	if err := db.InsertAuditLog("op-admin", "LOGIN", "operator", "op-admin", "{}"); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertAuditLog("", "fiscal_db_backup", "sqlite", "path", `{"path":"/tmp/backup.db"}`); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertAuditLog("op-admin", "EXPORT_SAFT", "saft_exports", "exp-1", `{"year":2026,"month":3}`); err != nil {
		t.Fatal(err)
	}

	adminAll, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if adminAll.Total != 3 {
		t.Fatalf("admin total: got %d want 3", adminAll.Total)
	}

	ownerView, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 50, OwnerFilter: true})
	if err != nil {
		t.Fatal(err)
	}
	if ownerView.Total != 2 {
		t.Fatalf("owner total: got %d want 2 (LOGIN+EXPORT_SAFT)", ownerView.Total)
	}
	for _, row := range ownerView.Items {
		if row.Action == "fiscal_db_backup" {
			t.Fatal("owner filter must exclude fiscal_db_backup")
		}
	}
}

func TestListAuditLogPaginationAndActionFilter(t *testing.T) {
	db, _ := seedAuditTestDB(t)
	defer db.Close()

	for i := 0; i < 5; i++ {
		if err := db.InsertAuditLog("op-admin", "LOGIN", "operator", "op-admin", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.InsertAuditLog("op-admin", "LOGOUT", "operator", "op-admin", "{}"); err != nil {
		t.Fatal(err)
	}

	page1, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 10, Action: "LOGIN"})
	if err != nil {
		t.Fatal(err)
	}
	if page1.Total != 5 || len(page1.Items) != 5 {
		t.Fatalf("page1: total=%d items=%d want 5/5", page1.Total, len(page1.Items))
	}
	page1Small, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 20, Action: "LOGIN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1Small.Items) != 5 {
		t.Fatalf("page1Small items: got %d want 5", len(page1Small.Items))
	}
	// Insert more LOGIN rows to exercise page 2 with page_size=10
	for i := 0; i < 6; i++ {
		if err := db.InsertAuditLog("op-admin", "LOGIN", "operator", "op-admin", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	page2, err := db.ListAuditLog(store.AuditLogQuery{Page: 2, PageSize: 10, Action: "LOGIN"})
	if err != nil {
		t.Fatal(err)
	}
	if page2.Total != 11 || len(page2.Items) != 1 {
		t.Fatalf("page2: total=%d items=%d want 11/1", page2.Total, len(page2.Items))
	}

	logoutOnly, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 50, Action: "LOGOUT"})
	if err != nil {
		t.Fatal(err)
	}
	if logoutOnly.Total != 1 {
		t.Fatalf("logout filter total: got %d want 1", logoutOnly.Total)
	}
}

func TestListAuditLogJoinsOperatorDisplayName(t *testing.T) {
	db, _ := seedAuditTestDB(t)
	defer db.Close()

	if err := db.InsertAuditLog("op-admin", "PIN_CHANGE", "operator", "op-admin", "{}"); err != nil {
		t.Fatal(err)
	}
	res, err := db.ListAuditLog(store.AuditLogQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].OperatorDisplayName != "Admin" {
		t.Fatalf("display_name join: %+v", res.Items)
	}
}
