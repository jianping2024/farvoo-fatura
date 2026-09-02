package main

import (
	"strings"
	"testing"
)

func TestIsWizardStaticPath(t *testing.T) {
	if !isWizardStaticPath("/wizard-ui-shared.js") {
		t.Fatal("expected wizard shared js path")
	}
	if isWizardStaticPath("/api/setup") {
		t.Fatal("api paths are not static wizard assets")
	}
}

func TestWizardUISharedJSEmbedded(t *testing.T) {
	body := string(wizardUISharedJS)
	if !strings.Contains(body, "MesaWizardUI") {
		t.Fatal("missing MesaWizardUI export")
	}
	if !strings.Contains(body, "postSetup") {
		t.Fatal("missing postSetup helper")
	}
	if !strings.Contains(body, "force_takeover") {
		t.Fatal("missing force_takeover in postSetup")
	}
	if !strings.Contains(body, "save_auth_revoked") {
		t.Fatal("missing save_auth_revoked key reference")
	}
}

func TestConfigureUIUsesSharedWizardJS(t *testing.T) {
	html := string(configureUIHTML)
	if !strings.Contains(html, "/wizard-ui-shared.js") {
		t.Fatal("configure_ui.html must load /wizard-ui-shared.js")
	}
	if strings.Contains(html, "function formatSaveError") {
		t.Fatal("duplicate formatSaveError should live in wizard_ui_shared.js")
	}
}

func TestSetupUIUsesSharedWizardJS(t *testing.T) {
	html := string(setupUIHTML)
	if !strings.Contains(html, "/wizard-ui-shared.js") {
		t.Fatal("setup_ui.html must load /wizard-ui-shared.js")
	}
	if strings.Contains(html, "function formatSaveError") {
		t.Fatal("duplicate formatSaveError should live in wizard_ui_shared.js")
	}
}

func TestSetupUINoNativeConfirm(t *testing.T) {
	html := string(setupUIHTML)
	if strings.Contains(html, "confirm(") {
		t.Fatal("setup_ui must not use native confirm(); use finishConfirmBlock")
	}
	if strings.Count(html, `id="finishConfirmBlock"`) != 1 {
		t.Fatal("finishConfirmBlock must exist exactly once")
	}
	if !strings.Contains(html, "function closeSetupPage") {
		t.Fatal("closeSetupPage must be the ONLY setup-done close path")
	}
}

func TestMappingSaveI18nKeys(t *testing.T) {
	for _, loc := range []string{"zh", "en", "pt"} {
		bundle := uiBundleMap(loc)
		for _, key := range []string{
			"save_ok", "save_cleared_ok", "save_need_mapping",
			"save_station_conflict_takeover_hint", "save_takeover_btn", "save_auth_revoked",
			"finish_close_anyway", "finish_back_to_test",
		} {
			if strings.TrimSpace(bundle[key]) == "" || bundle[key] == key {
				t.Fatalf("locale %s missing %s", loc, key)
			}
		}
	}
}
