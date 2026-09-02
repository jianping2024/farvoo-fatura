package api

import (
	"errors"
	"log"
	"os"
)

// ErrSessionSecretRequired is returned when production lacks FISCAL_SESSION_SECRET.
var ErrSessionSecretRequired = errors.New("api: FISCAL_SESSION_SECRET required when FISCAL_ALLOW_DEV_KEY is not 1")

// IsFiscalDevMode reports UAT/dev key bypass for session secret — ONLY dev mode gate.
func IsFiscalDevMode() bool {
	return os.Getenv("FISCAL_ALLOW_DEV_KEY") == "1"
}

// MustNewSessionManager loads session HMAC secret or exits the process — ONLY production fatal gate.
func MustNewSessionManager(dataDir string) *SessionManager {
	sm, err := NewSessionManager(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	return sm
}
