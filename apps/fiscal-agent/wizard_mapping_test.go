package main

import "testing"

func TestApplyPrinterSetupClearsAllMappings(t *testing.T) {
	cfg := &config{
		StationPrinters: map[string]string{
			"kitchen": "tcp:10.0.0.1:9100",
		},
	}
	cleaned, err := applyPrinterSetup(cfg, setupRequestBody{
		StationPrinters: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cleaned) != 0 {
		t.Fatalf("expected empty cleaned, got %v", cleaned)
	}
	if cfg.StationPrinters != nil {
		t.Fatalf("expected nil StationPrinters, got %v", cfg.StationPrinters)
	}
}

func TestApplyPrinterSetupKeepsValidMappings(t *testing.T) {
	cfg := &config{}
	cleaned, err := applyPrinterSetup(cfg, setupRequestBody{
		StationPrinters: map[string]string{
			" kitchen ": " tcp:10.0.0.2:9100 ",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cleaned) != 1 {
		t.Fatalf("expected 1 mapping, got %v", cleaned)
	}
	if cfg.StationPrinters == nil || cfg.StationPrinters["kitchen"] == "" {
		t.Fatalf("expected kitchen mapping saved, got %v", cfg.StationPrinters)
	}
}
