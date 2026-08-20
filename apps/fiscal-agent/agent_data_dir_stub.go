//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func agentDataDir() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "farvoo-fiscal-agent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "MesaPrintAgent")
	}
	return filepath.Join(home, ".local", "state", "farvoo-fiscal-agent")
}
