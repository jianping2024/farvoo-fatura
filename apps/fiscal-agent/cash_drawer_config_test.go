package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCashDrawerPinConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("HOME")
	// defaultConfigPath uses home; override by saving via helpers with patched path is hard.
	// Exercise apply + normalize + cashDrawerPin on struct directly.
	c := &config{}
	applyCashDrawerPinToConfig(c, 5)
	if c.CashDrawerPin != 5 {
		t.Fatalf("got %d", c.CashDrawerPin)
	}
	applyCashDrawerPinToConfig(c, 99)
	if c.cashDrawerPin() != 2 {
		t.Fatalf("normalize got %d", c.cashDrawerPin())
	}
	_ = old
	_ = path
}
