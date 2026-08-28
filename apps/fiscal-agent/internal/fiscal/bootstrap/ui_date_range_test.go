package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIDateRangeAssets(t *testing.T) {
	if !strings.Contains(string(fiscalUIDateRangeJS), "FiscalUI.createDateRangeFilter") {
		t.Fatal("date-range.js must export FiscalUI.createDateRangeFilter")
	}
	if !strings.Contains(string(fiscalUIDateRangeJS), "FiscalUI.dateRangePresetToDates") {
		t.Fatal("date-range.js must export FiscalUI.dateRangePresetToDates")
	}
	if n := strings.Count(string(fiscalUIDateRangeJS), "function createDateRangeFilter"); n != 1 {
		t.Fatalf("createDateRangeFilter must be defined once in date-range.js, got %d", n)
	}
	if !strings.Contains(string(fiscalUIDateRangeCSS), ".fiscal-date-presets") {
		t.Fatal("date-range.css must style fiscal-date-presets")
	}
}

func TestAdminHTMLInvoiceFiltersUnique(t *testing.T) {
	if !strings.Contains(adminHTML, `/fiscal-ui/date-range.js`) || !strings.Contains(adminHTML, `/fiscal-ui/date-range.css`) {
		t.Fatal("admin must load fiscal-ui date-range assets")
	}
	if strings.Count(adminHTML, `id="invoiceDateFilter"`) != 1 {
		t.Fatal("invoiceDateFilter mount must exist exactly once")
	}
	if strings.Count(adminHTML, `id="invoiceSearch"`) != 1 {
		t.Fatal("invoiceSearch must exist exactly once")
	}
	for _, fn := range []string{
		"function buildInvoicesQueryPath",
		"function initInvoiceFilters",
		"async function refreshHomeStats",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if n := strings.Count(adminHTML, "async function refreshInvoices"); n != 1 {
		t.Fatalf("refreshInvoices must be the ONLY invoice table loader, got %d", n)
	}
	if !strings.Contains(adminHTML, "FiscalUI.createDateRangeFilter('#invoiceDateFilter'") {
		t.Fatal("invoice date filter must use FiscalUI.createDateRangeFilter only")
	}
	if !strings.Contains(adminHTML, "未找到匹配的发票") || !strings.Contains(adminHTML, "该时段暂无发票") {
		t.Fatal("invoice list must distinguish empty range vs empty search")
	}
	if !strings.Contains(adminHTML, "加载中") {
		t.Fatal("invoice list must show loading state")
	}
}
