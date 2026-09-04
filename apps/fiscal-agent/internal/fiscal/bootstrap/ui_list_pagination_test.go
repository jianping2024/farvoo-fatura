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
	if !strings.Contains(string(fiscalUIListPaginationJS), "options.getLabels") {
		t.Fatal("pagination must re-read getLabels on paint (locale switch)")
	}
	if !strings.Contains(string(fiscalUIListPaginationCSS), ".fiscal-list-pagination") {
		t.Fatal("list-pagination.css must style fiscal-list-pagination")
	}
	if !strings.Contains(string(fiscalUIListPaginationCSS), ".admin-list-panel") {
		t.Fatal("list-pagination.css must style admin-list-panel")
	}
}

func TestAdminListFilterRowUnique(t *testing.T) {
	css := string(fiscalUIListPaginationCSS)
	if n := strings.Count(css, ".admin-list-filter-row {"); n != 1 {
		t.Fatalf(".admin-list-filter-row must have exactly one rule, got %d", n)
	}
	if n := strings.Count(css, ".admin-list-filter-row {\n  display: flex;"); n != 1 {
		t.Fatal("filter-row flex must live in that single .admin-list-filter-row rule")
	}
	if !strings.Contains(css, "align-items: center;") {
		t.Fatal("filter-row must align date presets and search on one baseline")
	}
	if n := strings.Count(css, ".invoice-list-panel .admin-list-search-field {"); n != 1 {
		t.Fatalf("invoice search field toolbar rule must appear once, got %d", n)
	}
	if !strings.Contains(string(fiscalUIListPaginationJS), `root.style.display = state.total === 0 ? 'none' : ''`) {
		t.Fatal("pagination bar must hide when total is 0 (single paint path)")
	}
	if n := strings.Count(css, ".admin-list-select-field {"); n != 1 {
		t.Fatalf(".admin-list-select-field block must appear once, got %d", n)
	}
	if strings.Contains(css, "invoice-filter-row") || strings.Contains(css, "invoice-search-field") {
		t.Fatal("list-pagination.css must not keep invoice-filter-row / invoice-search-field")
	}
	if strings.Contains(adminHTML, "invoice-filter-row") || strings.Contains(adminHTML, "invoice-search-field") {
		t.Fatal("admin HTML must use admin-list-filter-row / admin-list-search-field only")
	}
	if strings.Contains(adminHTML, ".invoice-filter-row") || strings.Contains(adminHTML, ".admin-list-filter-row {") {
		t.Fatal("filter-row flex must not be restated in admin index.html")
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "auditActionLabels") {
		t.Fatal("do not add a second audit action map in admin-i18n.js")
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
	for _, id := range []string{
		`id="invoiceListPagination"`,
		`id="productsListPagination"`,
		`id="customersListPagination"`,
		`id="billsListPagination"`,
	} {
		if strings.Count(adminHTML, id) != 1 {
			t.Fatalf("%s must exist exactly once", id)
		}
	}
	if strings.Count(adminHTML, "admin-list-panel") < 4 {
		t.Fatal("browse lists must use admin-list-panel shell")
	}
	for _, fn := range []string{
		"function buildInvoicesQueryPath",
		"function buildProductsQueryPath",
		"function buildCustomersQueryPath",
		"function initInvoiceFilters",
		"function initProductListPanel",
		"function initCustomerListPanel",
		"function renderClientPaginatedTable",
		"function resetInvoiceListPage",
		"async function refreshHomeStats",
		"async function refreshInvoices",
		"async function refreshProducts",
		"async function refreshCustomers",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if !strings.Contains(adminHTML, "FiscalUI.createListPaginationBar('#invoiceListPagination'") {
		t.Fatal("invoice pagination must use FiscalUI.createListPaginationBar")
	}
	if !strings.Contains(adminHTML, "FiscalUI.createListPaginationBar('#productsListPagination'") {
		t.Fatal("products pagination must use FiscalUI.createListPaginationBar")
	}
	if strings.Contains(adminHTML, "params.set('limit'") {
		t.Fatal("invoice list must not use legacy limit query param")
	}
	if !strings.Contains(adminHTML, "params.set('page_size'") {
		t.Fatal("lists must use page_size query param")
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
