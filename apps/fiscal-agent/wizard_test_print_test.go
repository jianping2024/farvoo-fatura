package main

import (
	"fmt"
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
	if bytesIndex(raw, escposenc.SizeProbeRows()) < 0 {
		t.Fatal("missing SIZE 1x2 probe rows")
	}
	if bytesIndex(raw, []byte("SIZE 1x2")) < 0 {
		t.Fatal("missing SIZE 1x2 section title")
	}
	if cutDotsInRaw(raw) == 0 {
		t.Fatal("probe slip must cut")
	}
	if bytesIndex(raw, []byte("BITMAP SIZE")) < 0 {
		t.Fatal("missing BITMAP SIZE section")
	}
	for _, px := range bitmapSizeProbeBasePx {
		if bytesIndex(raw, []byte(fmt.Sprintf("px=%d", px))) < 0 {
			t.Fatalf("missing px=%d block", px)
		}
	}
	if bytesIndex(raw, []byte{0x1D, 0x76, 0x30}) < 0 {
		t.Fatal("size probe must raster the Han note / dish")
	}

}

func TestBuildStationSampleSlipUsesFirmwareForAllLines(t *testing.T) {
	withThermalConfig(t, "han_firmware")
	raw := buildStationSampleSlip("pt")
	if bytesIndex(raw, []byte{0x1D, 0x76, 0x30}) >= 0 {
		t.Fatal("station sample must print pt + zh from printer fonts, no GS v 0")
	}
	for _, want := range [][]byte{
		append(leadRow(2, "012"), escposenc.Windows1252(" Bacalhau à Brás")...),
		escposenc.Windows1252("Azeitonas"),
		escposenc.Windows1252("» sem cebola, "),
		leadRow(1, "006"),
		{0x1C, 0x26, 0x1D, 0x21, 0x01, 0xC6, 0xCF}, // 葡… other-language Han name 1×2
		{0x1C, 0x26, 0x1D, 0x21, 0x01},             // Han note 1×2
	} {
		if bytesIndex(raw, want) < 0 {
			t.Fatalf("station sample missing % X", want)
		}
	}
	if cutDotsInRaw(raw) == 0 {
		t.Fatal("station sample must cut")
	}
}

func TestRunTestPrintStationSampleNeedsMapping(t *testing.T) {
	if err := runTestPrintForStation(&config{StationPrinters: map[string]string{}}, "", "", "pt", testPrintKindStationSample); err == nil {
		t.Fatal("expected mapping error")
	}
}
