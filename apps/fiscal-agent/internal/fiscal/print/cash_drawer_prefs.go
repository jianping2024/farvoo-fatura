package print

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PinPrefsFile stores cash_drawer_pin under DataDir when Agent config.json callbacks are absent (fiscal-local).
type PinPrefsFile struct {
	Path string
	mu   sync.Mutex
}

// PathInDataDir is the ONLY default prefs path for cash drawer pin under Fiscal DataDir.
func PathInDataDir(dataDir string) string {
	return filepath.Join(dataDir, "cash_drawer_pin.json")
}

type pinPrefsJSON struct {
	Pin int `json:"cash_drawer_pin"`
}

// Get is the ONLY reader for PinPrefsFile.
func (p *PinPrefsFile) Get() int {
	if p == nil || p.Path == "" {
		return 2
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	b, err := os.ReadFile(p.Path)
	if err != nil {
		return 2
	}
	var j pinPrefsJSON
	if json.Unmarshal(b, &j) != nil {
		return 2
	}
	return NormalizeCashDrawerPin(j.Pin)
}

// Set is the ONLY writer for PinPrefsFile.
func (p *PinPrefsFile) Set(pin int) error {
	if p == nil || p.Path == "" {
		return os.ErrInvalid
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(pinPrefsJSON{Pin: NormalizeCashDrawerPin(pin)})
	if err != nil {
		return err
	}
	return os.WriteFile(p.Path, b, 0o600)
}
