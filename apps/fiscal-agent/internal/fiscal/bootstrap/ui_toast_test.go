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
	if n := strings.Count(adminHTML, "function validatePortugueseNif"); n != 1 {
		t.Fatalf("validatePortugueseNif (Mod-11) must appear exactly once, got %d", n)
	}
	if n := strings.Count(adminHTML, "function readPersonBuyer"); n != 1 {
		t.Fatalf("readPersonBuyer must appear exactly once, got %d", n)
	}
	if strings.Contains(adminHTML, "function isNineDigitNif") {
		t.Fatal("remove isNineDigitNif; Mod-11 validatePortugueseNif is the ONLY NIF check")
	}
	if strings.Contains(adminHTML, `/^\d{9}$/`) {
		t.Fatal("must not use only-9-digit regex as sole NIF check")
	}
	if !strings.Contains(adminHTML, `id="billBuyerShared"`) || !strings.Contains(adminHTML, `data-person-nif=`) {
		t.Fatal("split bills need per-person NIF inputs; whole_table keeps billBuyerShared")
	}
	if !strings.Contains(adminHTML, `id="billNif"`) || !strings.Contains(adminHTML, `list="customerNifList"`) {
		t.Fatal("billNif must use shared customerNifList datalist")
	}
	if strings.Contains(adminHTML, "invNif').addEventListener('change'") {
		t.Fatal("do not bind invNif change separately; use bindCustomerNifAutofill")
	}
	if strings.Contains(adminHTML, "'<td>—</td>' +\n        '<td>' + fmtTimeShort") {
		t.Fatal("bill list must not hardcode amount column to em dash")
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
		"function readPersonBuyer",
	} {
		if n := strings.Count(adminHTML, fn); n != 1 {
			t.Fatalf("%s must appear exactly once, got %d", fn, n)
		}
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
	if !strings.Contains(adminHTML, "data-person-nif") || !strings.Contains(adminHTML, "data-issue-person") {
		t.Fatal("split bill UI must offer per-person NIF + issue")
	}
}
