package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSessionManager_ProductionRequiresSecret(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	_, err := NewSessionManager(t.TempDir(), false)
	if err != ErrSessionSecretRequired {
		t.Fatalf("got %v want %v", err, ErrSessionSecretRequired)
	}
}

func TestNewSessionManager_DevModeDerivedSecret(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	sm, err := NewSessionManager(t.TempDir(), false)
	if err != nil || sm == nil {
		t.Fatalf("err=%v sm=%v", err, sm)
	}
}

func TestNewSessionManager_ShortSecretRejected(t *testing.T) {
	t.Setenv("FISCAL_SESSION_SECRET", "short")
	_, err := NewSessionManager(t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestNewSessionManager_AutoFileProvisions(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	dir := t.TempDir()
	sm, err := NewSessionManager(dir, true)
	if err != nil || sm == nil {
		t.Fatalf("autoFile err=%v sm=%v", err, sm)
	}
	if _, err := os.Stat(filepath.Join(dir, sessionSecretFileName)); err != nil {
		t.Fatalf("expected session_hmac.key: %v", err)
	}
	sm2, err := NewSessionManager(dir, true)
	if err != nil || sm2 == nil {
		t.Fatalf("reload err=%v sm=%v", err, sm2)
	}
}
