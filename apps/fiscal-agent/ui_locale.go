package main

import (
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/locale"
)

// normalizeUILocale delegates to locale.NormalizeUILocale — ONLY normalize for UI.
func normalizeUILocale(raw string) string {
	return locale.NormalizeUILocale(raw)
}

// normalizePrintLocale normalizes payload.locale from Mesa print jobs (default pt).
// Kitchen/guest tickets only — not fiscal FT chrome (see locale.InvoiceLocaleFromUI).
func normalizePrintLocale(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	case "pt", "pt-br", "pt-pt", "por", "portuguese", "português":
		return "pt"
	case "zh", "zh-cn", "zh-hans", "zh-tw", "chinese", "cn":
		return "zh"
	case "":
		return "pt"
	default:
		return "pt"
	}
}

func printLocaleIsZh(loc string) bool {
	return normalizePrintLocale(loc) == "zh"
}

func (c *config) uiLocale() string {
	if c == nil {
		return "zh"
	}
	return normalizeUILocale(c.UILocale)
}

// testPrintPhrase is the headline printed on connection-test slips (must match UI hint).
func testPrintPhrase(loc string) string {
	return labelsFor(normalizeUILocale(loc)).connectionTest
}
