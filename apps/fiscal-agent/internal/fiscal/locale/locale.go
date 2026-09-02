// Package locale is the ONLY place for ui_locale normalize + invoice locale derivation (scheme A).
package locale

import "strings"

// NormalizeUILocale returns zh | en | pt (default zh).
func NormalizeUILocale(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	case "pt", "pt-br", "pt-pt", "por", "portuguese", "português":
		return "pt"
	default:
		return "zh"
	}
}

// InvoiceLocaleFromUI is the ONLY derivation of fiscal ticket chrome language from ui_locale (scheme A):
//
//	zh → pt  (Chinese UI; Portuguese invoice)
//	en → en
//	pt → pt
//
// Returns en | pt only (never zh on the fiscal ticket face).
func InvoiceLocaleFromUI(uiLocale string) string {
	switch NormalizeUILocale(uiLocale) {
	case "en":
		return "en"
	default:
		return "pt"
	}
}

// NormalizeInvoiceLocale coerces a frozen payload locale to en | pt (empty/unknown → pt).
func NormalizeInvoiceLocale(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	default:
		return "pt"
	}
}
