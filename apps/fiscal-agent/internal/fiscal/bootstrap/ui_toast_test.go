package bootstrap

import (
	"strings"
	"testing"
)

func TestAdminHTMLSeriesRegisterSinglePath(t *testing.T) {
	if !strings.Contains(adminHTML, "ONLY Admin path to POST /local/v1/setup/series/register") {
		t.Fatal("admin must document single registerSeries path")
	}
	if !strings.Contains(adminHTML, "function registerSeries(") {
		t.Fatal("admin must define registerSeries")
	}
	if !strings.Contains(adminHTML, "function applySeriesForm(") {
		t.Fatal("admin must define applySeriesForm")
	}
	// No second inline withBusy register for FT/NC (must go through registerSeries).
	if strings.Count(adminHTML, "POST', '/local/v1/setup/series/register'") != 1 {
		t.Fatalf("want exactly one series/register call site in admin, got %d",
			strings.Count(adminHTML, "POST', '/local/v1/setup/series/register'"))
	}
}

func TestAdminHTMLUsesSharedToastOnly(t *testing.T) {
	if !strings.Contains(adminHTML, `/fiscal-ui/toast.js`) {
		t.Fatal("admin must load /fiscal-ui/toast.js")
	}
	if !strings.Contains(adminHTML, `/fiscal-ui/toast.css`) {
		t.Fatal("admin must load /fiscal-ui/toast.css")
	}
	if strings.Contains(adminHTML, `function showToast`) {
		t.Fatal("do not redefine showToast in admin HTML; use FiscalUI.showToast")
	}
	if strings.Contains(adminHTML, `id="flash"`) || strings.Contains(adminHTML, `fiscal-toast-root`) {
		t.Fatal("do not embed toast/flash markup in admin; FiscalUI.showToast owns the root")
	}
	if !strings.Contains(string(fiscalUIToastJS), `FiscalUI.showToast`) {
		t.Fatal("toast.js must export FiscalUI.showToast")
	}
	if !strings.Contains(string(fiscalUIToastJS), `FiscalUI.reportError`) {
		t.Fatal("toast.js must export FiscalUI.reportError")
	}
	if !strings.Contains(adminHTML, `const reportError = FiscalUI.reportError`) {
		t.Fatal("admin must bind FiscalUI.reportError")
	}
	if strings.Contains(adminHTML, `toast(errText`) {
		t.Fatal("do not toast(errText); use reportError (single error-toast path)")
	}
	if strings.Contains(adminHTML, `station_id required`) {
		t.Fatal("requireStation must throw i18n message, not English station_id required")
	}
	if strings.Contains(adminHTML, `reportError(er); throw`) || strings.Contains(adminHTML, "reportError(er);\n      throw") {
		t.Fatal("do not reportError then throw (withBusy already reports; would double-toast)")
	}
	if n := strings.Count(adminHTML, "async function withBusy"); n != 1 {
		t.Fatalf("withBusy must appear exactly once, got %d", n)
	}
	wbIdx := strings.Index(adminHTML, "async function withBusy")
	wbEnd := strings.Index(adminHTML[wbIdx+1:], "\n  async function ")
	if wbEnd < 0 {
		wbEnd = strings.Index(adminHTML[wbIdx+1:], "\n  function ")
	}
	if wbEnd < 0 {
		t.Fatal("cannot bound withBusy")
	}
	wbBody := adminHTML[wbIdx : wbIdx+1+wbEnd]
	if strings.Count(wbBody, "reportError(er)") != 1 {
		t.Fatalf("withBusy must call reportError exactly once, got %d", strings.Count(wbBody, "reportError(er)"))
	}
	if !strings.Contains(adminHTML, "function requireStation") {
		t.Fatal("admin must define requireStation")
	}
	// requireStation must throw-only (no toast before throw).
	reqIdx := strings.Index(adminHTML, "function requireStation")
	reqEnd := strings.Index(adminHTML[reqIdx:], "\n  function ")
	if reqEnd < 0 {
		t.Fatal("cannot bound requireStation")
	}
	body := adminHTML[reqIdx : reqIdx+reqEnd]
	if strings.Contains(body, "toast(") {
		t.Fatal("requireStation must not toast; throw only (withBusy/reportError speaks)")
	}
	if strings.Count(string(fiscalUIToastCSS), "max-width: calc(100% - 2rem)") != 1 {
		t.Fatalf("toast max-width 100 percent writing must appear once, got %d",
			strings.Count(string(fiscalUIToastCSS), "max-width: calc(100% - 2rem)"))
	}
	if strings.Contains(string(fiscalUIToastCSS), "100vw") {
		t.Fatal("toast.css must not use 100vw")
	}
}

func TestAdminHTMLSplitSyncScopeHelper(t *testing.T) {
	if !strings.Contains(adminHTML, "function isSplitSyncScope") {
		t.Fatal("admin must define isSplitSyncScope for split bill-sync payloads")
	}
	if !strings.Contains(adminHTML, "function syncScopeType") {
		t.Fatal("admin must define syncScopeType helper")
	}
	if strings.Contains(adminHTML, "scopeType === 'person'") {
		t.Fatal("admin must not compare sync scope_type to 'person'; cloud sends split, issue mode is person")
	}
}

func TestAdminHTMLBillListAndCustomerNifHelpers(t *testing.T) {
	if !strings.Contains(adminHTML, "function billSyncPayloadAmount") {
		t.Fatal("admin must define billSyncPayloadAmount for bill list/detail amounts")
	}
	if !strings.Contains(adminHTML, "function assertCustomerNifOrToast") {
		t.Fatal("admin must define assertCustomerNifOrToast for NIF validation")
	}
	if !strings.Contains(adminHTML, "function renderCustomerNifDatalist") {
		t.Fatal("admin must define renderCustomerNifDatalist once for customer lookup")
	}
	if !strings.Contains(adminHTML, "function bindCustomerNifAutofill") {
		t.Fatal("admin must define bindCustomerNifAutofill as the ONLY NIF field binder")
	}
	if n := strings.Count(adminHTML, "function validatePortugueseNif"); n != 1 {
		t.Fatalf("validatePortugueseNif (Mod-11) must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function bindCustomerNifAutofill"); n != 1 {
		t.Fatalf("bindCustomerNifAutofill must appear exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "function isNineDigitNif") {
		t.Fatal("remove isNineDigitNif; Mod-11 validatePortugueseNif is the ONLY NIF check")
	}
	if strings.Contains(adminHTML, `/^\d{9}$/`) {
		t.Fatal("must not use only-9-digit regex as sole NIF check")
	}
	if !strings.Contains(adminHTML, `id="splitNif"`) || !strings.Contains(adminHTML, `id="view-bill-split"`) {
		t.Fatal("bill split workbench must use splitNif on view-bill-split")
	}
	if strings.Contains(adminHTML, `id="billBuyerShared"`) || strings.Contains(adminHTML, `id="billNif"`) {
		t.Fatal("legacy billDetail NIF UI must be removed")
	}
	if !strings.Contains(adminHTML, `list="customerNifList"`) {
		t.Fatal("NIF inputs must use shared customerNifList datalist")
	}
	if !strings.Contains(adminHTML, "addEventListener('blur'") || !strings.Contains(adminHTML, "customerNifInputError(nifEl.value)") {
		t.Fatal("NIF fields must validate on blur via customerNifInputError (not only on issue click)")
	}
	if strings.Contains(adminHTML, "invNif').addEventListener('change'") {
		t.Fatal("do not bind invNif change separately; use bindCustomerNifAutofill")
	}
	if strings.Contains(adminHTML, "splitNif').addEventListener('blur'") || strings.Contains(adminHTML, "cNif').addEventListener('blur'") {
		t.Fatal("do not bind NIF blur ad-hoc; use bindCustomerNifAutofill only")
	}
	if !strings.Contains(adminHTML, `id="splitPay"`) {
		t.Fatal("bill split must expose splitPay (same payment methods as order)")
	}
	if n := strings.Count(adminHTML, "function suggestedSaleDocumentType"); n != 1 {
		t.Fatalf("suggestedSaleDocumentType must be the ONLY soft FT/FS rule, got %d", n)
	}
	if n := strings.Count(adminHTML, "function syncSaleDocTypeSuggestion"); n != 1 {
		t.Fatalf("syncSaleDocTypeSuggestion must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function bindSaleDocTypeSuggestion"); n != 1 {
		t.Fatalf("bindSaleDocTypeSuggestion must be the ONLY soft FT/FS binder, got %d", n)
	}
	if n := strings.Count(adminHTML, "bindSaleDocTypeSuggestion($('#docType'"); n != 1 {
		t.Fatalf("order panel must bind soft FT/FS once, got %d", n)
	}
	if n := strings.Count(adminHTML, "bindSaleDocTypeSuggestion($('#splitDocType'"); n != 1 {
		t.Fatalf("split panel must bind soft FT/FS once, got %d", n)
	}
	if strings.Contains(adminHTML, "invNif').addEventListener('input'") || strings.Contains(adminHTML, "splitNif').addEventListener('input'") {
		t.Fatal("do not bind NIF input ad-hoc for doc type; use bindSaleDocTypeSuggestion only")
	}
	if strings.Contains(adminHTML, "'<td>—</td>' +\n        '<td>' + fmtTimeShort") {
		t.Fatal("bill list must not hardcode amount column to em dash")
	}
}

func TestAdminHTMLSchemeACopyUnique(t *testing.T) {
	// Scheme A: source-based labels; forbid old primary nav/CTA names.
	forbidden := []string{
		">订单<small>",
		">待开票账单",
		"＋ 新建订单",
		"处理待开票账单",
		"新建订单",
		"有新的待开票账单",
		"暂无待开票账单",
		"创建订单",
		"确认订单",
	}
	for _, s := range forbidden {
		if strings.Contains(adminHTML, s) {
			t.Fatalf("scheme A forbids leftover UI copy %q", s)
		}
	}
	requiredOnce := []string{
		`data-i18n="nav.invoices">发票</span>`,
		`data-i18n="nav.bills">收银账单</span>`,
		`id="homeGreeting"`,
		`id="operatorName"`,
		`data-i18n="home.cta.new_order"`,
		`data-i18n="home.stat.bills_cta"`,
		`FiscalAdminI18n.t('bills.toast.new')`,
		`FiscalAdminI18n.t('bills.empty')`,
		`id="uiLocaleSelect"`,
		`/fiscal-ui/admin-i18n.js`,
	}
	forbiddenLeftovers := []string{
		"view-home", "view-orders", "fiscal_orders", "order.issue_later",
		"nav.home", "nav.orders", "home.title", "home.recent",
		"statPendingOrders", "homeRecentInvoicesTbody", "btnOrderInvoiceBack",
		"renderOrdersList", "loadOrders", "data-nav=\"home\"", "data-nav=\"orders\"",
		"showView('home')", "手工待开", "稍后开票",
	}
	for _, s := range forbiddenLeftovers {
		if strings.Contains(adminHTML, s) {
			t.Fatalf("invoice hub must not retain leftover %q", s)
		}
	}
	for _, s := range requiredOnce {
		if n := strings.Count(adminHTML, s); n != 1 {
			t.Fatalf("scheme A unique copy %q must appear exactly once, got %d", s, n)
		}
	}
	if n := strings.Count(adminHTML, "创建开票"); n != 2 {
		t.Fatalf("创建开票 must appear twice (two flow bars), got %d", n)
	}
	if strings.Count(adminHTML, "新建开票") < 2 {
		t.Fatal("新建开票 must appear on CTA and order surfaces")
	}
}

func TestAdminHTMLNoPageXScrollUnique(t *testing.T) {
	if n := strings.Count(adminHTML, "overflow-x: hidden"); n != 1 {
		t.Fatalf("page overflow-x:hidden must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "224px minmax(0, 1fr)"); n != 1 {
		t.Fatalf("shell columns must appear exactly once, got %d", n)
	}
	if !strings.Contains(adminHTML, ".main { padding: 1.5rem 1.75rem 2.5rem; max-width: 980px; min-width: 0; }") {
		t.Fatal("main must shrink with min-width: 0")
	}
	if strings.Contains(adminHTML, "100vw") {
		t.Fatal("admin must not use 100vw (causes page x-scroll)")
	}
}

func TestAdminHTMLHomeWorkbenchUnique(t *testing.T) {
	for _, fn := range []string{
		"function renderHomeDateChip",
		"function renderHomeGreeting",
		"function invoiceRowActionsHtml",
		"function listRowMenuTriggerHtml",
		"function renderHomeStats",
		"function renderPendingBillsStat",
		"function invoiceListEmptyCellHtml",
		"function refreshHomeStats",
		"function focusHomePrimaryCta",
		"function cancelManualOrderFlow",
		"function formatInvoiceNoCell",
		"function formatInvoiceWhenCell",
		"function formatInvoiceOrderCell",
		"function formatInvoicePaymentLabel",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	for _, id := range []string{
		`id="homeDateChip"`,
		`id="homeGreeting"`,
		`id="ctaNewOrder"`,
		`id="ctaPendingBills"`,
		`id="statTodayInvoices"`,
		`id="statPendingBills"`,
		`id="statTodaySales"`,
		`id="btnOrderNewCancel"`,
		`id="btnOrderInvoiceCancel"`,
	} {
		if n := strings.Count(adminHTML, id); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", id, n)
		}
	}
	if n := strings.Count(adminHTML, `id="homeGreeting"`); n != 1 {
		t.Fatalf("homeGreeting must appear exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, `data-i18n="nav.invoices">发票</h1>`) {
		t.Fatal("invoice topbar must use welcome greeting, not nav.invoices title")
	}
	if strings.Contains(adminHTML, "#view-invoices { margin: -1.5rem -1.75rem 0; }") {
		t.Fatal("invoice view must not use negative-margin bleed (topbar wider than list)")
	}
	if strings.Contains(adminHTML, `class="cta-big" id="ctaNewOrder"`) || strings.Contains(adminHTML, `class="cta-big rest-only" id="ctaPendingBills"`) {
		t.Fatal("home CTAs must not use leftover cta-big")
	}
	if strings.Count(adminHTML, `data-row-kind="invoice"`) != 1 {
		t.Fatal("invoice ⋯ trigger markup must exist only in invoiceRowActionsHtml")
	}
	if strings.Contains(adminHTML, `data-reprint="' + documentId + '"`) {
		t.Fatal("invoice list must not inline data-reprint buttons; use invoiceRowActionsHtml ⋯")
	}
	if strings.Contains(adminHTML, "$('#homeDateChip').textContent") {
		t.Fatal("do not write home date outside renderHomeDateChip")
	}
	if strings.Contains(adminHTML, "$('#homeGreeting').textContent") {
		t.Fatal("do not write home greeting outside renderHomeGreeting")
	}
	if strings.Contains(adminHTML, "stat-row") || strings.Contains(adminHTML, "stat-icon") || strings.Contains(adminHTML, "stat-cta-label") {
		t.Fatal("invoice hub must not keep tall KPI card styles/markup")
	}
	if strings.Count(adminHTML, "+ formatInvoiceNoCell(inv)") != 1 {
		t.Fatal("invoice_no cells must call formatInvoiceNoCell only in invoice list")
	}
	if strings.Count(adminHTML, "formatInvoiceNoCell(inv)") != 2 {
		t.Fatal("formatInvoiceNoCell(inv) must appear only in def and invoice list")
	}
	if strings.Count(adminHTML, "+ formatInvoiceWhenCell(inv)") != 1 {
		t.Fatal("issued-at cells must call formatInvoiceWhenCell only in invoice list")
	}
	if strings.Count(adminHTML, "+ formatInvoiceOrderCell(inv)") != 1 {
		t.Fatal("order/source cells must call formatInvoiceOrderCell only in invoice list")
	}
	if strings.Count(adminHTML, "+ formatInvoicePaymentLabel(inv)") != 1 {
		t.Fatal("payment cells must call formatInvoicePaymentLabel only in invoice list")
	}
	if strings.Count(adminHTML, "formatInvoicePaymentLabel(inv)") != 3 {
		t.Fatal("formatInvoicePaymentLabel(inv) must appear only in def, invoice list, and detail")
	}
	if strings.Contains(adminHTML, "inv.order_label || '—'") {
		t.Fatal("do not inline order_label fallback; use formatInvoiceOrderCell")
	}
	if strings.Contains(adminHTML, "inv.invoice_no || inv.document_type") {
		t.Fatal("do not fall back invoice_no to document_type in list cells")
	}
	if strings.Contains(adminHTML, "$('#statPendingBills').textContent") {
		t.Fatal("do not write pending bills count outside renderPendingBillsStat")
	}
	if strings.Count(adminHTML, "invoiceListEmptyCellHtml(") != 2 {
		t.Fatal("invoiceListEmptyCellHtml must appear only in def and refreshInvoices")
	}
}

func TestAdminHTMLInvoiceHubLayout(t *testing.T) {
	start := strings.Index(adminHTML, `id="view-invoices"`)
	if start < 0 {
		t.Fatal("missing view-invoices")
	}
	end := strings.Index(adminHTML[start:], `id="view-products"`)
	if end < 0 {
		t.Fatal("cannot bound view-invoices section")
	}
	section := adminHTML[start : start+end]
	if !strings.Contains(section, `class="home-topbar"`) {
		t.Fatal("invoice hub must use home-topbar chrome")
	}
	if !strings.Contains(section, `id="homeDateChip"`) {
		t.Fatal("invoice hub must include date chip")
	}
	if !strings.Contains(section, `id="homeGreeting"`) || !strings.Contains(section, `class="home-greeting"`) {
		t.Fatal("invoice topbar must include welcome greeting")
	}
	if strings.Contains(section, `data-i18n="nav.invoices">发票</h1>`) {
		t.Fatal("invoice topbar must not use nav.invoices as title")
	}
	if !strings.Contains(section, `class="hub-entry-strip"`) {
		t.Fatal("invoice hub must use compact hub-entry-strip")
	}
	if !strings.Contains(section, `class="hub-bills-cta rest-only"`) {
		t.Fatal("pending bills CTA must be hub-bills-cta (peer primary), not tall stat card")
	}
	if !strings.Contains(section, `id="statTodayInvoices"`) {
		t.Fatal("invoice hub must include today invoice metric")
	}
	if strings.Contains(section, `statPendingOrders`) || strings.Contains(section, "home-recent") {
		t.Fatal("invoice hub must not include pending orders or home-recent mini-table")
	}
	if !strings.Contains(section, `data-i18n="doc.tab.FT">完整发票</span><small>FT</small>`) {
		t.Fatal("invoice type tabs must show business label first, abbr second")
	}
	if strings.Contains(section, `>FT<small data-i18n="doc.tab.FT"`) {
		t.Fatal("do not put FT abbr before business label on type tabs")
	}
	for _, h := range []string{
		`class="col-when" data-i18n="col.issued_at"`,
		`class="col-no" data-i18n="col.doc_no"`,
		`class="col-buyer" data-i18n="col.buyer"`,
		`class="col-source" data-i18n="col.source"`,
		`class="col-pay" data-i18n="pay.label"`,
		`class="col-money" data-i18n="col.amount"`,
		`class="col-actions"`,
	} {
		if n := strings.Count(section, h); n != 1 {
			t.Fatalf("invoice list header %q must appear once in hub, got %d", h, n)
		}
	}
	if strings.Contains(section, `class="col-actions" data-i18n="common.actions"`) {
		t.Fatal("invoice actions header must be blank (⋯ column), not labeled 操作")
	}
	for _, css := range []string{
		".invoices-table.list-table { table-layout: fixed; width: 100%; }",
		".invoices-table.list-table .col-no { width: 10rem; white-space: nowrap; }",
		".invoices-table.list-table .col-pay {",
	} {
		if n := strings.Count(adminHTML, css); n != 1 {
			t.Fatalf("clerk invoice layout CSS %q must appear exactly once, got %d", css, n)
		}
	}
	if strings.Contains(adminHTML, "function applyHomeRecentColNoWidth") || strings.Contains(adminHTML, "function measureAdminTableTextPx") {
		t.Fatal("home-recent column measure helpers must be removed")
	}
	if !strings.Contains(adminHTML, "formatInvoiceBuyerCell(inv)") || !strings.Contains(adminHTML, "formatInvoiceOrderCell(inv)") {
		t.Fatal("invoice list must render 购方/来源 via shared formatters")
	}
	if !strings.Contains(adminHTML, "formatInvoicePaymentLabel(inv)") {
		t.Fatal("invoice list must render 付款方式 via formatInvoicePaymentLabel")
	}
}

func TestAdminHTMLHomeCtaPeerAndFocus(t *testing.T) {
	if !strings.Contains(adminHTML, `id="ctaNewOrder"`) || !strings.Contains(adminHTML, `id="ctaPendingBills"`) {
		t.Fatal("home must expose ctaNewOrder and ctaPendingBills")
	}
	if !strings.Contains(adminHTML, "function focusHomePrimaryCta") {
		t.Fatal("admin must define focusHomePrimaryCta once")
	}
	if !strings.Contains(adminHTML, "showView('invoices')") {
		t.Fatal("login/enterApp must default to invoices view")
	}
	idx := strings.Index(adminHTML, `id="ctaPendingBills"`)
	if idx < 0 {
		t.Fatal("missing ctaPendingBills")
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + 120
	if end > len(adminHTML) {
		end = len(adminHTML)
	}
	snippet := adminHTML[start:end]
	if strings.Contains(snippet, "secondary") || strings.Contains(snippet, "border-color:var(--line)") {
		t.Fatalf("pending bills CTA must not be demoted: %s", snippet)
	}
}

func TestAdminHTMLProductWorkbenchUnique(t *testing.T) {
	if strings.Contains(adminHTML, `id="btnAddLine"`) || strings.Contains(adminHTML, "添加一行") {
		t.Fatal("remove btnAddLine / 添加一行; order lines come from product workbench only")
	}
	for _, fn := range []string{
		"function openProductWorkbench",
		"function filterCatalogProducts",
		"function saveProductFromFields",
		"function addProductToCurrentOrder",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Count(adminHTML, `id="pCode"`) != 1 {
		t.Fatal("product fields must exist exactly once (shared workbench)")
	}
	if strings.Count(adminHTML, `id="btnPickProduct"`) != 1 {
		t.Fatal("order CTA btnPickProduct must exist exactly once")
	}
	if !strings.Contains(adminHTML, `id="pickerSearch"`) {
		t.Fatal("product workbench must include pickerSearch")
	}
	if strings.Contains(adminHTML, `id="productForm"`) {
		t.Fatal("inline productForm panel must not return; use productFormFields in workbench")
	}
}

func TestAdminHTMLBillDraftEventsUnique(t *testing.T) {
	for _, fn := range []string{
		"function updateBillsNavBadge",
		"function startBillDraftEvents",
		"function stopBillDraftEvents",
		"function validatePortugueseNif",
		"function openBillDetail",
		"function renderBillSplitWorkbench",
		"function mapAllocPeople",
		"function selectSplitPerson",
		"function applySplitMarkerNameFromInput",
		"function buildAllocationForCommit",
		"function commitSplitAllocationForIssue",
		"function discardSplitLocalEdits",
		"function applySplitDetailAllocation",
		"function issueBill",
		"function localRemainingMap",
		"function paintSplitPoolAndHint",
		"function splitOverAllocMessages",
		"function assertSplitNotOverAllocated",
		"function onSplitQtyEdited",
		"function syncCurrentSharesFromDom",
		"function coalescePersonShares",
		"function upsertPersonShare",
		"function shareLineGrossAmount",
		"function personGrossPreview",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Contains(adminHTML, "此人已有该菜") {
		t.Fatal("reject-on-duplicate share must be removed; upsertPersonShare merges qty")
	}
	if strings.Count(adminHTML, `id="splitPersonTotal"`) != 1 {
		t.Fatal("splitPersonTotal must exist exactly once")
	}
	if strings.Contains(adminHTML, "cur.shares.push({ line_key: key") {
		t.Fatal("pool add must not push duplicate shares; use upsertPersonShare only")
	}
	if strings.Count(adminHTML, "new EventSource('/local/v1/events')") != 1 {
		t.Fatal("Admin must open EventSource /local/v1/events exactly once")
	}
	if strings.Contains(adminHTML, "setInterval(() =>") || strings.Contains(adminHTML, "setInterval(function") {
		t.Fatal("do not poll refreshBills via setInterval; SSE is the primary path")
	}
	if !strings.Contains(adminHTML, `id="navBillsBadge"`) {
		t.Fatal("sidebar must include navBillsBadge")
	}
	if strings.Count(adminHTML, "addEventListener('bill_drafts_changed'") != 1 {
		t.Fatal("must listen for bill_drafts_changed exactly once")
	}
	if strings.Contains(adminHTML, "function isNineDigitNif") {
		t.Fatal("replace isNineDigitNif with validatePortugueseNif Mod-11")
	}
	if strings.Count(adminHTML, `id="view-bill-split"`) != 1 {
		t.Fatal("bill split must be dedicated main view view-bill-split")
	}
	if strings.Count(adminHTML, `id="splitAllocHint"`) != 1 {
		t.Fatal("over-alloc live hint splitAllocHint must exist exactly once")
	}
	if strings.Contains(adminHTML, `id="billDetail"`) {
		t.Fatal("legacy billDetail under list must be removed")
	}
	if strings.Contains(adminHTML, `id="btnSplitAddPerson"`) || strings.Contains(adminHTML, `id="btnSplitSaveAlloc"`) {
		t.Fatal("remove Add/Save split buttons; marker name + issue-commit only")
	}
	if strings.Contains(adminHTML, "function saveSplitAllocation") {
		t.Fatal("saveSplitAllocation removed; commitSplitAllocationForIssue is the ONLY allocation PUT")
	}
	if !strings.Contains(adminHTML, "/allocation") || !strings.Contains(adminHTML, "allocation_revision") {
		t.Fatal("split workbench must commit allocation and pass allocation_revision on person issue")
	}
	if strings.Contains(adminHTML, "function remainingMap") {
		t.Fatal("remove remainingMap; localRemainingMap is the ONLY pool calculator")
	}
	if strings.Count(adminHTML, "b.disabled = !!done") != 0 {
		t.Fatal("issued chips must stay clickable (read-only); do not disable")
	}
}

func TestAdminHTMLInvoiceListColumnsUnique(t *testing.T) {
	start := strings.Index(adminHTML, `id="view-invoices"`)
	if start < 0 {
		t.Fatal("missing view-invoices")
	}
	end := strings.Index(adminHTML[start:], `id="view-products"`)
	if end < 0 {
		t.Fatal("cannot bound view-invoices section")
	}
	section := adminHTML[start : start+end]

	requiredHeaders := []string{
		`class="col-when" data-i18n="col.issued_at"`,
		`class="col-no" data-i18n="col.doc_no"`,
		`class="col-buyer" data-i18n="col.buyer"`,
		`class="col-source" data-i18n="col.source"`,
		`class="col-pay" data-i18n="pay.label"`,
		`class="col-money" data-i18n="col.amount"`,
	}
	for _, h := range requiredHeaders {
		if n := strings.Count(section, h); n != 1 {
			t.Fatalf("invoice list header %q must appear exactly once in view-invoices, got %d", h, n)
		}
	}
	for _, forbidden := range []string{
		"<th>发票日</th>", "<th>打印状态</th>", ">时间</th>",
		"<th>上张 Hash</th>", "<th>Hash</th>", "<th>ATCUD</th>",
		"<th>类型</th>", "<th>单据状态</th>", "<th>购方 NIF</th>", "<th>购方名称</th>",
		"Hash 入参可见",
	} {
		if strings.Contains(section, forbidden) {
			t.Fatalf("invoice list must not include %q in main table", forbidden)
		}
	}
	for _, fn := range []string{
		"function truncateHash",
		"function formatInvoiceBuyerCell",
		"function renderInvoiceDetailModal",
		"function openInvoiceDetail",
		"function syncOrderInvoicePanel",
		"function openAdjustModal",
		"function invoiceCanCredit",
		"function invoiceCanDebit",
		"async function ensureSetupStatus",
		"async function refreshInvoices",
		"async function reprintInvoice",
		"function handleReprintClick",
		"async function creditInvoice",
		"async function debitInvoice",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Contains(adminHTML, `id="creditModal"`) || strings.Contains(adminHTML, "prompt(") {
		t.Fatal("creditModal and prompt() debit path must be removed; use adjustModal only")
	}
	if strings.Count(adminHTML, `id="adjustModal"`) != 1 {
		t.Fatal("adjustModal must exist exactly once")
	}
	if strings.Count(adminHTML, `id="btnOrderReprint"`) != 1 {
		t.Fatal("btnOrderReprint must exist exactly once")
	}
	if strings.Contains(adminHTML, "btnInvoiceDetailReprint').dataset.reprint") ||
		strings.Contains(adminHTML, "btnOrderReprint').dataset.reprint") ||
		strings.Contains(adminHTML, "reprintBtn.dataset.reprint") {
		t.Fatal("detail/order reprint buttons must not use data-reprint (delegation double-fire)")
	}
	if strings.Count(adminHTML, "withBusy(btn, FiscalAdminI18n.t('common.reprinting')") != 1 {
		t.Fatal("reprint withBusy must exist exactly once in handleReprintClick")
	}
	staticBtnDelegationMarkers := []string{
		"btnInvoiceDetailCredit').dataset.credit",
		"btnInvoiceDetailDebit').dataset.debit",
		"creditBtn.dataset.credit",
		"debitBtn.dataset.debit",
		"btnInvLinkedOpen').dataset.openOriginal",
		"linkBtn.dataset.openOriginal",
	}
	for _, forbidden := range staticBtnDelegationMarkers {
		if strings.Contains(adminHTML, forbidden) {
			t.Fatalf("static Admin buttons must not use delegation data-* markers: %q", forbidden)
		}
	}
	if strings.Count(adminHTML, `id="invoiceDetailModal"`) != 1 {
		t.Fatal("invoiceDetailModal must exist exactly once")
	}
	if !strings.Contains(adminHTML, "formatInvoiceBuyerCell(inv)") {
		t.Fatal("invoice list must render buyer via formatInvoiceBuyerCell only")
	}
	if !strings.Contains(adminHTML, "formatInvoiceNoCell(inv)") {
		t.Fatal("invoice list must render ticket no via formatInvoiceNoCell only")
	}
	if !strings.Contains(adminHTML, "formatInvoiceWhenCell(inv)") {
		t.Fatal("invoice list must render issued-at via formatInvoiceWhenCell only")
	}
	if !strings.Contains(adminHTML, "formatInvoicePaymentLabel(inv)") {
		t.Fatal("invoice list must render payment via formatInvoicePaymentLabel only")
	}
	if !strings.Contains(adminHTML, `colspan="7" class="hint">' + FiscalAdminI18n.t('common.loading')`) {
		t.Fatal("invoice list loading colspan must be 7")
	}
	if strings.Contains(adminHTML, "truncateHash(inv.") || strings.Contains(adminHTML, `title="' + (inv.hash`) {
		t.Fatal("invoice list must not show hash columns or title tooltips")
	}
	if !strings.Contains(adminHTML, "invoiceDetailHashRow") {
		t.Fatal("hash fields must render in invoice detail drawer only")
	}
	if strings.Contains(adminHTML, "inv.print_status") && strings.Contains(adminHTML, "refreshInvoices") {
		// print_status only in detail modal renderer, not list row template
		idx := strings.Index(adminHTML, "async function refreshInvoices")
		endFn := strings.Index(adminHTML[idx:], "async function refreshBills")
		if endFn < 0 {
			endFn = strings.Index(adminHTML[idx:], "function renderHomeStats")
		}
		if endFn > 0 && strings.Contains(adminHTML[idx:idx+endFn], "inv.print_status") {
			t.Fatal("invoice list must not render print_status column")
		}
	}
	if !strings.Contains(adminHTML, "inv.system_entry_date") {
		t.Fatal("invoice list must display system_entry_date as 签发时刻")
	}
	if strings.Contains(adminHTML, "inv.invoice_date") {
		t.Fatal("invoice list must not render invoice_date")
	}
}

func TestAdminHTMLOperatorAccessSinglePath(t *testing.T) {
	for _, fn := range []string{
		"function operatorRole()",
		"function canAccessSettings",
		"function canManageProvisioning",
		"function canManageOperators",
		"function applyOperatorAccess",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Contains(adminHTML, "function isLoggedInOwner") {
		t.Fatal("remove isLoggedInOwner; use canAccessSettings/canManageProvisioning")
	}
	if strings.Contains(adminHTML, "operator-owner") || strings.Contains(adminHTML, "owner-only") {
		t.Fatal("remove M3.2b operator-owner/owner-only; use settings-access/admin/manager classes")
	}
	if !strings.Contains(adminHTML, `id="navSettings"`) || !strings.Contains(adminHTML, "settings-access-only") {
		t.Fatal("settings nav must use settings-access-only")
	}
	if strings.Count(adminHTML, "settings-admin-only") < 4 {
		t.Fatal("settings must gate admin-only sections")
	}
	if strings.Count(adminHTML, `id="changePinModal"`) != 1 {
		t.Fatal("changePinModal must exist exactly once")
	}
	if strings.Count(adminHTML, `id="btnChangePin"`) != 1 {
		t.Fatal("btnChangePin must exist exactly once in sidebar")
	}
	if !strings.Contains(adminHTML, "function submitChangePin") {
		t.Fatal("submitChangePin must be the ONLY change-pin submit path")
	}
	if strings.Count(adminHTML, "/local/v1/setup/change-pin") != 1 {
		t.Fatalf("change-pin API must be called from exactly one place, got %d",
			strings.Count(adminHTML, "/local/v1/setup/change-pin"))
	}
}

func TestAdminHTMLSettingsUILayoutSinglePath(t *testing.T) {
	if n := strings.Count(adminHTML, "function applySettingsStatusUI"); n != 1 {
		t.Fatalf("applySettingsStatusUI must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function saveTaxpayerSettings"); n != 1 {
		t.Fatalf("saveTaxpayerSettings must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function saveATCredentialsSettings"); n != 1 {
		t.Fatalf("saveATCredentialsSettings must appear exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "setupChecklist") {
		t.Fatal("legacy setupChecklist must be removed; use setupDashboard")
	}
	if !strings.Contains(adminHTML, `id="setupDashboard"`) {
		t.Fatal("settings must use setupDashboard")
	}
	if !strings.Contains(adminHTML, `id="settingsShell"`) || !strings.Contains(adminHTML, `id="settingsNav"`) {
		t.Fatal("settings must define shell + nav")
	}
	if strings.Count(adminHTML, "j('PUT', '/local/v1/setup/taxpayer'") != 1 {
		t.Fatalf("taxpayer PUT must go through saveTaxpayerSettings only, got %d",
			strings.Count(adminHTML, "j('PUT', '/local/v1/setup/taxpayer'"))
	}
	if strings.Count(adminHTML, "j('PUT', '/local/v1/setup/at-credentials'") != 1 {
		t.Fatalf("at-credentials PUT must go through saveATCredentialsSettings only, got %d",
			strings.Count(adminHTML, "j('PUT', '/local/v1/setup/at-credentials'"))
	}
	if strings.Contains(adminHTML, "1. 门店信息") || strings.Contains(adminHTML, "8. 备份与换机") {
		t.Fatal("numbered settings sections must be removed")
	}
}

func TestAdminHTMLOperatorManageM32bSinglePath(t *testing.T) {
	if n := strings.Count(adminHTML, "function forceLogout"); n != 1 {
		t.Fatalf("forceLogout must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function putOperator"); n != 1 {
		t.Fatalf("putOperator must appear exactly once, got %d", n)
	}
	if strings.Count(adminHTML, "/local/v1/setup/operators/manage") != 1 {
		t.Fatalf("operators/manage must be called from loadOperatorManageCache only, got %d",
			strings.Count(adminHTML, "/local/v1/setup/operators/manage"))
	}
	if strings.Count(adminHTML, "j('PUT', '/local/v1/setup/operator'") != 1 {
		t.Fatalf("operator PUT must go through putOperator() only, got %d direct j() calls",
			strings.Count(adminHTML, "j('PUT', '/local/v1/setup/operator'"))
	}
	if strings.Contains(adminHTML, "btnAddOperator") || strings.Contains(adminHTML, "operatorsList") {
		t.Fatal("legacy operator checklist UI must be removed")
	}
	if !strings.Contains(adminHTML, "session_revoked") || !strings.Contains(adminHTML, "operator_inactive") {
		t.Fatal("j() must handle session_revoked and operator_inactive via forceLogout")
	}
	for _, fn := range []string{"function loggedInOperatorId", "function isEditingSelf", "function operatorMenuActionVisible"} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if !strings.Contains(adminHTML, "editingSelf") || !strings.Contains(adminHTML, "isEditingSelf($('#operatorFormId').value)") {
		t.Fatal("operator self-edit must hide role and preserve existing role on submit")
	}
	if n := strings.Count(adminHTML, "function applySetupStatusFromResponse"); n != 1 {
		t.Fatalf("applySetupStatusFromResponse must appear exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "submitOperatorForm") && strings.Contains(adminHTML, "await refreshSetupStatus();") {
		if strings.Count(adminHTML, "async function submitOperatorForm") > 0 {
			// submitOperatorForm must not re-fetch status after putOperator applies embedded SetupStatus.
			idx := strings.Index(adminHTML, "async function submitOperatorForm")
			end := strings.Index(adminHTML[idx:], "async function submitOperatorResetPin")
			if end < 0 {
				end = len(adminHTML) - idx
			}
			block := adminHTML[idx : idx+end]
			if strings.Contains(block, "refreshSetupStatus") {
				t.Fatal("submitOperatorForm must not call refreshSetupStatus; use applySetupStatusFromResponse via putOperator")
			}
		}
	}
}

func TestAdminHTMLListRowMenuSinglePath(t *testing.T) {
	for _, fn := range []string{
		"function openListRowMenu",
		"function closeListRowMenu",
		"function positionListRowMenu",
		"async function runListRowMenuAction",
		"function listRowMenuItemsForTrigger",
		"function listRowMenuCtxForTrigger",
		"function listRowMenuTriggerHtml",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if n := strings.Count(adminHTML, `id="listRowMenuPop"`); n != 1 {
		t.Fatalf("listRowMenuPop must exist exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "operatorMenuPop") || strings.Contains(adminHTML, "openOperatorMenuPop") {
		t.Fatal("operator menu must be folded into #listRowMenuPop only")
	}
	if strings.Contains(adminHTML, "data-op-menu-trigger") || strings.Contains(adminHTML, "btn-op-menu-trigger") {
		t.Fatal("use data-row-menu-trigger / btn-row-menu-trigger only")
	}
	if strings.Contains(adminHTML, "class=\"op-menu\"") || strings.Contains(adminHTML, `<details class="op-menu"`) {
		t.Fatal("row menu must not use details/op-menu")
	}
	if strings.Contains(adminHTML, `data-op-action="edit" data-op-id`) {
		t.Fatal("row template must not embed inline menu actions; use #listRowMenuPop")
	}
}

func TestAdminHTMLConfirmActionSinglePath(t *testing.T) {
	if strings.Contains(adminHTML, "confirm(") {
		t.Fatal("admin must not use native confirm(); use openConfirmAction")
	}
	if strings.Contains(adminHTML, "alert(") || strings.Contains(adminHTML, "prompt(") {
		t.Fatal("admin must not use native alert() or prompt()")
	}
	for _, fn := range []string{
		"function openConfirmAction",
		"function closeConfirmActionModal",
		"async function runConfirmAction",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if n := strings.Count(adminHTML, `id="confirmActionModal"`); n != 1 {
		t.Fatalf("confirmActionModal must exist exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "    openConfirmAction({"); n != 7 {
		t.Fatalf("openConfirmAction must be called from exactly 7 sites, got %d", n)
	}
}

func TestAdminHTMLAuditLogSinglePath(t *testing.T) {
	for _, fn := range []string{
		"function auditLogQueryUrl",
		"function renderAuditFilterOptions",
		"function renderAuditLogTable",
		"async function refreshAuditLog",
		"function initAuditLogPanel",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if n := strings.Count(adminHTML, "/local/v1/audit-log"); n != 1 {
		t.Fatalf("audit-log API must be called from auditLogQueryUrl only, got %d", n)
	}
	if !strings.Contains(adminHTML, `data-settings-section="audit"`) {
		t.Fatal("audit panel must use data-settings-section=audit")
	}
	if !strings.Contains(adminHTML, ">操作记录<") {
		t.Fatal("audit nav must contain 操作记录")
	}
	if strings.Contains(adminHTML, "auditActionLabels") || strings.Contains(adminHTML, "OWNER_AUDIT_ACTIONS") {
		t.Fatal("do not duplicate action labels in admin JS; use auditActionText + API codes")
	}
	if n := strings.Count(adminHTML, "function auditActionText"); n != 1 {
		t.Fatalf("auditActionText must be the only action-label mapper, got %d", n)
	}
	if n := strings.Count(adminHTML, "admin-list-select-field"); n != 2 {
		t.Fatalf("audit filters must use admin-list-select-field exactly twice, got %d", n)
	}
	if !strings.Contains(adminHTML, `<label for="auditActionFilter" data-i18n="settings.audit.action_type">操作类型</label>`) {
		t.Fatal("audit action filter must use admin-list-select-field + settings.audit.action_type")
	}
}

func TestAdminHTMLOperatorsTableLayout(t *testing.T) {
	if !strings.Contains(adminHTML, "#operatorsTable.list-table { table-layout: fixed; width: 100%; }") {
		t.Fatal("operators table must use full-width fixed layout")
	}
	if strings.Contains(adminHTML, "<colgroup>") {
		t.Fatal("operators table must not use colgroup; use scoped th/td width only")
	}
	if strings.Count(adminHTML, `id="operatorsTable"`) != 1 {
		t.Fatal("operators table must exist exactly once")
	}
	if strings.Contains(adminHTML, "#operatorsTable.list-table th.col-actions") {
		t.Fatal("operators actions width must use shared sticky .col-actions only (list-pagination.css)")
	}
	if !strings.Contains(string(fiscalUIListPaginationCSS), "/* ONLY sticky last-column actions") {
		t.Fatal("sticky col-actions must live in list-pagination.css")
	}
}
