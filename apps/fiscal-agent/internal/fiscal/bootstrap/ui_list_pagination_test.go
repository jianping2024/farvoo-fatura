package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUIListPaginationAssets(t *testing.T) {
	js := string(fiscalUIListPaginationJS)
	css := string(fiscalUIListPaginationCSS)
	if !strings.Contains(js, "FiscalUI.createListPaginationBar") {
		t.Fatal("list-pagination.js must export FiscalUI.createListPaginationBar")
	}
	if n := strings.Count(js, "function createListPaginationBar"); n != 1 {
		t.Fatalf("createListPaginationBar must be defined once in list-pagination.js, got %d", n)
	}
	if !strings.Contains(js, "options.getLabels") {
		t.Fatal("pagination must re-read getLabels on paint (locale switch)")
	}
	for _, role := range []string{`data-role="first"`, `data-role="prev"`, `data-role="next"`, `data-role="last"`} {
		// once in markup template + once in querySelector
		if n := strings.Count(js, role); n != 2 {
			t.Fatalf("nav %s must appear twice in list-pagination.js (markup+query), got %d", role, n)
		}
	}
	if n := strings.Count(js, "onPageChange(1)"); n != 1 {
		t.Fatalf("first-page jump must be the ONLY onPageChange(1), got %d", n)
	}
	if n := strings.Count(js, "onPageChange(state.totalPages)"); n != 1 {
		t.Fatalf("last-page jump must be the ONLY onPageChange(state.totalPages), got %d", n)
	}
	if n := strings.Count(js, "var PAGER_ICON_SVG"); n != 1 {
		t.Fatalf("PAGER_ICON_SVG must be the ONLY pager icon map, got %d", n)
	}
	if n := strings.Count(js, "ONLY pager chevron SVGs"); n != 1 {
		t.Fatalf("pager SVG map must be documented once, got %d", n)
	}
	if n := strings.Count(js, `fiscal-list-pagination__icon-btn`); n != 4 {
		t.Fatalf("nav markup must use __icon-btn exactly 4 times (first/prev/next/last), got %d", n)
	}
	if strings.Contains(js, "firstBtn.textContent") || strings.Contains(js, "prevBtn.textContent") ||
		strings.Contains(js, "nextBtn.textContent") || strings.Contains(js, "lastBtn.textContent") {
		t.Fatal("pager nav must not set button textContent; labels are aria-label + title only")
	}
	if n := strings.Count(js, `firstBtn.setAttribute('aria-label'`); n != 1 {
		t.Fatalf("first aria-label must be set once in paint, got %d", n)
	}
	if n := strings.Count(js, `firstBtn.setAttribute('title'`); n != 1 {
		t.Fatalf("first title must be set once in paint, got %d", n)
	}
	if !strings.Contains(css, ".fiscal-list-pagination") {
		t.Fatal("list-pagination.css must style fiscal-list-pagination")
	}
	if !strings.Contains(css, ".admin-list-panel") {
		t.Fatal("list-pagination.css must style admin-list-panel")
	}
	if n := strings.Count(css, "ONLY pager nav icon buttons"); n != 1 {
		t.Fatalf("pager icon-btn CSS must be the ONLY documented rule, got %d", n)
	}
	if n := strings.Count(css, "button.fiscal-list-pagination__icon-btn {"); n != 1 {
		t.Fatalf("pager __icon-btn size rule must appear once, got %d", n)
	}
	if strings.Contains(css, ".fiscal-list-pagination__nav button {\n  padding: 0.4rem 0.75rem") {
		t.Fatal("pager nav must not keep text-button padding rule")
	}
}

func TestAdminListToolbarUnique(t *testing.T) {
	css := string(fiscalUIListPaginationCSS)
	if n := strings.Count(css, ".admin-list-toolbar {"); n != 1 {
		t.Fatalf(".admin-list-toolbar must have exactly one rule, got %d", n)
	}
	if n := strings.Count(css, "button.btn-toolbar {"); n != 1 {
		t.Fatalf(".btn-toolbar must have exactly one rule, got %d", n)
	}
	if n := strings.Count(css, "button.btn-toolbar--icon {"); n != 1 {
		t.Fatalf(".btn-toolbar--icon must have exactly one rule, got %d", n)
	}
	if strings.Contains(css, "btn-icon-refresh") {
		t.Fatal("btn-icon-refresh must be removed; use btn-toolbar btn-toolbar--icon")
	}
	if n := strings.Count(css, ".admin-list-filter-panel {"); n != 1 {
		t.Fatalf(".admin-list-filter-panel must have exactly one rule, got %d", n)
	}
	if n := strings.Count(css, ".admin-list-filter-panel__row {"); n != 1 {
		t.Fatalf(".admin-list-filter-panel__row must have exactly one rule, got %d", n)
	}
	if strings.Contains(css, "admin-list-filter-panel__footer") {
		t.Fatal("filter panel must not keep empty __footer row")
	}
	if n := strings.Count(css, "/* ONLY sticky last-column actions"); n != 1 {
		t.Fatalf("sticky col-actions must be the ONLY sticky actions block, got %d", n)
	}
	if n := strings.Count(css, "position: sticky;\n  right: 0;"); n != 1 {
		t.Fatalf("sticky right:0 for actions must appear once, got %d", n)
	}
	if n := strings.Count(css, ".admin-list-filter-row {"); n != 1 {
		t.Fatalf(".admin-list-filter-row must have exactly one rule, got %d", n)
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
	if strings.Contains(css, ".invoice-list-panel .admin-list-search-field") {
		t.Fatal("invoice search must use admin-list-toolbar__search only")
	}
	if strings.Contains(adminHTML, "field admin-list-toolbar__search") {
		t.Fatal("toolbar search must not use .field (form margin skews peer buttons)")
	}
	if strings.Contains(adminHTML, "invoice-filter-row") || strings.Contains(adminHTML, "invoice-search-field") {
		t.Fatal("admin HTML must not keep legacy invoice-filter-row")
	}
	if strings.Contains(adminHTML, ".admin-list-toolbar {") || strings.Contains(adminHTML, ".admin-list-filter-row {") {
		t.Fatal("list toolbar/filter-row flex must not be restated in admin index.html")
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "auditActionLabels") {
		t.Fatal("do not add a second audit action map in admin-i18n.js")
	}
	for _, id := range []string{
		`id="btnRefreshInvoices"`,
		`id="btnRefreshBills"`,
		`id="btnRefreshProducts"`,
		`id="btnRefreshCustomers"`,
		`id="btnOperatorsRefresh"`,
		`id="btnAuditRefresh"`,
		`id="btnTerminalsRefresh"`,
		`id="btnSaftRefresh"`,
	} {
		if strings.Count(adminHTML, id) != 1 {
			t.Fatalf("%s must exist exactly once", id)
		}
	}
	if strings.Count(adminHTML, `class="btn-toolbar btn-toolbar--icon"`) < 8 {
		t.Fatal("list refresh controls must use btn-toolbar btn-toolbar--icon")
	}
	if strings.Contains(adminHTML, "btn-icon-refresh") {
		t.Fatal("admin must not keep btn-icon-refresh")
	}
	if strings.Count(adminHTML, `id="btnInvoiceFilter" class="btn-toolbar btn-filter-toggle"`) != 1 &&
		strings.Count(adminHTML, `class="btn-toolbar btn-filter-toggle" id="btnInvoiceFilter"`) != 1 {
		t.Fatal("invoice filter must be btn-toolbar (not bare secondary)")
	}
	if strings.Contains(adminHTML, `class="secondary btn-filter-toggle"`) {
		t.Fatal("filter must not use secondary CTA padding in toolbar")
	}
	if strings.Contains(adminHTML, `id="btnRefreshInvoices" data-i18n="common.refresh">刷新</button>`) {
		t.Fatal("invoice refresh must be icon-only in list toolbar, not topbar text 刷新")
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
		`id="invoiceFilterPanel"`,
		`id="btnInvoiceFilter"`,
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
		"function setInvoiceFilterPanelOpen",
		"async function refreshHomeStats",
		"async function refreshInvoices",
		"async function refreshProducts",
		"async function refreshCustomers",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if !strings.Contains(adminHTML, "fiscal_invoice_date_range_v2") {
		t.Fatal("invoice date storage key must bump to v2 (drop sticky yesterday)")
	}
	if !strings.Contains(adminHTML, "fiscal_invoice_doc_type_v2") {
		t.Fatal("invoice doc-type storage key must bump to v2 (default all)")
	}
	if !strings.Contains(adminHTML, `data-invoice-type="" role="tab"`) {
		t.Fatal("invoice tabs must include 全部 (empty document_type)")
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
	if strings.Contains(adminHTML, "btnInvoiceFilterApply") || strings.Contains(adminHTML, "apply_filters") {
		t.Fatal("invoice filter panel must not keep redundant Apply (presets + date-range Apply suffice)")
	}
	if strings.Count(adminHTML, `id="btnInvoiceFilterClear"`) != 1 {
		t.Fatal("invoice filter Clear must exist exactly once (same row as dates)")
	}
	if !strings.Contains(adminHTML, `class="admin-list-filter-panel__row"`) {
		t.Fatal("invoice filter must use __row (dates + Clear), not footer")
	}
	if strings.Contains(adminHTML, "admin-list-filter-panel__footer") {
		t.Fatal("admin HTML must not keep filter __footer")
	}
	if n := strings.Count(adminHTML, "/* ONLY admin surface card chrome"); n != 1 {
		t.Fatalf("surface card chrome must be documented once, got %d", n)
	}
	if n := strings.Count(adminHTML, ".home-topbar,\n    .panel {"); n != 1 {
		t.Fatalf("home-topbar+panel card chrome must be a single shared rule, got %d", n)
	}
	if strings.Contains(adminHTML, ".home-topbar {\n      height: 52px; background: rgba") {
		t.Fatal("home-topbar must not keep sticky glass strip chrome")
	}
	if strings.Count(adminHTML, ".home-topbar {\n      display: flex") != 1 {
		t.Fatal("home-topbar layout rule must appear once (chrome shared with .panel)")
	}
	if strings.Count(adminHTML, ".panel {\n      padding: 1.1rem 1.25rem") != 1 {
		t.Fatal("panel layout padding must appear once (chrome shared with home-topbar)")
	}
	if !strings.Contains(adminHTML, ".hub-metric {\n      display: inline-flex; align-items: center; gap: 0.65rem;\n      min-height: 2.75rem; padding: 0.5rem 0.9rem;\n      background: #fff; border: 1px solid var(--line); border-radius: var(--radius);\n      box-shadow: var(--shadow);\n    }") {
		t.Fatal("hub-metric must use var(--radius) + shadow once (no third hardcoded 10px radius)")
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

func TestAdminNativeSelectChevronUnique(t *testing.T) {
	if n := strings.Count(adminHTML, "ONLY native select disclosure"); n != 1 {
		t.Fatalf("native select chevron must be the ONLY Admin rule, got %d", n)
	}
	if n := strings.Count(adminHTML, "\n      appearance: none;"); n != 1 {
		t.Fatalf("appearance: none for select must appear once, got %d", n)
	}
	if n := strings.Count(adminHTML, "\n      -webkit-appearance: none;"); n != 1 {
		t.Fatalf("-webkit-appearance: none must appear once, got %d", n)
	}
	if n := strings.Count(adminHTML, "background-image: url(\"data:image/svg+xml"); n != 1 {
		t.Fatalf("select chevron SVG must be defined once, got %d", n)
	}
	if n := strings.Count(adminHTML, "--select-chevron-slot:"); n != 1 {
		t.Fatalf("--select-chevron-slot must be declared once, got %d", n)
	}
	if n := strings.Count(adminHTML, "--select-pad-x:"); n != 1 {
		t.Fatalf("--select-pad-x must be declared once, got %d", n)
	}
	css := string(fiscalUIListPaginationCSS)
	if strings.Contains(css, "appearance:") || strings.Contains(css, "background-image:") {
		t.Fatal("list-pagination.css must not redefine select chevron chrome")
	}
	if strings.Contains(css, "padding: 0.35rem 0.5rem") {
		t.Fatal("pagination size select must not use padding shorthand that resets chevron slot")
	}
	if !strings.Contains(css, "Keep Admin select padding-right / chevron") {
		t.Fatal("pagination size select must document that chevron padding stays from Admin")
	}
}
