package main

import "testing"

func TestRunTestPrintForStationNeedsMapping(t *testing.T) {
	err := runTestPrintForStation(&config{StationPrinters: map[string]string{}}, "", "", "zh")
	if err == nil {
		t.Fatal("expected mapping error")
	}
}

func TestRunTestPrintForStationPicksFirstMapped(t *testing.T) {
	cfg := &config{
		StationPrinters: map[string]string{
			"st-1": "tcp://127.0.0.1:9100",
		},
	}
	// Will fail at printToTarget (no printer), but must get past mapping / locale normalize.
	err := runTestPrintForStation(cfg, "", "", "en")
	if err == nil {
		t.Fatal("expected print failure without live printer")
	}
}

func TestNormalizePrintLocaleForTestSlip(t *testing.T) {
	if got := normalizePrintLocale("zh"); got != "zh" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePrintLocale(""); got != "pt" {
		t.Fatalf("empty should default pt, got %q", got)
	}
}
