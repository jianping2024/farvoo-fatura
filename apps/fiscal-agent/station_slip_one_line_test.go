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

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

func withStationSlipPrintOpts(t *testing.T, nameOne, noteOne bool) {
	t.Helper()
	body := fmt.Sprintf(
		`{"agentjwt":"x","station_name_one_line":%t,"station_note_one_line":%t}`,
		nameOne, noteOne,
	)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = prev })
}

func TestStationSlipKeepLines(t *testing.T) {
	in := []string{"a", "b", "c"}
	if got := stationSlipKeepLines(in, false); len(got) != 3 {
		t.Fatalf("off: want 3 got %v", got)
	}
	got := stationSlipKeepLines(in, true)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("on: want [a] got %v", got)
	}
	if strings.Contains(got[0], "…") || strings.Contains(got[0], "...") {
		t.Fatal("must hard-discard without ellipsis")
	}
}

func TestStationTicketNameOneLineDiscardsWrap(t *testing.T) {
	long := strings.Repeat("Cafe ", 20) + "Extra"
	withStationSlipPrintOpts(t, false, false)
	rawWrap := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines:            []jobLine{{ItemCode: "E11", Qty: 1, PrintNames: []string{long}}},
	})
	if !bytes.Contains(rawWrap, []byte("Extra")) {
		t.Fatal("fixture must wrap so Extra appears when one-line is off")
	}
	withStationSlipPrintOpts(t, true, false)
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines:            []jobLine{{ItemCode: "E11", Qty: 1, PrintNames: []string{long}}},
	})
	if bytes.Contains(raw, []byte("Extra")) {
		t.Fatal("name one-line must discard overflow of Portuguese name")
	}
	if bytes.Contains(raw, []byte("…")) || bytes.Contains(raw, []byte("...")) {
		t.Fatal("must not append ellipsis")
	}
}

func TestStationTicketNoteOneLineDiscardsWrap(t *testing.T) {
	note := strings.Repeat("nota ", 30) + "XYZZY"
	withStationSlipPrintOpts(t, false, false)
	rawWrap := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines:            []jobLine{{ItemCode: "001", Qty: 1, PrintNames: []string{"Agua"}, Note: note}},
	})
	if !bytes.Contains(rawWrap, []byte("XYZZY")) {
		t.Fatal("fixture must wrap so XYZZY appears when note one-line is off")
	}
	withStationSlipPrintOpts(t, false, true)
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "07",
		Lines:            []jobLine{{ItemCode: "001", Qty: 1, PrintNames: []string{"Agua"}, Note: note}},
	})
	if bytes.Contains(raw, []byte("XYZZY")) {
		t.Fatal("note one-line must discard overflow without printing the tail marker")
	}
}

func TestWizardStationSlipPrintOptsRoundTrip(t *testing.T) {
	t.Cleanup(func() { fiscalprint.SetThermalEncoding("auto") })
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := reloadConfig(path, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, path, cfgPtr, nil, nil)

	state := func() (px int, nameOne, noteOne bool) {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/configure-state", nil))
		var j struct {
			HanBitmapFontPx    int  `json:"han_bitmap_font_px"`
			StationNameOneLine bool `json:"station_name_one_line"`
			StationNoteOneLine bool `json:"station_note_one_line"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		return j.HanBitmapFontPx, j.StationNameOneLine, j.StationNoteOneLine
	}
	px, nameOne, noteOne := state()
	if px != bitmapTextDefaultFontPx {
		t.Fatalf("default han px=%d want %d", px, bitmapTextDefaultFontPx)
	}
	if nameOne || noteOne {
		t.Fatal("one-line flags default false")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/ui-locale",
		strings.NewReader(`{"han_bitmap_font_px":28,"station_name_one_line":true,"station_note_one_line":true}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	px, nameOne, noteOne = state()
	if px != 28 || !nameOne || !noteOne {
		t.Fatalf("after save px=%d name=%v note=%v", px, nameOne, noteOne)
	}
	disk, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if disk.HanBitmapFontPx != 28 || !disk.StationNameOneLine || !disk.StationNoteOneLine {
		t.Fatalf("config.json %#v", disk)
	}
}

func TestWizardUIHasStationSlipPrintOpts(t *testing.T) {
	for name, html := range map[string]string{
		"configure_ui.html": string(configureUIHTML),
		"setup_ui.html":     string(setupUIHTML),
	} {
		for _, want := range []string{
			`id="hanBitmapFontPx"`, `id="stationNameOneLine"`, `id="stationNoteOneLine"`,
			"MesaWizardUI.bindHanBitmapFontPxInput", "MesaWizardUI.bindStationOneLineCheckbox",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s missing %s", name, want)
			}
		}
	}
	js := string(wizardUISharedJS)
	for _, want := range []string{
		"bindHanBitmapFontPxInput:", "bindStationOneLineCheckbox:",
		"HAN_BITMAP_FONT_PX_DEFAULT",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("wizard_ui_shared.js missing %s", want)
		}
	}
}

func TestStationSlipPrintOptsI18nKeys(t *testing.T) {
	for _, loc := range []string{"zh", "en", "pt"} {
		bundle := uiBundleMap(loc)
		for _, key := range []string{
			"han_bitmap_font_px_label", "han_bitmap_font_px_hint", "han_bitmap_font_px_saved",
			"station_name_one_line_label", "station_name_one_line_hint", "station_name_one_line_saved",
			"station_note_one_line_label", "station_note_one_line_hint", "station_note_one_line_saved",
		} {
			if strings.TrimSpace(bundle[key]) == "" || bundle[key] == key {
				t.Fatalf("locale %s missing %s", loc, key)
			}
		}
	}
}
