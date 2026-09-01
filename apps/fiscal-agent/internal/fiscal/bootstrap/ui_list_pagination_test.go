package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIListPaginationAssets(t *testing.T) {
	if !strings.Contains(string(fiscalUIListPaginationJS), "FiscalUI.createListPaginationBar") {
		t.Fatal("list-pagination.js must export FiscalUI.createListPaginationBar")
	}
	if n := strings.Count(string(fiscalUIListPaginationJS), "function createListPaginationBar"); n != 1 {
		t.Fatalf("createListPaginationBar must be defined once in list-pagination.js, got %d", n)
	}
	if !strings.Contains(string(fiscalUIListPaginationCSS), ".fiscal-list-pagination") {
		t.Fatal("list-pagination.css must style fiscal-list-pagination")
	}
}

func TestAdminHTMLInvoiceListPaginationUnique(t *testing.T) {
	if !strings.Contains(adminHTML, `/fiscal-ui/list-pagination.js`) || !strings.Contains(adminHTML, `/fiscal-ui/list-pagination.css`) {
		t.Fatal("admin must load fiscal-ui list-pagination assets")
	}
	if strings.Contains(adminHTML, `id="invoiceListMeta"`) {
		t.Fatal("invoiceListMeta must be removed; total lives in pagination bar")
	}
	if strings.Contains(adminHTML, "function paintInvoiceListMeta") {
		t.Fatal("paintInvoiceListMeta must be removed")
	}
	if strings.Count(adminHTML, `id="invoiceListPagination"`) != 1 {
		t.Fatal("invoiceListPagination mount must exist exactly once")
	}
	if strings.Count(adminHTML, "invoice-list-panel") != 1 {
		t.Fatal("invoice list must use a single merged panel")
	}
	for _, fn := range []string{
		"function buildInvoicesQueryPath",
		"function initInvoiceFilters",
		"function resetInvoiceListPage",
		"async function refreshHomeStats",
		"async function refreshInvoices",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if !strings.Contains(adminHTML, "FiscalUI.createListPaginationBar('#invoiceListPagination'") {
		t.Fatal("invoice pagination must use FiscalUI.createListPaginationBar only")
	}
	if strings.Contains(adminHTML, "params.set('limit'") {
		t.Fatal("invoice list must not use legacy limit query param")
	}
	if !strings.Contains(adminHTML, "params.set('page_size'") {
		t.Fatal("invoice list must use page_size query param")
	}
	if !strings.Contains(adminHTML, "data.total") || !strings.Contains(adminHTML, "data.gross_total_sum") {
		t.Fatal("home stats must use API total and gross_total_sum")
	}
	if strings.Count(adminHTML, `id="invoiceTypeTabs"`) != 1 {
		t.Fatal("invoiceTypeTabs must exist exactly once")
	}
	for _, dt := range []string{"FT", "FS", "NC", "ND"} {
		if !strings.Contains(adminHTML, `data-invoice-type="`+dt+`"`) {
			t.Fatalf("invoice type tab %s missing", dt)
		}
	}
	if !strings.Contains(adminHTML, "params.set('document_type'") {
		t.Fatal("invoice list must filter by document_type query param")
	}
	if !strings.Contains(adminHTML, "function setInvoiceListDocType") {
		t.Fatal("invoice list must use setInvoiceListDocType for type tabs")
	}
}
