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
