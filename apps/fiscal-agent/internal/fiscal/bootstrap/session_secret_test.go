package bootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/api"
	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestNewSessionManagerOnlyInBootstrapGo(t *testing.T) {
	b, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatal(err)
	}
	if c := strings.Count(string(b), "NewSessionManager("); c != 1 {
		t.Fatalf("bootstrap.go must call NewSessionManager exactly once, got %d", c)
	}
}

func TestStartCoreRetailSessionSecretFile(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "secure")
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath:                filepath.Join(dir, "f.db"),
		DataDir:               dataDir,
		StoreID:               "store-retail-001",
		PrintSink:             &worker.MemorySink{},
		AutoSessionSecretFile: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	keyPath := filepath.Join(dataDir, api.SessionSecretFileName)
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected %s: %v", keyPath, err)
	}
}

func TestStartCoreFiscalLocalRequiresSecretOrDev(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	dir := t.TempDir()
	_, err := bootstrap.StartCore(bootstrap.Options{
		DBPath:    filepath.Join(dir, "f.db"),
		DataDir:   filepath.Join(dir, "secure"),
		StoreID:   "store-local-fail",
		PrintSink: &worker.MemorySink{},
	})
	if err == nil {
		t.Fatal("expected error without dev key, env, or AutoSessionSecretFile")
	}
}
