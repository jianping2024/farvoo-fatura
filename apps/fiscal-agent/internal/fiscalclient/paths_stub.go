//go:build !windows

package fiscalclient

import (
	"fmt"
	"os"
	"path/filepath"
)

func localAppDataDir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "farvoo-fiscal-client"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "farvoo-fiscal-client"), nil
}
