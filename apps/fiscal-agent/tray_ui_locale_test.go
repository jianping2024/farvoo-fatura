//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetTrayUILocalePersists(t *testing.T) {
	dir := t.TempDir()
	prev := configPathOverride
	configPathOverride = filepath.Join(dir, "config.json")
	t.Cleanup(func() { configPathOverride = prev })

	if err := setTrayUILocale("en"); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(configPathOverride)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.uiLocale() != "en" {
		t.Fatalf("got %q want en", cfg.uiLocale())
	}
}

func TestUILocaleOptionTitleMarksCurrent(t *testing.T) {
	if got := uiLocaleOptionTitle("zh", "zh"); got != "✓ "+uiT("zh", "menu_ui_locale_opt_zh") {
		t.Fatalf("zh mark: %q", got)
	}
	if got := uiLocaleOptionTitle("zh", "en"); got != uiT("zh", "menu_ui_locale_opt_en") {
		t.Fatalf("en unmarked: %q", got)
	}
}

func TestSetTrayUILocaleCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })

	if err := setTrayUILocale("pt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
