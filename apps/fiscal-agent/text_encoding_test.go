package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// withThermalConfig points loadConfig at a temp config; enc "" = no text_encoding key (auto).
// Keeps tests off the developer's real ~/.config file.
func withThermalConfig(t *testing.T, enc string) {
	t.Helper()
	body := `{"agentjwt":"x"}`
	if enc != "" {
		body = fmt.Sprintf(`{"agentjwt":"x","text_encoding":%q}`, enc)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })
}

func TestTextModeForThermalDefaultsBitmap(t *testing.T) {
	withThermalConfig(t, "")
	if got := textModeForThermal(true); got != escposTextBitmap {
		t.Fatalf("default non-latin text mode = %v want bitmap", got)
	}
	if got := textModeForThermal(false); got != escposTextLatin {
		t.Fatalf("latin text mode = %v", got)
	}
	withThermalConfig(t, "bitmap")
	if got := textModeForThermal(true); got != escposTextBitmap {
		t.Fatalf("legacy bitmap text mode = %v", got)
	}
	withThermalConfig(t, "han_firmware")
	if got := textModeForThermal(true); got != escposTextHanFirmware {
		t.Fatalf("han_firmware text mode = %v", got)
	}
}

func TestNormalizeTextEncodingNoGBKMode(t *testing.T) {
	if got := normalizeTextEncoding("gbk"); got != "auto" {
		t.Fatalf("gbk must fall back to auto bitmap mode, got %q", got)
	}
	if got := normalizeTextEncoding("utf-8"); got != "utf8" {
		t.Fatalf("utf8 alias = %q", got)
	}
}
