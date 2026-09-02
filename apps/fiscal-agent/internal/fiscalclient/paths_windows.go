//go:build windows

package fiscalclient

import (
	"fmt"
	"os"
)

func localAppDataDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", fmt.Errorf("LOCALAPPDATA not set")
	}
	return base, nil
}
