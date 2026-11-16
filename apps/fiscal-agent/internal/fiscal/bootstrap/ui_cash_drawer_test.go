package bootstrap

import (
	"strings"
	"testing"
)

func TestAdminCashDrawerUniqueWriters(t *testing.T) {
	html := adminHTML
	if strings.Count(html, `id="btnOpenCashDrawer"`) != 1 {
		t.Fatal("ONLY one #btnOpenCashDrawer")
	}
	if strings.Count(html, `function openCashDrawer`) != 1 {
		t.Fatal("ONLY openCashDrawer")
	}
	if strings.Count(html, `function saveCashDrawerPin`) != 1 {
		t.Fatal("ONLY saveCashDrawerPin")
	}
	if strings.Count(html, `function refreshCashDrawerPanel`) != 1 {
		t.Fatal("ONLY refreshCashDrawerPanel")
	}
	if strings.Count(html, `function syncMainChrome`) != 1 {
		t.Fatal("ONLY syncMainChrome for open-drawer dock visibility")
	}
	if strings.Count(html, `syncMainChrome(name);`) != 1 {
		t.Fatal("syncMainChrome must be called only from showView")
	}
	if !strings.Contains(html, `chrome.hidden = name !== 'invoices'`) {
		t.Fatal("open-drawer dock must show ONLY on invoices hub")
	}
	if strings.Count(html, `POST', '/local/v1/cash-drawer/open'`) != 1 {
		t.Fatalf("ONLY one POST cash-drawer/open got %d", strings.Count(html, `POST', '/local/v1/cash-drawer/open'`))
	}
	if strings.Count(html, `PUT', '/local/v1/setup/cash-drawer'`) != 1 {
		t.Fatal("ONLY one PUT setup/cash-drawer")
	}
	if strings.Count(html, `GET', '/local/v1/setup/cash-drawer'`) != 1 {
		t.Fatal("ONLY one GET setup/cash-drawer")
	}
	if !strings.Contains(html, `id="mainChrome"`) {
		t.Fatal("main-chrome host for drawer button required")
	}
	if !strings.Contains(html, `id="mainChrome" hidden`) {
		t.Fatal("mainChrome must start hidden until showView(invoices)")
	}
	if !strings.Contains(html, `id="cashDrawerPin"`) {
		t.Fatal("settings pin select required")
	}
}

func TestAdminCashDrawerDockBottomLeftPrimary(t *testing.T) {
	html := adminHTML
	if strings.Count(html, `position: fixed`) < 1 {
		t.Fatal("drawer dock must use position fixed")
	}
	if strings.Count(html, `left: calc(224px + 1.75rem)`) != 1 {
		t.Fatal("ONLY one desktop left: calc(224px + 1.75rem) for drawer dock")
	}
	if strings.Count(html, `.main-chrome { left: 1.75rem; }`) != 1 {
		t.Fatal("ONLY one narrow-screen .main-chrome left override")
	}
	if !strings.Contains(html, `bottom: 1.25rem`) {
		t.Fatal("drawer dock must sit near viewport bottom")
	}
	// Must be primary button level — not secondary ghost.
	if strings.Contains(html, `class="secondary btn-drawer"`) || strings.Contains(html, `class="btn-drawer secondary"`) {
		t.Fatal("btnOpenCashDrawer must not use secondary (ghost) level")
	}
	if !strings.Contains(html, `class="btn-drawer" id="btnOpenCashDrawer"`) {
		t.Fatal("btnOpenCashDrawer must use primary button + btn-drawer size only")
	}
	if n := strings.Count(html, `id="btnOpenCashDrawer"`); n != 1 {
		t.Fatalf("ONLY one open-drawer control, got %d", n)
	}
}
