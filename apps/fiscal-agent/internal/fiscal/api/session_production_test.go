package api

import (
	"testing"
)

func TestNewSessionManager_ProductionRequiresSecret(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	_, err := NewSessionManager(t.TempDir())
	if err != ErrSessionSecretRequired {
		t.Fatalf("got %v want %v", err, ErrSessionSecretRequired)
	}
}

func TestNewSessionManager_DevModeDerivedSecret(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	t.Setenv("FISCAL_SESSION_SECRET", "")
	sm, err := NewSessionManager(t.TempDir())
	if err != nil || sm == nil {
		t.Fatalf("err=%v sm=%v", err, sm)
	}
}

func TestNewSessionManager_ShortSecretRejected(t *testing.T) {
	t.Setenv("FISCAL_SESSION_SECRET", "short")
	_, err := NewSessionManager(t.TempDir())
	if err == nil {
		t.Fatal("expected error for short secret")
	}
}
