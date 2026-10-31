package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHanBitmapFontPxFromConfigIgnoresPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"agentjwt":"x","han_bitmap_font_px":28}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })

	if got := hanBitmapFontPxFromConfig(); got != 28 {
		t.Fatalf("config px=%d want 28", got)
	}
	w := newEscposForStationTicket(jobPayload{HanBitmapFontPx: 40, Locale: "pt"})
	if w.hanFontPx != 28 {
		t.Fatalf("writer must use agent config 28, not payload 40; got %d", w.hanFontPx)
	}
}

func TestHanBitmapFontPxDefault22(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"agentjwt":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })
	if got := hanBitmapFontPxFromConfig(); got != 22 {
		t.Fatalf("default=%d want 22", got)
	}
	if bitmapTextDefaultFontPx != 22 {
		t.Fatalf("bitmapTextDefaultFontPx=%d want 22", bitmapTextDefaultFontPx)
	}
}
