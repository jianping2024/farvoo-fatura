package main

import (
	"regexp"
	"strings"
	"testing"
)

// Ensures configure_ui.html DOM ids match every getElementById() call (prevents silent null deref).
func TestConfigureUIDOMIDs(t *testing.T) {
	html := string(configureUIHTML)

	idRe := regexp.MustCompile(`\bid="([^"]+)"`)
	ids := map[string]bool{}
	for _, m := range idRe.FindAllStringSubmatch(html, -1) {
		ids[m[1]] = true
	}

	getRe := regexp.MustCompile(`getElementById\('([^']+)'\)`)
	seen := map[string]bool{}
	for _, m := range getRe.FindAllStringSubmatch(html, -1) {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		if !ids[id] {
			t.Errorf("getElementById(%q) has no matching id= in HTML", id)
		}
	}

	if ids["stations"] {
		t.Error(`use id="stationsRoot" for the station list container; id="stations" collides with the JS stations array name in some hosts`)
	}
	if !ids["stationsRoot"] {
		t.Error(`missing id="stationsRoot" container for station rows`)
	}
	if !strings.Contains(html, "phase stations_apply ok") {
		t.Error("missing phase log stations_apply ok")
	}
	if !strings.Contains(html, "phase stations_render") {
		t.Error("missing phase log stations_render")
	}
	if !strings.Contains(html, "phase printers_scan start") {
		t.Error("missing phase log printers_scan start")
	}
	if !strings.Contains(html, "phase printers_scan coalesce") {
		t.Error("missing phase log printers_scan coalesce (single-flight)")
	}
	if !strings.Contains(html, "phase loadPage skip auto scan") {
		t.Error("missing phase log loadPage skip auto scan when local maps exist")
	}
	if !strings.Contains(html, "phase printer_lists") {
		t.Error("missing phase log printer_lists (saved + scan merge)")
	}
	if !strings.Contains(html, "dedupe_skip") {
		t.Error("missing dedupe_skip in printer_lists log")
	}
}
