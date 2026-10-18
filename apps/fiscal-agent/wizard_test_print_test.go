package main

import (
	"bytes"
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
}

func TestBuildStationSampleSlipUsesFirmwareForAllLines(t *testing.T) {
	withThermalConfig(t, "")
	raw := buildStationSampleSlip("pt")
	if bytesIndex(raw, []byte{0x1D, 0x76, 0x30}) >= 0 {
		t.Fatal("station sample must print pt + zh from printer fonts, no GS v 0")
	}
	for _, want := range [][]byte{
		append(reverseBlock(2, "012"), escposenc.Windows1252(" Bacalhau à Brás")...),
		escposenc.Windows1252("Azeitonas"),
		escposenc.Windows1252("» sem cebola, "),
		reverseBlock(1, "006"),
		{0x1C, 0x26, 0x1D, 0x21, 0x01}, // Han 1×2 via GS ! (dish + note)
	} {
		if bytesIndex(raw, want) < 0 {
			t.Fatalf("station sample missing % X", want)
		}
	}
	if cutDotsInRaw(raw) == 0 {
		t.Fatal("station sample must cut")
	}
	probe := bytesIndex(raw, []byte("QTY / CODE PROBE"))
	if probe < 0 {
		t.Fatal("station sample must append the qty/code probe slip")
	}
	for _, r := range qtyCodeProbeRows {
		if bytesIndex(raw[probe:], []byte(r.label)) < 0 {
			t.Fatalf("probe missing row %q", r.label)
		}
	}
	if bytes.Count(raw, []byte{0x1D, 0x56, 0x42}) != 2 {
		t.Fatal("sample slip and probe slip each cut once")
	}
}

func TestQtyCodeProbeRowStyles(t *testing.T) {
	raw := buildQtyCodeProbeSlip("pt")
	block := escposenc.Windows1252(stationSlipQtyCodeBlock(2, "020"))
	lead := escposenc.Windows1252("2× 020")
	for _, want := range [][]byte{
		append([]byte{0x1D, 0x21, 0x01, 0x1B, 0x45, 0x00, 0x1D, 0x42, 0x01}, block...), // A
		append([]byte{0x1D, 0x21, 0x11, 0x1B, 0x45, 0x00, 0x1D, 0x42, 0x01}, block...), // B
		append([]byte{0x1D, 0x21, 0x11, 0x1B, 0x45, 0x01}, lead...),                    // C
		append([]byte{0x1D, 0x21, 0x01, 0x1B, 0x45, 0x00, 0x1B, 0x2D, 0x02}, lead...),  // D
		append([]byte{0x1D, 0x21, 0x11, 0x1B, 0x45, 0x00}, lead...),                    // E
	} {
		if bytesIndex(raw, want) < 0 {
			t.Fatalf("probe missing % X", want)
		}
	}
}

func TestRunTestPrintStationSampleNeedsMapping(t *testing.T) {
	if err := runTestPrintForStation(&config{StationPrinters: map[string]string{}}, "", "", "pt", testPrintKindStationSample); err == nil {
		t.Fatal("expected mapping error")
	}
}
