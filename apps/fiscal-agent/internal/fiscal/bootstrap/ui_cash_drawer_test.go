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
	if strings.Count(html, `POST', '/local/v1/cash-drawer/open'`) != 1 {
		t.Fatalf("ONLY one POST cash-drawer/open got %d", strings.Count(html, `POST', '/local/v1/cash-drawer/open'`))
	}
	if strings.Count(html, `PUT', '/local/v1/setup/cash-drawer'`) != 1 {
		t.Fatal("ONLY one PUT setup/cash-drawer")
	}
	if strings.Count(html, `GET', '/local/v1/setup/cash-drawer'`) != 1 {
		t.Fatal("ONLY one GET setup/cash-drawer")
	}
	if strings.Contains(html, "tray") && strings.Contains(html, "cash-drawer") {
		// soft: no tray cash drawer — tray is windows Go; just ensure openCashDrawer is not named for tray
	}
	if !strings.Contains(html, `id="mainChrome"`) {
		t.Fatal("main-chrome host for drawer button required")
	}
	if !strings.Contains(html, `id="cashDrawerPin"`) {
		t.Fatal("settings pin select required")
	}
}
