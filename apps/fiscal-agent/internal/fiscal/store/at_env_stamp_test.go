package store_test

import (
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestEnsureATEnvStamp_BindsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fiscal-test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureATEnvStamp("test"); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureATEnvStamp("test"); err != nil {
		t.Fatalf("same env reopen: %v", err)
	}
	if err := db.EnsureATEnvStamp("prod"); err == nil || !strings.Contains(err.Error(), `bound to AT env "test"`) {
		t.Fatalf("prod on test db must fail, got %v", err)
	}
	_ = db.Close()
}

func TestEnsureATEnvStamp_LegacyDataIsMock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fiscal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.SQL.Exec(`INSERT INTO series (id, store_id, document_type, series_code, validation_code, fiscal_year, status, created_at, updated_at)
		VALUES ('s1','store-demo-001','FT','FT2026A','CSDF7T5H',2026,'ACTIVE','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureATEnvStamp("prod"); err == nil || !strings.Contains(err.Error(), `bound to AT env "mock"`) {
		t.Fatalf("legacy data opened as prod must fail, got %v", err)
	}
	if err := db.EnsureATEnvStamp("mock"); err != nil {
		t.Fatalf("legacy stays usable as mock: %v", err)
	}
}
