package bootstrap

import (
	"strings"
	"testing"
)

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
		">手工开票<small>本机新建</small>",
		">收银账单<small>待开票 · 来自收银</small>",
		"＋ 新建开票",
		"处理收银账单",
		"有新的收银账单",
		"暂无收银账单",
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

func TestAdminHTMLHomeCtaPeerAndFocus(t *testing.T) {
	if !strings.Contains(adminHTML, `id="ctaNewOrder"`) || !strings.Contains(adminHTML, `id="ctaPendingBills"`) {
		t.Fatal("home must expose ctaNewOrder and ctaPendingBills")
	}
	if !strings.Contains(adminHTML, "function focusHomePrimaryCta") {
		t.Fatal("admin must define focusHomePrimaryCta once")
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
		"<th>签发时刻</th>",
		"<th>票号</th>",
		"<th>金额</th>",
		"<th>购方</th>",
		"<th>来源</th>",
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
		"async function refreshInvoices",
		"async function reprintInvoice",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
	}
	if strings.Count(adminHTML, `id="invoiceDetailModal"`) != 1 {
		t.Fatal("invoiceDetailModal must exist exactly once")
	}
	if !strings.Contains(adminHTML, "formatInvoiceBuyerCell(inv)") {
		t.Fatal("invoice list must render buyer via formatInvoiceBuyerCell only")
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
