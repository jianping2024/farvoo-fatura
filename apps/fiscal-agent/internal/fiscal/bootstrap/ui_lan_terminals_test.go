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
	if strings.Count(adminHTML, "function refreshTerminalsPanel") != 1 {
		t.Fatal("ONLY refreshTerminalsPanel for terminals UI")
	}
	if strings.Count(adminHTML, "function saveTerminalLabel") != 1 {
		t.Fatal("ONLY saveTerminalLabel for terminal label writes")
	}
	if strings.Count(adminHTML, "function mountTerminalLabelInput") != 1 {
		t.Fatal("ONLY mountTerminalLabelInput for label cells")
	}
	if strings.Count(adminHTML, "/label'") != 1 {
		t.Fatalf("want exactly one …/label PUT call site, got %d", strings.Count(adminHTML, "/label'"))
	}
	if strings.Contains(adminHTML, "电脑名称") {
		t.Fatal("column header must be 备注, not 电脑名称")
	}
	if !strings.Contains(string(fiscalUIAdminI18nJS), "'settings.terminals.need_label'") {
		t.Fatal("need_label i18n required")
	}
	if strings.Count(adminHTML, "function refreshEffectivePrintStation") != 1 {
		t.Fatal("ONLY refreshEffectivePrintStation for effective station")
	}
	if strings.Contains(adminHTML, `id="stationSel"`) || strings.Contains(adminHTML, `id="invStationSel"`) {
		t.Fatal("stationSel and invStationSel must be removed")
	}
	if !strings.Contains(adminHTML, "settings.printers.configure_link") {
		t.Fatal("configure_link i18n key required in printers section")
	}
	if n := strings.Count(adminHTML, "#terminalsTable.list-table { table-layout: fixed"); n != 1 {
		t.Fatalf("terminalsTable fixed layout must be the ONLY width rule, got %d", n)
	}
	if n := strings.Count(adminHTML, "#terminalsTable .col-station select"); n != 1 {
		t.Fatalf("station select fill CSS must exist once, got %d", n)
	}
	if strings.Contains(adminHTML, "input.style.maxWidth") || strings.Contains(adminHTML, "maxWidth = '10rem'") {
		t.Fatal("note input maxWidth must be CSS-only via .col-note input")
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "停止这台开票") {
		t.Fatal("revoke button copy must be 停用, not 停止这台开票")
	}
	if !strings.Contains(string(fiscalUIAdminI18nJS), "'settings.terminals.revoke_btn': '停用'") {
		t.Fatal("zh revoke_btn must be 停用")
	}
	if n := strings.Count(adminHTML, "function formatMdHms"); n != 1 {
		t.Fatalf("formatMdHms must be the ONLY MM-dd HH:mm:ss formatter, got %d", n)
	}
	if strings.Count(adminHTML, "formatMdHms(row.last_seen_at") != 1 {
		t.Fatal("terminals last-seen must use formatMdHms once")
	}
	if strings.Contains(adminHTML, "escHtml(row.last_seen_at || row.registered_at") {
		t.Fatal("do not dump raw ISO into terminals last-seen cell")
	}
}
