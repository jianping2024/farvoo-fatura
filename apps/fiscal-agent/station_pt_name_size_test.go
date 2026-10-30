package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/escposenc"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

func withStationPtNameSize(t *testing.T, size string) {
	t.Helper()
	body := `{"agentjwt":"x"}`
	if size != "" {
		body = fmt.Sprintf(`{"agentjwt":"x","station_pt_name_size":%q}`, size)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })
}

func TestNormalizeStationPtNameSize(t *testing.T) {
	for in, want := range map[string]string{
		"":     stationPtNameSize1x2,
		"1x2":  stationPtNameSize1x2,
		"1X2":  stationPtNameSize1x2,
		"2x2":  stationPtNameSize2x2,
		"2×2":  stationPtNameSize2x2,
		"bogus": stationPtNameSize1x2,
	} {
		if got := normalizeStationPtNameSize(in); got != want {
			t.Fatalf("normalize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestStationTicketFirstName2x2SameRowAsLead(t *testing.T) {
	withStationPtNameSize(t, "2x2")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines: []jobLine{{
			ItemCode:   "E11",
			Qty:        1,
			PrintNames: []string{"Sandwich Salmao", "Salmon tartare"},
		}},
	})
	lead := stationSlipQtyCodeLead(1, "E11")
	// Lead and first name share one 2×2 GS ! — no mid-row switch to 1×2 before the PT name.
	want := append(append([]byte{}, gsSize2x2...), escposenc.Windows1252(lead+" Sandwich Salmao")...)
	if !bytes.Contains(raw, want) {
		t.Fatal("2×2 first name must stay on the same GS ! 2×2 as the qty+code lead")
	}
	hang := strings.Repeat(" ", stationSlipNameHangIndent(lead, true)) + "Salmon tartare"
	idx := bytes.Index(raw, []byte(hang))
	if idx < 0 {
		t.Fatal("secondary name must hang under the 2×2 first-name column")
	}
	// Secondary is preceded by writeBody1x2 (GS ! 1×2); not still sitting on the lead's 2×2.
	prefix := raw[:idx]
	if !bytes.Contains(prefix[len(prefix)-16:], gsSize1x2) {
		t.Fatal("secondary name must print at 1×2")
	}
}

func TestStationTicketFirstNameDefaultStays1x2(t *testing.T) {
	withStationPtNameSize(t, "")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines:            []jobLine{{ItemCode: "E11", Qty: 1, PrintNames: []string{"Sandwich Salmao"}}},
	})
	if !bytes.Contains(raw, append(leadRow(1, "E11"), []byte(" Sandwich Salmao")...)) {
		t.Fatal("default station_pt_name_size must keep 2×2 lead then 1×2 name")
	}
}

func TestWizardStationPtNameSizeDropdownInBothUIs(t *testing.T) {
	for name, html := range map[string]string{
		"configure_ui.html": string(configureUIHTML),
		"setup_ui.html":     string(setupUIHTML),
	} {
		for _, want := range []string{
			`id="stationPtNameSize"`, `id="stationPtNameSizeLabel"`, `id="stationPtNameSizeHint"`,
			`id="stationPtNameSizeErr"`, `id="stationPtNameSizeOk"`,
			"t('station_pt_name_size_label')", "t('station_pt_name_size_hint')",
			"MesaWizardUI.fillStationPtNameSizeSelect", "MesaWizardUI.bindStationPtNameSizeSelect",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s missing %s", name, want)
			}
		}
	}
}

func TestWizardUISharedStationPtNameSizeOptions(t *testing.T) {
	js := string(wizardUISharedJS)
	if !strings.Contains(js, "['1x2', '2x2']") {
		t.Fatal("STATION_PT_NAME_SIZE_OPTIONS must be 1x2 then 2x2")
	}
	for _, want := range []string{
		"'station_pt_name_size_' + v", "station_pt_name_size:", "'station_pt_name_size_saved'",
		"fillStationPtNameSizeSelect:", "postStationPtNameSize:", "bindStationPtNameSizeSelect:",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("wizard_ui_shared.js missing %s", want)
		}
	}
}

func TestStationPtNameSizeI18nKeys(t *testing.T) {
	for _, loc := range []string{"zh", "en", "pt"} {
		bundle := uiBundleMap(loc)
		for _, key := range []string{
			"station_pt_name_size_label", "station_pt_name_size_hint", "station_pt_name_size_saved",
			"station_pt_name_size_1x2", "station_pt_name_size_2x2",
		} {
			if strings.TrimSpace(bundle[key]) == "" || bundle[key] == key {
				t.Fatalf("locale %s missing %s", loc, key)
			}
		}
	}
}

func TestWizardStationPtNameSizeRoundTrip(t *testing.T) {
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
			StationPtNameSize string `json:"station_pt_name_size"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		return j.StationPtNameSize
	}
	if got := state(); got != stationPtNameSize1x2 {
		t.Fatalf("default station_pt_name_size=%q want 1x2", got)
	}

	for _, size := range []string{"2x2", "1x2"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/ui-locale",
			strings.NewReader(`{"station_pt_name_size":"`+size+`"}`))
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status %d body %s", size, rr.Code, rr.Body.String())
		}
		if got := state(); got != size {
			t.Fatalf("after save %s: state=%q", size, got)
		}
		disk, err := loadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := normalizeStationPtNameSize(disk.StationPtNameSize); got != size {
			t.Fatalf("after save %s: config.json=%q", size, got)
		}
	}
}
