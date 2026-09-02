package fiscalclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const productDirName = "Farvoo Fiscal Client"

// ConfigPath returns the Client settings file path for this OS.
func ConfigPath() (string, error) {
	base, err := localAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, productDirName, "config.json"), nil
}

// WebViewDataDir returns the WebView2 user-data folder for session cookies.
func WebViewDataDir() (string, error) {
	base, err := localAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, productDirName, "webview"), nil
}

// LoadConfig reads config.json; missing file returns zero Config, nil error.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	cfg.AgentBase, err = NormalizeAgentBaseURL(cfg.AgentBase)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SaveConfig writes config.json after normalization.
func SaveConfig(cfg Config) error {
	normalized, err := NormalizeAgentBaseURL(cfg.AgentBase)
	if err != nil {
		return err
	}
	cfg.AgentBase = normalized
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
