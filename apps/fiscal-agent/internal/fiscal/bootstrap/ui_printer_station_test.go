package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIPrinterStationAssets(t *testing.T) {
	if !strings.Contains(string(fiscalUIPrinterStationJS), "FiscalUI.formatPrinterStationOption") {
		t.Fatal("printer-station.js must export FiscalUI.formatPrinterStationOption")
	}
	if n := strings.Count(string(fiscalUIPrinterStationJS), "function formatPrinterStationOption"); n != 1 {
		t.Fatalf("formatPrinterStationOption must be defined once, got %d", n)
	}
	if n := strings.Count(string(fiscalUIPrinterStationJS), "function printerDisplayName"); n != 1 {
		t.Fatalf("printerDisplayName must be defined once, got %d", n)
	}
}

func TestAdminHTMLPrinterStationUnique(t *testing.T) {
	if !strings.Contains(adminHTML, `/fiscal-ui/printer-station.js`) {
		t.Fatal("admin must load fiscal-ui printer-station.js")
	}
	if strings.Contains(adminHTML, "function printerDisplayName") {
		t.Fatal("printerDisplayName must live only in printer-station.js")
	}
	if strings.Contains(adminHTML, "function formatPrinterStationOption") {
		t.Fatal("formatPrinterStationOption must live only in printer-station.js")
	}
	if n := strings.Count(adminHTML, "FiscalUI.formatPrinterStationOption"); n != 1 {
		t.Fatalf("FiscalUI.formatPrinterStationOption must be used exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "s.id.slice(0, 8)") {
		t.Fatal("admin must not inline UUID fallback label formatting")
	}
	if strings.Contains(adminHTML, "s.printer || s.id") {
		t.Fatal("admin must not use legacy printer-only option labels")
	}
}
