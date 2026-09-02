package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeUILocale(t *testing.T) {
	cases := map[string]string{
		"": "zh", "zh": "zh", "EN": "en", "pt-BR": "pt", "bogus": "zh",
	}
	for in, want := range cases {
		if got := normalizeUILocale(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestUILocaleDefaultZh(t *testing.T) {
	c := &config{}
	if c.uiLocale() != "zh" {
		t.Fatalf("default %q", c.uiLocale())
	}
	c.UILocale = "en"
	if c.uiLocale() != "en" {
		t.Fatalf("got %q", c.uiLocale())
	}
}

func TestSetAgentUILocalePersists(t *testing.T) {
	dir := t.TempDir()
	prev := configPathOverride
	configPathOverride = filepath.Join(dir, "config.json")
	t.Cleanup(func() { configPathOverride = prev })

	if err := setAgentUILocale("en"); err != nil {
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

func TestSetAgentUILocaleCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })

	if err := setAgentUILocale("pt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
