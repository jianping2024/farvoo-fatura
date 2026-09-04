package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIDateRangeAssets(t *testing.T) {
	js := string(fiscalUIDateRangeJS)
	if !strings.Contains(js, "FiscalUI.createDateRangeFilter") {
		t.Fatal("date-range.js must export FiscalUI.createDateRangeFilter")
	}
	if !strings.Contains(js, "FiscalUI.dateRangePresetToDates") {
		t.Fatal("date-range.js must export FiscalUI.dateRangePresetToDates")
	}
	if n := strings.Count(js, "function createDateRangeFilter"); n != 1 {
		t.Fatalf("createDateRangeFilter must be defined once in date-range.js, got %d", n)
	}
	if !strings.Contains(js, "options.getLabels") {
		t.Fatal("date-range must re-read getLabels on paint (locale switch)")
	}
	if n := strings.Count(js, "relabel:"); n != 1 {
		t.Fatalf("date-range relabel must exist once, got %d", n)
	}
	if n := strings.Count(js, "function paint"); n != 1 {
		t.Fatalf("date-range paint must be the ONLY label/preset render path, got %d", n)
	}
	if strings.Contains(js, "label: '今天'") || strings.Contains(js, "label: \"今天\"") {
		t.Fatal("preset labels must not be hardcoded on PRESET ids; use getLabels only")
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
		"function resetInvoiceListPage",
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
	if n := strings.Count(adminHTML, "function listDateRangeLabels"); n != 1 {
		t.Fatalf("listDateRangeLabels must be the ONLY date-range copy, got %d", n)
	}
	if n := strings.Count(adminHTML, "getLabels: listDateRangeLabels"); n != 1 {
		t.Fatalf("invoice date filter must use listDateRangeLabels once, got %d", n)
	}
	if !strings.Contains(adminHTML, "invoiceDateFilterCtrl]") {
		t.Fatal("applyAdminLocale must include invoiceDateFilterCtrl in the shared relabel loop")
	}
	if n := strings.Count(adminHTML, ".relabel()"); n != 1 {
		t.Fatalf("relabel() must be invoked from exactly one locale loop, got %d", n)
	}
	for _, key := range []string{
		"date.today", "date.yesterday", "date.last7", "date.month", "date.custom",
		"date.group_aria", "date.from", "date.to", "date.apply", "date.err_need", "date.err_order",
	} {
		if !strings.Contains(string(fiscalUIAdminI18nJS), "'"+key+"'") {
			t.Fatalf("admin-i18n missing %s", key)
		}
	}
	if !strings.Contains(adminHTML, "invoices.empty_match") || !strings.Contains(adminHTML, "invoices.empty_range") {
		t.Fatal("invoice list must distinguish empty range vs empty search via i18n keys")
	}
	if !strings.Contains(adminHTML, "common.loading") {
		t.Fatal("invoice list must show loading state via common.loading")
	}
}
