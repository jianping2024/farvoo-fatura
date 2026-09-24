package store

import (
	"fmt"
	"strings"
)

// requireFiscalTerminalFreeze is the ONLY gate that every IssueFT/NC/ND/RG insert must pass
// before writing fiscal_terminal_id / fiscal_terminal_label.
func requireFiscalTerminalFreeze(id, label string) (string, string, error) {
	id = strings.TrimSpace(id)
	label = strings.TrimSpace(label)
	if id == "" {
		return "", "", fmt.Errorf("store: fiscal_terminal_id required")
	}
	if label == "" {
		return "", "", fmt.Errorf("store: fiscal_terminal_label required")
	}
	return id, label, nil
}
