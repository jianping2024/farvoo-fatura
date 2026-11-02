package bootstrap_test

import (
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func startCoreAt(t *testing.T, dbPath string) error {
	t.Helper()
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath:    dbPath,
		DataDir:   filepath.Join(filepath.Dir(dbPath), "sec"),
		StoreID:   "store-demo-001",
		PrintSink: &worker.MemorySink{},
	})
	if err == nil {
		rt.Close()
	}
	return err
}

func TestStartCore_InvalidATEnvFails(t *testing.T) {
	t.Setenv("FISCAL_AT_ENV", "production")
	if err := startCoreAt(t, filepath.Join(t.TempDir(), "f.db")); err == nil || !strings.Contains(err.Error(), "invalid fiscal_at_env") {
		t.Fatalf("want invalid env error, got %v", err)
	}
}

func TestStartCore_DBBoundToATEnv(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "f.db")
	t.Setenv("FISCAL_AT_ENV", "mock")
	if err := startCoreAt(t, dbPath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FISCAL_AT_ENV", "prod")
	if err := startCoreAt(t, dbPath); err == nil || !strings.Contains(err.Error(), `bound to AT env "mock"`) {
		t.Fatalf("mock db reopened as prod must fail, got %v", err)
	}
}

func TestStartCore_SeedRequiresMock(t *testing.T) {
	t.Setenv("FISCAL_AT_ENV", "test")
	_, err := bootstrap.StartCore(bootstrap.Options{
		DBPath:            filepath.Join(t.TempDir(), "f.db"),
		StoreID:           "store-demo-001",
		PrintSink:         &worker.MemorySink{},
		Seed:              true,
		SigningKeyPEMPath: filepath.Join("..", "testdata", "dev_signing_key.pem"),
	})
	if err == nil || !strings.Contains(err.Error(), "requires FISCAL_AT_ENV=mock") {
		t.Fatalf("want seed guard, got %v", err)
	}
}
