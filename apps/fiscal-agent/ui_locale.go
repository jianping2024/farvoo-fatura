package main

import "strings"

// UILocale controls agent UI (tray, configure/setup wizards, local test print only).
// It does not affect production print_jobs (those use payload.locale from Mesa).
func normalizeUILocale(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	case "pt", "pt-br", "por", "portuguese", "português":
		return "pt"
	default:
		return "zh"
	}
}

// normalizePrintLocale normalizes payload.locale from Mesa print jobs (default pt).
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

func printLocaleIsZh(locale string) bool {
	return normalizePrintLocale(locale) == "zh"
}

func (c *config) uiLocale() string {
	if c == nil {
		return "zh"
	}
	return normalizeUILocale(c.UILocale)
}

// testPrintPhrase is the headline printed on connection-test slips (must match UI hint).
func testPrintPhrase(locale string) string {
	return labelsFor(normalizeUILocale(locale)).connectionTest
}
