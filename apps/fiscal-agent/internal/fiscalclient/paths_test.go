package fiscalclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	if err := SaveConfig(Config{AgentBase: "http://192.168.1.10:17880"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentBase != "http://192.168.1.10:17880" {
		t.Fatalf("got %q", cfg.AgentBase)
	}
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != productDirName {
		t.Fatalf("config dir: %s", path)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentBase != "" {
		t.Fatalf("expected empty config, got %q", cfg.AgentBase)
	}
}

func TestSaveConfigRejectsBadURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := SaveConfig(Config{AgentBase: "https://bad"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestWebViewDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	p, err := WebViewDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p, 0o700); err != nil {
		t.Fatal(err)
	}
}
