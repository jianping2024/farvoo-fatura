package main

import (
	"testing"

	"farvoo-fiscal-agent/internal/escposenc"
)

func TestRunTestPrintForStationNeedsMapping(t *testing.T) {
	err := runTestPrintForStation(&config{StationPrinters: map[string]string{}}, "", "", "zh", "")
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
	err := runTestPrintForStation(cfg, "", "", "en", "")
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

func TestBuildCodePageProbeHasReferenceBitmapAndAllRows(t *testing.T) {
	raw := buildCodePageProbe("demo.farvoo.pt")
	if bytesIndex(raw, []byte("CODE PAGE TEST")) < 0 {
		t.Fatal("missing title")
	}
	if bytesIndex(raw, []byte{0x1D, 0x76, 0x30}) < 0 {
		t.Fatal("reference row must be GS v 0 bitmap")
	}
	if bytesIndex(raw, escposenc.CodePageProbeRows()) < 0 {
		t.Fatal("missing probe rows")
	}
	if bytesIndex(raw, escposenc.HanProbeRows()) < 0 {
		t.Fatal("missing Chinese probe rows")
	}
	if cutDotsInRaw(raw) == 0 {
		t.Fatal("probe slip must cut")
	}
}
