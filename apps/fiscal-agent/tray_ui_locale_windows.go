//go:build windows

package main

import "farvoo-fiscal-agent/internal/fiscal/locale"

func setTrayUILocale(code string) error {
	return setAgentUILocale(code)
}

func uiLocaleOptionTitle(menuLocale, option string) string {
	label := uiT(menuLocale, "menu_ui_locale_opt_"+option)
	if locale.NormalizeUILocale(menuLocale) == locale.NormalizeUILocale(option) {
		return "✓ " + label
	}
	return label
}

func uiLocaleOptionLogLabel(code string) string {
	code = locale.NormalizeUILocale(code)
	return uiT(code, "menu_ui_locale_opt_"+code)
}
