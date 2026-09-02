package api

import "os"

// IsFiscalDevMode reports UAT/dev key bypass for session secret — ONLY dev mode gate.
func IsFiscalDevMode() bool {
	return os.Getenv("FISCAL_ALLOW_DEV_KEY") == "1"
}
