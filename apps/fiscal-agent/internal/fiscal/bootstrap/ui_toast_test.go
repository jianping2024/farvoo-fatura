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
