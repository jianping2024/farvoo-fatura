package locale

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PrefsFile is the ONLY file-backed ui_locale store for fiscal-local (DataDir/ui_locale.json).
// Agent embed must inject Get/Set callbacks against config.json instead — do not dual-write.
type PrefsFile struct {
	mu   sync.Mutex
	Path string
}

type prefsJSON struct {
	UILocale string `json:"ui_locale"`
}

// PathInDataDir is the ONLY relative prefs name under fiscal DataDir.
func PathInDataDir(dataDir string) string {
	return filepath.Join(dataDir, "ui_locale.json")
}

func (p *PrefsFile) Get() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	raw, err := os.ReadFile(p.Path)
	if err != nil {
		return "zh"
	}
	var j prefsJSON
	if json.Unmarshal(raw, &j) != nil {
		return "zh"
	}
	return NormalizeUILocale(j.UILocale)
}

func (p *PrefsFile) Set(uiLocale string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	loc := NormalizeUILocale(uiLocale)
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(prefsJSON{UILocale: loc}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.Path, raw, 0o600)
}
