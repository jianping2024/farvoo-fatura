package bootstrap

import (
	"strings"
	"testing"
)

func TestAdminMultiPC_NoAllowNextCopyAndSingleWizard(t *testing.T) {
	for _, bad := range []string{"允许下一台", "Allow next", "Permitir seguinte"} {
		if strings.Contains(adminHTML, bad) || strings.Contains(string(fiscalUIAdminI18nJS), bad) {
			t.Fatalf("forbidden copy %q still present", bad)
		}
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "settings.terminals.code_hint") {
		t.Fatal("retired code_hint key must not remain; wizard_meta is the only expiry copy")
	}
	if strings.Count(adminHTML, "function startAddTerminalWizard") != 1 {
		t.Fatal("ONLY startAddTerminalWizard for minting pair codes")
	}
	if strings.Count(adminHTML, "function refreshLanAccessPanel") != 1 {
		t.Fatal("ONLY refreshLanAccessPanel for LAN UI")
	}
	if strings.Count(adminHTML, "function saveLanAccess") != 1 {
		t.Fatal("ONLY saveLanAccess for LAN save")
	}
	if strings.Contains(adminHTML, "function allowNextTerminal") {
		t.Fatal("allowNextTerminal must be removed")
	}
	if !strings.Contains(adminHTML, `id="lanAccessAllow"`) || !strings.Contains(adminHTML, `id="terminalsAddWizard"`) {
		t.Fatal("LAN toggle + add-PC wizard required")
	}
	if strings.Count(adminHTML, "POST', '/local/v1/setup/terminals/allow-next'") != 1 {
		t.Fatalf("want exactly one allow-next call site, got %d",
			strings.Count(adminHTML, "POST', '/local/v1/setup/terminals/allow-next'"))
	}
	if strings.Count(adminHTML, "PUT', '/local/v1/setup/lan-access'") != 1 {
		t.Fatal("ONLY one lan-access PUT call site")
	}
}
