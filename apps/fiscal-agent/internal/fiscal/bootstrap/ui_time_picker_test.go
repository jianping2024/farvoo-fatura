package bootstrap

import (
	"strings"
	"testing"
)

func TestFiscalUITimePickerAssets(t *testing.T) {
	js := string(fiscalUITimePickerJS)
	if !strings.Contains(js, "FiscalUI.createTimePicker") {
		t.Fatal("time-picker.js must export FiscalUI.createTimePicker")
	}
	if n := strings.Count(js, "function createTimePicker"); n != 1 {
		t.Fatalf("createTimePicker must be defined once, got %d", n)
	}
	if !strings.Contains(js, "FiscalUI.hmInTimezone") {
		t.Fatal("time-picker.js must export FiscalUI.hmInTimezone")
	}
	if strings.Contains(js, "type=\"time\"") || strings.Contains(js, "type='time'") {
		t.Fatal("time-picker must not use native input type=time")
	}
	if !strings.Contains(string(fiscalUITimePickerCSS), ".fiscal-time-picker__popup") {
		t.Fatal("time-picker.css must style portal popup")
	}
	if !strings.Contains(adminHTML, `/fiscal-ui/time-picker.js`) || !strings.Contains(adminHTML, `/fiscal-ui/time-picker.css`) {
		t.Fatal("admin must load fiscal-ui time-picker assets")
	}
	dateIdx := strings.Index(adminHTML, `/fiscal-ui/date-picker.js`)
	timeIdx := strings.Index(adminHTML, `/fiscal-ui/time-picker.js`)
	if dateIdx < 0 || timeIdx < 0 || dateIdx > timeIdx {
		t.Fatal("date-picker.js must load before time-picker.js")
	}
	if n := strings.Count(adminHTML, "FiscalUI.createTimePicker"); n != 1 {
		t.Fatalf("admin must mount createTimePicker once (movement start), got %d", n)
	}
	if n := strings.Count(adminHTML, "function ptAddressDefect"); n != 1 {
		t.Fatalf("ptAddressDefect must be the ONLY GR/GT address defect detector, got %d", n)
	}
	if n := strings.Count(adminHTML, "function reportTransportAddressDefect"); n != 1 {
		t.Fatalf("reportTransportAddressDefect must be the ONLY address toast+focus path, got %d", n)
	}
	if strings.Contains(adminHTML, "gr.need_postal_pt") || strings.Contains(string(fiscalUIAdminI18nJS), "'gr.need_postal_pt'") {
		t.Fatal("generic gr.need_postal_pt must be removed; use gr.addr.* role×part")
	}
	if strings.Contains(string(fiscalUIAdminI18nJS), "'gr.need_address'") || strings.Contains(string(fiscalUIAdminI18nJS), "'gt.need_address'") {
		t.Fatal("gr/gt.need_address must be removed; customer defects use gr.addr.*")
	}
	for _, key := range []string{
		"gr.addr.incomplete", "gr.addr.role.ship_to", "gr.addr.part.city",
		"gr.need_movement_date", "gr.need_movement_time", "gr.movement_time_ph",
	} {
		if !strings.Contains(string(fiscalUIAdminI18nJS), "'"+key+"'") {
			t.Fatalf("admin-i18n missing %s", key)
		}
	}
}
