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

// TestAdminSetupStatusCacheSplit nails P1-S Public vs session Full: login must not
// poison setupStatusCache; ensureSetupStatus trusts isFullSetupStatus only.
func TestAdminSetupStatusCacheSplit(t *testing.T) {
	if n := strings.Count(adminHTML, "async function loadLoginSetupPublic"); n != 1 {
		t.Fatalf("loadLoginSetupPublic must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function isFullSetupStatus"); n != 1 {
		t.Fatalf("isFullSetupStatus must appear exactly once, got %d", n)
	}
	if !strings.Contains(adminHTML, "if (isFullSetupStatus(setupStatusCache)) return setupStatusCache") {
		t.Fatal("ensureSetupStatus must trust only isFullSetupStatus(setupStatusCache)")
	}
	if !strings.Contains(adminHTML, "if (!isFullSetupStatus(st))") {
		t.Fatal("refreshSetupStatus must reject anonymous Public before caching")
	}
	// Non-null writers of setupStatusCache: only applySetupStatusFromResponse + refreshSetupStatus.
	assigns := 0
	for _, line := range strings.Split(adminHTML, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "setupStatusCache =") {
			continue
		}
		if trim == "setupStatusCache = null;" || trim == "let setupStatusCache = null;" {
			continue
		}
		if trim != "setupStatusCache = st;" {
			t.Fatalf("unexpected setupStatusCache assign: %s", trim)
		}
		assigns++
	}
	if assigns != 2 {
		t.Fatalf("want exactly 2 setupStatusCache = st writers (apply + refresh), got %d", assigns)
	}
	loginFn := adminHTML[strings.Index(adminHTML, "async function loadLoginOperators("):]
	if i := strings.Index(loginFn[1:], "\n  async function "); i > 0 {
		loginFn = loginFn[:i+1]
	}
	if strings.Contains(loginFn, "setupStatusCache") {
		t.Fatal("loadLoginOperators must not reference setupStatusCache")
	}
	if !strings.Contains(loginFn, "loadLoginSetupPublic()") {
		t.Fatal("loadLoginOperators must call loadLoginSetupPublic")
	}
}

// TestAdminBootstrapOwnerAndLoginAutoLogin nails cold-start create-admin → login:
// must clear loginSetupPublicCache and write outer pin (not a shadowed param).
func TestAdminBootstrapOwnerAndLoginAutoLogin(t *testing.T) {
	if n := strings.Count(adminHTML, "async function bootstrapOwnerAndLogin"); n != 1 {
		t.Fatalf("bootstrapOwnerAndLogin must appear exactly once, got %d", n)
	}
	fn := adminHTML[strings.Index(adminHTML, "async function bootstrapOwnerAndLogin"):]
	if i := strings.Index(fn[1:], "\n  async function "); i > 0 {
		fn = fn[:i+1]
	}
	if !strings.Contains(fn, "loginSetupPublicCache = null") {
		t.Fatal("bootstrapOwnerAndLogin must invalidate loginSetupPublicCache before loadLoginOperators")
	}
	if !strings.Contains(fn, "await loadLoginOperators()") {
		t.Fatal("bootstrapOwnerAndLogin must reload operators after create")
	}
	if !strings.Contains(fn, "await tryLogin()") {
		t.Fatal("bootstrapOwnerAndLogin must call tryLogin after create")
	}
	// Param must not shadow outer let pin (regression: pin=pinVal only updated the param).
	if strings.Contains(fn, "async function bootstrapOwnerAndLogin(displayName, pin)") {
		t.Fatal("bootstrapOwnerAndLogin must not name the PIN param `pin` (shadows outer pin)")
	}
	if !strings.Contains(fn, "pin = pinVal") {
		t.Fatal("bootstrapOwnerAndLogin must assign outer pin = pinVal for tryLogin")
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
	if n := strings.Count(adminHTML, "function setFiscalBtnLocked"); n != 1 {
		t.Fatalf("setFiscalBtnLocked must appear exactly once, got %d", n)
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
	if strings.Contains(wbBody, "btn.disabled = false") {
		t.Fatal("withBusy must not unconditionally re-enable; respect data-fiscal-locked")
	}
	if !strings.Contains(wbBody, "btn.dataset.fiscalLocked === '1'") {
		t.Fatal("withBusy finally must restore disabled from data-fiscal-locked")
	}
	if !strings.Contains(adminHTML, "setFiscalBtnLocked(btn, true)") || !strings.Contains(adminHTML, "setFiscalBtnLocked(btn, false)") {
		t.Fatal("applySeriesForm must lock/unlock series buttons via setFiscalBtnLocked")
	}
	if !strings.Contains(adminHTML, "setFiscalBtnLocked(cloudBtn,") {
		t.Fatal("applySettingsStatusUI must lock #btnActCloud via setFiscalBtnLocked")
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
	if n := strings.Count(adminHTML, "const DEFAULT_SALE_DOC = 'FS'"); n != 1 {
		t.Fatalf("DEFAULT_SALE_DOC must be FS exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "const DEFAULT_SALE_DOC = 'FT'") {
		t.Fatal("DEFAULT_SALE_DOC must not default to FT")
	}
	if n := strings.Count(adminHTML, "function fmtVatPercent"); n != 1 {
		t.Fatalf("fmtVatPercent must be the ONLY IVA%% label helper, got %d", n)
	}
	if strings.Contains(adminHTML, "p.vat_rate + '%'") || strings.Contains(adminHTML, "ln.vat + '%'") {
		t.Fatal("do not concatenate vat_rate/'%' ad-hoc; use fmtVatPercent only")
	}
	if n := strings.Count(adminHTML, "const PRODUCT_VAT_RATES"); n != 1 {
		t.Fatalf("PRODUCT_VAT_RATES must be the ONLY catalog IVA%% list, got %d", n)
	}
	if n := strings.Count(adminHTML, "const DEFAULT_PRODUCT_VAT"); n != 1 {
		t.Fatalf("DEFAULT_PRODUCT_VAT must be the ONLY product IVA default, got %d", n)
	}
	if n := strings.Count(adminHTML, "function productVatOrDefault"); n != 1 {
		t.Fatalf("productVatOrDefault must be the ONLY product IVA resolver, got %d", n)
	}
	if n := strings.Count(adminHTML, "function fillProductVatSelect"); n != 1 {
		t.Fatalf("fillProductVatSelect must be the ONLY #pVat option writer, got %d", n)
	}
	if n := strings.Count(adminHTML, "fillProductVatSelect();"); n != 1 {
		t.Fatalf("fillProductVatSelect must be called once at boot, got %d", n)
	}
	if strings.Contains(adminHTML, `<input id="pVat"`) {
		t.Fatal("pVat must be <select>, not free-text <input>")
	}
	if !strings.Contains(adminHTML, `<select id="pVat"></select>`) {
		t.Fatal("pVat must be an empty <select> filled only by fillProductVatSelect")
	}
	if strings.Contains(adminHTML, "$('#pVat').value = '13.00'") || strings.Contains(adminHTML, `$('#pVat').value = "13.00"`) {
		t.Fatal("do not hardcode #pVat default; use DEFAULT_PRODUCT_VAT / productVatOrDefault")
	}
	if !strings.Contains(adminHTML, "const PRODUCT_VAT_RATES = ['23.00', '13.00', '6.00']") {
		t.Fatal("PRODUCT_VAT_RATES must be mainland 23/13/6 only")
	}
	if !strings.Contains(adminHTML, "const DEFAULT_PRODUCT_VAT = '13.00'") {
		t.Fatal("DEFAULT_PRODUCT_VAT must be 13.00")
	}
	if n := strings.Count(adminHTML, "function lineFromGrossPreview"); n != 1 {
		t.Fatalf("lineFromGrossPreview must be the ONLY gross→net/tax preview, got %d", n)
	}
	if n := strings.Count(adminHTML, "function orderMoneyBreakdown"); n != 1 {
		t.Fatalf("orderMoneyBreakdown must be the ONLY order net/tax/gross preview, got %d", n)
	}
	if n := strings.Count(adminHTML, "function renderOrderMoneyBox"); n != 1 {
		t.Fatalf("renderOrderMoneyBox must be the ONLY order money box writer, got %d", n)
	}
	if !strings.Contains(adminHTML, `data-i18n="col.vat"`) {
		t.Fatal("order lines table must expose IVA column (col.vat)")
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
		"处理收银账单",
		"收银账单",
		"新建开票",
		"手工开票",
		"＋ 手工开票",
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
		`data-i18n="home.cta.new_order">＋ 开票</button>`,
		`data-i18n="orders.new">开票</h1>`,
		`id="homeGreeting"`,
		`id="operatorName"`,
		`id="uiLocaleSelect"`,
		`/fiscal-ui/admin-i18n.js`,
	}
	forbiddenLeftovers := []string{
		"view-home", "view-orders", "fiscal_orders", "order.issue_later",
		"nav.home", "nav.orders", "home.title", "home.recent",
		"statPendingOrders", "homeRecentInvoicesTbody", "btnOrderInvoiceBack",
		"renderOrdersList", "loadOrders", "data-nav=\"home\"", "data-nav=\"orders\"",
		"showView('home')", "手工待开", "稍后开票",
		`id="navBills"`, `id="navBillsBadge"`, `id="ctaPendingBills"`, `id="statPendingBills"`,
		`data-nav="bills"`, `home.stat.bills_cta`,
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
	if n := strings.Count(adminHTML, "＋ 开票"); n != 1 {
		t.Fatalf("hub CTA ＋ 开票 must appear exactly once, got %d", n)
	}
}

func TestAdminHTMLNoPageXScrollUnique(t *testing.T) {
	if n := strings.Count(adminHTML, "overflow-x: hidden"); n != 1 {
		t.Fatalf("page overflow-x:hidden must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "224px minmax(0, 1fr)"); n != 1 {
		t.Fatalf("shell columns must appear exactly once, got %d", n)
	}
	if !strings.Contains(adminHTML, ".main { padding: 1.5rem 1.75rem 5rem; max-width: 980px; min-width: 0; }") {
		t.Fatal("main must shrink with min-width: 0")
	}
	if strings.Contains(adminHTML, "100vw") {
		t.Fatal("admin must not use 100vw (causes page x-scroll)")
	}
}

func TestAdminHTMLShellContentScrollUnique(t *testing.T) {
	// Scheme A: locked #app-shell + ONLY #mainScroll as content scrollport.
	if n := strings.Count(adminHTML, `id="mainScroll"`); n != 1 {
		t.Fatalf("ONLY one #mainScroll content scrollport, got %d", n)
	}
	if n := strings.Count(adminHTML, "ONLY content scrollport"); n != 1 {
		t.Fatalf("ONLY one content-scrollport marker comment, got %d", n)
	}
	if !strings.Contains(adminHTML, "height: 100%; overflow: hidden;") {
		t.Fatal("#app-shell must lock height and hide outer overflow")
	}
	if strings.Contains(adminHTML, "min-height: 100vh; position: sticky; top: 0;") {
		t.Fatal("sidebar must not use sticky+min-height:100vh (stretch defeats sticky)")
	}
	if n := strings.Count(adminHTML, "mainScroll.scrollTop = 0"); n != 1 {
		t.Fatalf("showView must reset #mainScroll once, got %d", n)
	}
}

func TestAdminHTMLHomeWorkbenchUnique(t *testing.T) {
	for _, fn := range []string{
		"function renderHomeDateChip",
		"function renderHomeGreeting",
		"function invoiceRowActionsHtml",
		"function listRowMenuTriggerHtml",
		"function renderInvoiceHubStats",
		"function renderPendingBillsStat",
		"function invoiceListEmptyCellHtml",
		"function refreshInvoiceHubStats",
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
		`id="statGrossNet"`,
		`id="statCashSum"`,
		`id="btnOrderNewCancel"`,
		`id="btnOrderInvoiceCancel"`,
	} {
		if n := strings.Count(adminHTML, id); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", id, n)
		}
	}
	for _, id := range []string{`id="ctaPendingBills"`, `id="statPendingBills"`, `id="navBills"`, `id="navBillsBadge"`} {
		if strings.Contains(adminHTML, id) {
			t.Fatalf("bill sync UI paused: must not expose %s", id)
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
	if strings.Contains(adminHTML, `class="cta-big" id="ctaNewOrder"`) {
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
		t.Fatal("order/source list cell must call formatInvoiceOrderCell once")
	}
	if strings.Count(adminHTML, "formatInvoiceOrderCell(inv)") != 3 {
		t.Fatal("formatInvoiceOrderCell(inv) must appear only in def, invoice list, and detail")
	}
	if strings.Count(adminHTML, "+ formatInvoicePaymentLabel(inv)") != 1 {
		t.Fatal("payment cells must call formatInvoicePaymentLabel only in invoice list")
	}
	if strings.Count(adminHTML, "formatInvoicePaymentLabel(inv)") != 3 {
		t.Fatal("formatInvoicePaymentLabel(inv) must appear only in def, invoice list, and detail")
	}
	if strings.Contains(adminHTML, "inv.order_label") {
		t.Fatal("do not read order_label; use formatInvoiceOrderCell")
	}
	if !strings.Contains(adminHTML, "formatInvoiceDocStatusDetailHtml(inv.document_status)") {
		t.Fatal("detail doc status must use formatInvoiceDocStatusDetailHtml")
	}
	if !strings.Contains(adminHTML, "formatInvoicePrintStatus(inv.print_status)") {
		t.Fatal("detail print status must use formatInvoicePrintStatus")
	}
	if strings.Contains(adminHTML, "inv.invoice_no || inv.document_type") {
		t.Fatal("do not fall back invoice_no to document_type in list cells")
	}
	if strings.Contains(adminHTML, "$('#statPendingBills')") || strings.Contains(adminHTML, "$('#ctaPendingBills')") {
		t.Fatal("bill sync UI paused: do not reference removed hub bills elements")
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
	if strings.Contains(section, `hub-bills-cta`) || strings.Contains(section, `id="ctaPendingBills"`) {
		t.Fatal("bill sync UI paused: hub must not expose bills CTA")
	}
	if !strings.Contains(section, `id="ctaNewOrder"`) {
		t.Fatal("hub must expose ctaNewOrder")
	}
	if strings.Contains(section, `home-topbar-right`) && strings.Contains(section[strings.Index(section, `home-topbar-right`):strings.Index(section, `hub-entry-strip`)], `id="ctaNewOrder"`) {
		t.Fatal("ctaNewOrder must not remain in home-topbar")
	}
	if !strings.Contains(adminHTML, "ONLY Admin button levels") {
		t.Fatal("button level contract must be documented once in admin CSS")
	}
	if strings.Count(adminHTML, `id="ctaNewOrder"`) != 1 {
		t.Fatal("ctaNewOrder must appear exactly once")
	}
	if !strings.Contains(section, `id="statGrossNet"`) {
		t.Fatal("invoice hub must include today invoice metric")
	}
	if strings.Contains(section, `statPendingOrders`) || strings.Contains(section, "home-recent") {
		t.Fatal("invoice hub must not include pending orders or home-recent mini-table")
	}
	if !strings.Contains(section, `data-i18n="common.all">全部</span></button>`) {
		t.Fatal("全部 tab must show i18n label only (no ALL abbr)")
	}
	if strings.Contains(section, `data-invoice-type="FT"`) {
		t.Fatal("invoice type tabs (except 全部) must not be hard-coded; renderInvoiceTypeTabs owns them")
	}
	if n := strings.Count(adminHTML, "function enabledInvoiceListDocTypes"); n != 1 {
		t.Fatalf("enabledInvoiceListDocTypes must be the only enabled-list source, got %d", n)
	}
	if n := strings.Count(adminHTML, "function renderInvoiceTypeTabs"); n != 1 {
		t.Fatalf("renderInvoiceTypeTabs must be the only invoice tab renderer, got %d", n)
	}
	if n := strings.Count(adminHTML, "function docTypeHasActiveSeries"); n != 1 {
		t.Fatalf("docTypeHasActiveSeries must be the only series_ok gate for doc types, got %d", n)
	}
	if !strings.Contains(adminHTML, `'doc.tab.' + code`) {
		t.Fatal("renderInvoiceTypeTabs must use doc.tab.* business labels")
	}
	for _, abbr := range []string{"ALL", "FT", "FS", "NC", "ND"} {
		if strings.Contains(section, "<small>"+abbr+"</small>") {
			t.Fatalf("invoice type tabs must not show %s abbr under label", abbr)
		}
	}
	if !strings.Contains(adminHTML, "align-items: center; gap: 0.1rem;") ||
		!strings.Contains(adminHTML, ".invoice-type-tabs button {\n      min-width: 5.5rem; padding: 0.4rem 0.7rem; line-height: 1.2; text-align: center;") {
		t.Fatal("invoice type tab cards must center label")
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
		".invoices-table.list-table .col-no { width: 10rem; white-space: nowrap; font-weight: 600; }",
		".invoices-table.list-table .col-pay {",
	} {
		if n := strings.Count(adminHTML, css); n != 1 {
			t.Fatalf("clerk invoice layout CSS %q must appear exactly once, got %d", css, n)
		}
	}
	if n := strings.Count(adminHTML, "ONLY Admin type colors"); n != 1 {
		t.Fatalf("type color contract must be documented once, got %d", n)
	}
	if n := strings.Count(adminHTML, "--head:"); n != 1 {
		t.Fatalf("--head token must be defined once, got %d", n)
	}
	if !strings.Contains(adminHTML, "th { color: var(--head);") {
		t.Fatal("table headers must use --head, not --muted")
	}
	if strings.Contains(adminHTML, "th { color: var(--muted);") {
		t.Fatal("table headers must not use --muted")
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
	if !strings.Contains(adminHTML, `id="ctaNewOrder"`) {
		t.Fatal("home must expose ctaNewOrder")
	}
	if strings.Contains(adminHTML, `id="ctaPendingBills"`) {
		t.Fatal("bill sync UI paused: must not expose ctaPendingBills")
	}
	if n := strings.Count(adminHTML, "function focusHomePrimaryCta"); n != 1 {
		t.Fatalf("focusHomePrimaryCta must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function appShellIsOn"); n != 1 {
		t.Fatalf("appShellIsOn must be the ONLY shell-visible gate, got %d", n)
	}
	if !strings.Contains(adminHTML, "if (!appShellIsOn()) return;") {
		t.Fatal("focusHomePrimaryCta must no-op until app shell is on")
	}
	if strings.Contains(adminHTML, "suppressHomeCtaFocus") {
		t.Fatal("remove suppressHomeCtaFocus; use appShellIsOn in focusHomePrimaryCta")
	}
	focusIdx := strings.Index(adminHTML, "function focusHomePrimaryCta")
	if focusIdx < 0 {
		t.Fatal("missing focusHomePrimaryCta")
	}
	focusBody := adminHTML[focusIdx : focusIdx+400]
	if strings.Contains(focusBody, "ctaPendingBills") || strings.Contains(focusBody, "openBillCount > 0") {
		t.Fatal("focusHomePrimaryCta must only focus ＋开票 while bill sync UI paused")
	}
	if n := strings.Count(adminHTML, "function loadViewData"); n != 1 {
		t.Fatalf("loadViewData must be the ONLY view data loader, got %d", n)
	}
	if n := strings.Count(adminHTML, "await loadViewData(name)"); n != 1 {
		t.Fatalf("showView must await loadViewData exactly once, got %d", n)
	}
	for _, view := range []string{"invoices", "products", "customers", "bills", "settings"} {
		needle := "name === '" + view + "'"
		if !strings.Contains(adminHTML[strings.Index(adminHTML, "async function loadViewData"):], needle) {
			t.Fatalf("loadViewData must handle %s", view)
		}
	}
	if n := strings.Count(adminHTML, "function revealAppShell"); n != 1 {
		t.Fatalf("revealAppShell must be the ONLY login→shell reveal, got %d", n)
	}
	if n := strings.Count(adminHTML, "$('#app-shell').classList.add('on')"); n != 1 {
		t.Fatalf("#app-shell.on must be added only in revealAppShell, got %d", n)
	}
	enterIdx := strings.Index(adminHTML, "async function enterApp()")
	if enterIdx < 0 {
		t.Fatal("missing enterApp")
	}
	enterBody := adminHTML[enterIdx : enterIdx+1600]
	if !strings.Contains(enterBody, "await showView('invoices')") {
		t.Fatal("enterApp must await showView('invoices') (landing) before reveal")
	}
	if !strings.Contains(enterBody, "revealAppShell()") {
		t.Fatal("enterApp must reveal via revealAppShell")
	}
	awaitView := strings.Index(enterBody, "await showView('invoices')")
	reveal := strings.Index(enterBody, "revealAppShell()")
	if awaitView < 0 || reveal < 0 || awaitView > reveal {
		t.Fatal("enterApp must await landing showView before revealAppShell")
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
	for _, fn := range []string{
		"function setProductFieldsLocked",
		"function openProductFromList",
		"function closeModalById",
		"function topOpenModalBackdrop",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Contains(adminHTML, `data-row-kind="product"`) {
		t.Fatal("product rows open the workbench on click; do not keep a product ⋯ menu")
	}
	if !strings.Contains(adminHTML, `class="clickable" data-product-code="`) {
		t.Fatal("product list rows must be clickable via data-product-code")
	}
	if strings.Count(adminHTML, `colspan="4" class="hint">' + FiscalAdminI18n.t('common.loading')`) != 1 {
		t.Fatal("product list loading colspan must be 4")
	}
	if n := strings.Count(adminHTML, `class="ghost sheet-close"`); n != 9 {
		t.Fatalf("every Admin modal needs one sheet-close, got %d", n)
	}
	if n := strings.Count(adminHTML, `data-sheet-close data-i18n="common.close"`); n != 9 {
		t.Fatalf("data-sheet-close must appear once per modal, got %d", n)
	}
	if strings.Count(adminHTML, "ONLY modal dismiss chrome") != 1 {
		t.Fatal("sheet-close layout comment must appear once")
	}
	if strings.Count(adminHTML, "ONLY dismiss path for Admin .modal") != 1 {
		t.Fatal("closeModalById must be the only modal dismiss path")
	}
	if strings.Contains(adminHTML, `e.target.id === 'invoiceDetailModal'`) ||
		strings.Contains(adminHTML, `e.target.id === 'productPicker'`) ||
		strings.Contains(adminHTML, `e.target === $('#changePinModal')`) {
		t.Fatal("per-modal backdrop listeners must be gone; use closeModalById")
	}
	if strings.Count(adminHTML, `product.source === 'REMOTE_SYNC'`) != 1 {
		t.Fatal("REMOTE_SYNC readonly must be decided once inside openProductWorkbench")
	}
	orderTotal := strings.Index(adminHTML, `data-i18n="order.total"`)
	if orderTotal < 0 {
		t.Fatal("missing order total sheet")
	}
	orderHead := adminHTML[orderTotal-80 : orderTotal+80]
	if strings.Contains(orderHead, "data-sheet-close") {
		t.Fatal("in-page order sheets must not get a modal close button")
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
	if strings.Contains(adminHTML, "new EventSource('/local/v1/events')") {
		t.Fatal("bill sync UI paused: Admin must not open EventSource for bill drafts")
	}
	if strings.Contains(adminHTML, "setInterval(() =>") || strings.Contains(adminHTML, "setInterval(function") {
		t.Fatal("do not poll refreshBills via setInterval")
	}
	if strings.Contains(adminHTML, `id="navBillsBadge"`) || strings.Contains(adminHTML, `id="navBills"`) {
		t.Fatal("bill sync UI paused: sidebar must not include bills nav/badge")
	}
	if strings.Contains(adminHTML, "addEventListener('bill_drafts_changed'") {
		t.Fatal("bill sync UI paused: must not listen for bill_drafts_changed")
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
		"function isFullSetupStatus",
		"async function loadLoginSetupPublic",
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
	if n := strings.Count(adminHTML, "ONLY pinned head/body/foot shell"); n != 1 {
		t.Fatalf("sheet shell ONLY marker must appear once, got %d", n)
	}
	for _, legacy := range []string{"modal-head", "modal-body", "modal-foot", "panel-head", `class="panel-body"`, `class="panel-foot"`} {
		if strings.Contains(adminHTML, legacy) {
			t.Fatalf("legacy shell class %q must not appear; use sheet-head/body/foot only", legacy)
		}
	}
	if strings.Count(adminHTML, `class="panel panel-sheet"`) != 2 {
		t.Fatal("order-new confirm + order-invoice info must use panel-sheet exactly twice")
	}
	if !strings.Contains(adminHTML, `id="btnIssueInvoice"`) || !strings.Contains(adminHTML, `id="btnConfirmOrder"`) {
		t.Fatal("issue/confirm CTAs must remain")
	}
	issueIdx := strings.Index(adminHTML, `id="btnIssueInvoice"`)
	footBeforeIssue := strings.LastIndex(adminHTML[:issueIdx], `class="sheet-foot"`)
	if footBeforeIssue < 0 {
		t.Fatal("btnIssueInvoice must sit inside sheet-foot")
	}
	confirmIdx := strings.Index(adminHTML, `id="btnConfirmOrder"`)
	footBeforeConfirm := strings.LastIndex(adminHTML[:confirmIdx], `class="sheet-foot"`)
	if footBeforeConfirm < 0 {
		t.Fatal("btnConfirmOrder must sit inside sheet-foot")
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
	if !strings.Contains(adminHTML, "invoiceDetailRow(FiscalAdminI18n.t('col.issued_at'), formatInvoiceWhenCell(inv))") {
		t.Fatal("invoice detail issued-at must use formatInvoiceWhenCell (MM/dd HH:mm:ss)")
	}
	if strings.Contains(adminHTML, "function fmtTime(") {
		t.Fatal("fmtTime removed; issued-at must use formatMdHms via formatInvoiceWhenCell")
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
			endFn = strings.Index(adminHTML[idx:], "function renderInvoiceHubStats")
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
	if n := strings.Count(adminHTML, "function resetAdminListSessionState"); n != 1 {
		t.Fatalf("resetAdminListSessionState must be the ONLY logout list wipe, got %d", n)
	}
	if n := strings.Count(adminHTML, "resetAdminListSessionState();"); n != 1 {
		t.Fatalf("resetAdminListSessionState must be called only from forceLogout, got %d", n)
	}
	if !strings.Contains(adminHTML, "ONLY Admin list/filter session wipe on logout") {
		t.Fatal("resetAdminListSessionState must be documented as ONLY logout list wipe")
	}
	if n := strings.Count(adminHTML, "fiscal_invoice_date_range_v2"); n != 2 {
		// createDateRangeFilter storageKey + logout removeItem fallback
		t.Fatalf("fiscal_invoice_date_range_v2 must appear twice (init key + logout fallback), got %d", n)
	}
	if !strings.Contains(adminHTML, "invoiceDateFilterCtrl.resetToDefault") {
		t.Fatal("logout must reset invoice date via date-range resetToDefault")
	}
	if strings.Contains(adminHTML, "localStorage.removeItem('fiscal_invoice_date_range_v2')") &&
		strings.Count(adminHTML, "localStorage.removeItem('fiscal_invoice_date_range_v2')") != 1 {
		t.Fatal("logout date localStorage fallback must be exactly one removeItem")
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
	if n := strings.Count(adminHTML, `id="confirmActionReason"`); n != 1 {
		t.Fatalf("confirmActionReason input must exist exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "const CONFIRM_ACTION_REASON_MAX = 50"); n != 1 {
		t.Fatalf("CONFIRM_ACTION_REASON_MAX must be declared once as 50, got %d", n)
	}
	nCalls := strings.Count(adminHTML, "openConfirmAction({") - strings.Count(adminHTML, "function openConfirmAction({")
	if nCalls != 8 {
		t.Fatalf("openConfirmAction must be called from exactly 8 sites, got %d", nCalls)
	}
	// PF/GR/GT annul must collect reason from the confirm modal (SAF-T ≤50), not hard-code Cancelado.
	annulIdx := strings.Index(adminHTML, "$('#btnInvoiceDetailAnnulPf').onclick")
	if annulIdx < 0 {
		t.Fatal("annul button handler missing")
	}
	annulBlock := adminHTML[annulIdx:]
	if end := strings.Index(annulBlock, "$('#receiptFull')"); end > 0 {
		annulBlock = annulBlock[:end]
	}
	if !strings.Contains(annulBlock, "withReason: true") {
		t.Fatal("annul confirm must use withReason: true")
	}
	if strings.Contains(annulBlock, "reason: 'Cancelado'") || strings.Contains(annulBlock, `reason: "Cancelado"`) {
		t.Fatal("annul must not hard-code reason Cancelado; use modal input")
	}
}

func TestAdminHTMLAnnulledDocVisualSinglePath(t *testing.T) {
	for _, fn := range []string{
		"function invoiceListStatusPill(",
		"function formatInvoiceDocStatusDetailHtml(",
		"function formatInvoiceDocStatus(",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if !strings.Contains(adminHTML, "is-annulled") {
		t.Fatal("invoice list must mark annulled rows with is-annulled")
	}
	if !strings.Contains(adminHTML, "status-pill annulled") {
		t.Fatal("annulled status pill class missing")
	}
	i18n := string(fiscalUIAdminI18nJS)
	if !strings.Contains(i18n, `'invoice.doc_status.A'`) {
		t.Fatal("i18n must define invoice.doc_status.A")
	}
	if !strings.Contains(i18n, `'invoice.doc_status.F'`) {
		t.Fatal("i18n must define invoice.doc_status.F")
	}
	if !strings.Contains(i18n, `'invoice.doc_status.N'`) {
		t.Fatal("i18n must define invoice.doc_status.N")
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
	if !strings.Contains(adminHTML, "let auditLogPageSize = FiscalUI.LIST_DEFAULT_PAGE_SIZE;") {
		t.Fatal("audit list must use FiscalUI.LIST_DEFAULT_PAGE_SIZE (not a hardcoded size)")
	}
	if strings.Contains(adminHTML, "let auditLogPageSize = 50") {
		t.Fatal("audit list must not hardcode page size 50")
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

func TestAdminWizardLanguageFirstUnique(t *testing.T) {
	if n := strings.Count(adminHTML, `id="uiLocaleSelect"`); n != 1 {
		t.Fatalf("uiLocaleSelect must be the ONLY language control, got %d", n)
	}
	if n := strings.Count(adminHTML, "async function saveUILocale"); n != 1 {
		t.Fatalf("saveUILocale must be the ONLY Admin ui_locale write helper, got %d", n)
	}
	if n := strings.Count(adminHTML, "fiscal_settings_wizard_step_v2"); n != 1 {
		t.Fatalf("wizard step key must be v2 exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, `const SETTINGS_WIZARD_STEP_KEY = 'fiscal_settings_wizard_step'`) {
		t.Fatal("old wizard step key without _v2 must be removed")
	}
	if strings.Contains(adminHTML, `#settings-section-overview { display: none`) {
		t.Fatal("overview must not be force-hidden in wizard; language step reuses it")
	}
	if !strings.Contains(adminHTML, `id="settings-section-overview" data-wizard-step="1"`) {
		t.Fatal("overview must be wizard step 1 (language)")
	}
	if !strings.Contains(adminHTML, `data-settings-section="store" data-wizard-step="2"`) {
		t.Fatal("store must be wizard step 2")
	}
	if !strings.Contains(adminHTML, `data-settings-section="series" data-wizard-step="3"`) {
		t.Fatal("series must be wizard step 3")
	}
	if !strings.Contains(adminHTML, `{ step: 1, nav: 'lang' }`) {
		t.Fatal("SETTINGS_WIZARD_STEPS must start with lang")
	}
	if !strings.Contains(adminHTML, `{ step: 2, nav: 'store' }`) || !strings.Contains(adminHTML, `{ step: 3, nav: 'series' }`) {
		t.Fatal("SETTINGS_WIZARD_STEPS must continue store→series")
	}
	if n := strings.Count(adminHTML, "function wizardStepNav"); n != 1 {
		t.Fatalf("wizardStepNav must be the ONLY step→nav mapper, got %d", n)
	}
	if n := strings.Count(adminHTML, "function markLangWizardConfirmed"); n != 1 {
		t.Fatalf("markLangWizardConfirmed must be unique, got %d", n)
	}
	if !strings.Contains(adminHTML, `if (nav === 'lang')`) {
		t.Fatal("wizardNextStep must gate language via nav === 'lang' (not bare step===1 store check)")
	}
	if !strings.Contains(adminHTML, `else if (nav === 'store')`) {
		t.Fatal("wizardNextStep must gate store via nav === 'store'")
	}
	i18n := string(fiscalUIAdminI18nJS)
	if n := strings.Count(i18n, "'settings.nav.lang':"); n != 3 {
		t.Fatalf("settings.nav.lang must appear once per locale (3), got %d", n)
	}
}

// Save buttons stuck on "保存中…": markSaveSuccess captured the busy label as the original
// and its 1.8s timer restored it after withBusy's finally. Guard the handshake.
func TestAdminWithBusyMarkSaveSuccessLabel(t *testing.T) {
	for _, want := range []string{
		"btn.dataset.busyPrev = prev;",
		"if (!btn.classList.contains('btn-saved')) btn.textContent = prev;",
		"const orig = btn.dataset.labelOrig || btn.dataset.busyPrev || btn.textContent;",
	} {
		if !strings.Contains(adminHTML, want) {
			t.Fatalf("admin missing %q", want)
		}
	}
}

// Store form must load saved data, never ship demo values that a save would write back.
func TestAdminStoreFormLoadsSavedNoDemoDefaults(t *testing.T) {
	for _, bad := range []string{`value="517535009"`, `value="517535009/37"`, `value="demo-pass"`, `value="Farvoo Demo Lda"`} {
		if strings.Contains(adminHTML, bad) {
			t.Fatalf("admin store form still prefilled with %s", bad)
		}
	}
	for _, want := range []string{"async function loadSavedStoreForms()", "j('GET', '/local/v1/setup/taxpayer')", "j('GET', '/local/v1/setup/at-credentials')"} {
		if !strings.Contains(adminHTML, want) {
			t.Fatalf("admin missing %q", want)
		}
	}
}

// GR/GT load/unload points are structured fields sent as ship_from/ship_to objects.
func TestAdminGRShipPointsStructured(t *testing.T) {
	for _, want := range []string{`id="invShipFromPostal"`, `id="invShipToCity"`, "function readShipPoint(prefix)", "body.ship_from = shipFrom"} {
		if !strings.Contains(adminHTML, want) {
			t.Fatalf("admin missing %q", want)
		}
	}
	if strings.Contains(adminHTML, "ship_from_address") || strings.Contains(adminHTML, "ship_to_address") {
		t.Fatal("admin must not send legacy free-text ship addresses")
	}
}
