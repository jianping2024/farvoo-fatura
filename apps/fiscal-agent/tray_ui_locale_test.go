//go:build windows

package main

import "testing"

func TestUILocaleOptionTitleMarksCurrent(t *testing.T) {
	if got := uiLocaleOptionTitle("zh", "zh"); got != "✓ "+uiT("zh", "menu_ui_locale_opt_zh") {
		t.Fatalf("zh mark: %q", got)
	}
	if got := uiLocaleOptionTitle("zh", "en"); got != uiT("zh", "menu_ui_locale_opt_en") {
		t.Fatalf("en unmarked: %q", got)
	}
}
