package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

func TestWizardTextEncodingDropdownInBothUIs(t *testing.T) {
	for name, html := range map[string]string{
		"configure_ui.html": string(configureUIHTML),
		"setup_ui.html":     string(setupUIHTML),
	} {
		for _, want := range []string{
			`id="textEncoding"`, `id="textEncodingLabel"`, `id="textEncodingHint"`,
			`id="textEncodingErr"`, `id="textEncodingOk"`,
			"t('text_encoding_label')", "t('text_encoding_hint')",
			"MesaWizardUI.fillTextEncodingSelect", "MesaWizardUI.bindTextEncodingSelect",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s missing %s", name, want)
			}
		}
	}
}

func TestWizardUISharedTextEncodingOptions(t *testing.T) {
	js := string(wizardUISharedJS)
	if !strings.Contains(js, "['auto', 'han_firmware', 'utf8', 'latin']") {
		t.Fatal("TEXT_ENCODING_OPTIONS must list auto first, then han_firmware, utf8, latin")
	}
	for _, want := range []string{
		"'text_encoding_' + v", "'/api/ui-locale'", "text_encoding:", "'text_encoding_saved'",
		"fillTextEncodingSelect:", "postTextEncoding:", "bindTextEncodingSelect:",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("wizard_ui_shared.js missing %s", want)
		}
	}
}

func TestTextEncodingI18nKeys(t *testing.T) {
	for _, loc := range []string{"zh", "en", "pt"} {
		bundle := uiBundleMap(loc)
		for _, key := range []string{
			"text_encoding_label", "text_encoding_hint", "text_encoding_saved",
			"text_encoding_auto", "text_encoding_han_firmware", "text_encoding_utf8", "text_encoding_latin",
		} {
			if strings.TrimSpace(bundle[key]) == "" || bundle[key] == key {
				t.Fatalf("locale %s missing %s", loc, key)
			}
		}
	}
}

func TestWizardTextEncodingRoundTrip(t *testing.T) {
	t.Cleanup(func() { fiscalprint.SetThermalEncoding("auto") })
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := reloadConfig(path, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, path, cfgPtr, nil, nil)

	state := func() string {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/configure-state", nil))
		var j struct {
			TextEncoding string `json:"text_encoding"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		return j.TextEncoding
	}
	if got := state(); got != "auto" {
		t.Fatalf("default text_encoding=%q want auto", got)
	}

	for _, enc := range []string{"han_firmware", "utf8", "latin", "auto"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/ui-locale", strings.NewReader(`{"text_encoding":"`+enc+`"}`))
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status %d body %s", enc, rr.Code, rr.Body.String())
		}
		if got := state(); got != enc {
			t.Fatalf("after save %s: state text_encoding=%q", enc, got)
		}
		disk, err := loadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := normalizeTextEncoding(disk.TextEncoding); got != enc {
			t.Fatalf("after save %s: config.json text_encoding=%q", enc, got)
		}
	}
}
