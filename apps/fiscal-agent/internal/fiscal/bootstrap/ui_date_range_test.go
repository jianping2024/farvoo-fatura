package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIDatePickerAssets(t *testing.T) {
	js := string(fiscalUIDatePickerJS)
	if !strings.Contains(js, "FiscalUI.createDatePicker") {
		t.Fatal("date-picker.js must export FiscalUI.createDatePicker")
	}
	if n := strings.Count(js, "function createDatePicker"); n != 1 {
		t.Fatalf("createDatePicker must be defined once, got %d", n)
	}
	if strings.Contains(js, "type=\"date\"") || strings.Contains(js, "type='date'") {
		t.Fatal("date-picker must not use native input type=date")
	}
	if !strings.Contains(string(fiscalUIDatePickerCSS), ".fiscal-date-picker__popup") {
		t.Fatal("date-picker.css must style portal popup")
	}
	if !strings.Contains(adminHTML, `/fiscal-ui/date-picker.js`) || !strings.Contains(adminHTML, `/fiscal-ui/date-picker.css`) {
		t.Fatal("admin must load fiscal-ui date-picker assets before date-range")
	}
	pickerIdx := strings.Index(adminHTML, `/fiscal-ui/date-picker.js`)
	rangeIdx := strings.Index(adminHTML, `/fiscal-ui/date-range.js`)
	if pickerIdx < 0 || rangeIdx < 0 || pickerIdx > rangeIdx {
		t.Fatal("date-picker.js must load before date-range.js")
	}
}

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
	if n := strings.Count(js, "function paint()"); n != 1 {
		t.Fatalf("date-range paint must be the ONLY label/preset render path, got %d", n)
	}
	if n := strings.Count(js, "function paintPresets()"); n != 1 {
		t.Fatalf("paintPresets must appear once, got %d", n)
	}
	if n := strings.Count(js, "var PRESET_IDS = ['today', 'yesterday', 'last7', 'month'];"); n != 1 {
		t.Fatalf("PRESET_IDS must be the four quick pills only (no custom chip), got %d", n)
	}
	if strings.Contains(js, "['today', 'yesterday', 'last7', 'month', 'custom']") {
		t.Fatal("must not keep custom as a clickable PRESET_IDS chip")
	}
	if strings.Contains(js, "label: '今天'") || strings.Contains(js, "label: \"今天\"") {
		t.Fatal("preset labels must not be hardcoded on PRESET ids; use getLabels only")
	}
	if strings.Contains(js, "type=\"date\"") || strings.Contains(js, "type='date'") {
		t.Fatal("date-range must not use native input type=date; use createDatePicker only")
	}
	if !strings.Contains(js, "FiscalUI.createDatePicker") {
		t.Fatal("date-range must mount from/to via FiscalUI.createDatePicker")
	}
	if n := strings.Count(js, "FiscalUI.createDatePicker("); n != 2 {
		t.Fatalf("date-range must create exactly two DatePickers (from/to), got %d", n)
	}
	if strings.Contains(string(fiscalUIDateRangeCSS), "fiscal-date-custom.hidden") ||
		strings.Contains(string(fiscalUIDateRangeCSS), ".fiscal-date-custom.hidden") {
		t.Fatal("date-range must not hide custom panel with display:none (layout shift)")
	}
	if !strings.Contains(string(fiscalUIDateRangeCSS), "button.fiscal-date-preset") {
		t.Fatal("date-range.css must style outline preset chips (override global primary button)")
	}
	if !strings.Contains(string(fiscalUIDateRangeCSS), "button.fiscal-date-preset.active") {
		t.Fatal("date-range.css must define clear .active preset state")
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
	if !strings.Contains(adminHTML, "getLocale:") {
		t.Fatal("invoice date filter must pass getLocale for DatePicker labels")
	}
	if !strings.Contains(adminHTML, "invoiceDateFilterCtrl]") {
		t.Fatal("applyAdminLocale must include invoiceDateFilterCtrl in the shared relabel loop")
	}
	if n := strings.Count(adminHTML, ".relabel()"); n != 1 {
		t.Fatalf("relabel() must be invoked from exactly one locale loop, got %d", n)
	}
	for _, key := range []string{
		"date.today", "date.yesterday", "date.last7", "date.month",
		"date.group_aria", "date.from", "date.to", "date.apply", "date.err_need", "date.err_order",
	} {
		if !strings.Contains(string(fiscalUIAdminI18nJS), "'"+key+"'") {
			t.Fatalf("admin-i18n missing %s", key)
		}
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "'date.custom'") {
		t.Fatal("date.custom i18n must be removed (no custom preset pill)")
	}
	if strings.Contains(adminHTML, "date.custom") {
		t.Fatal("listDateRangeLabels must not reference date.custom")
	}
	if !strings.Contains(adminHTML, "invoices.empty_match") || !strings.Contains(adminHTML, "invoices.empty_range") {
		t.Fatal("invoice list must distinguish empty range vs empty search via i18n keys")
	}
	if !strings.Contains(adminHTML, "invoices.empty_next") || !strings.Contains(adminHTML, "invoices.empty_next_search") {
		t.Fatal("invoice empty state must include next-step i18n keys")
	}
	if !strings.Contains(adminHTML, "function invoiceListEmptyCellHtml") {
		t.Fatal("invoice empty row must use invoiceListEmptyCellHtml only")
	}
	if !strings.Contains(adminHTML, "common.loading") {
		t.Fatal("invoice list must show loading state via common.loading")
	}
}
