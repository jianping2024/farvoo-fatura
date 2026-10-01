package main

import (
	"regexp"
	"strings"
	"unicode"

	"farvoo-fiscal-agent/internal/escposenc"
)

var defaultGuestPayerRe = regexp.MustCompile(`(?i)^(客人|Guest|Pessoa)\s*(\d+)$`)

// formatSplitPayerForReceipt strips UI placeholder names (e.g. "客人 2") so Latin mode shows "Guest:2".
func formatSplitPayerForReceipt(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if m := defaultGuestPayerRe.FindStringSubmatch(name); len(m) == 3 {
		return m[2]
	}
	return name
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// stationTicketNeedsBitmap — firmware-safe when chrome/content has non-ASCII (pt accents, Han, …).
func stationTicketNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	if normalizePrintLocale(p.Locale) == "pt" {
		return true
	}
	for _, ln := range p.Lines {
		if escposenc.HasNonASCII(ln.CategoryGroupHeader) || escposenc.HasNonASCII(ln.DisplayName) || escposenc.HasNonASCII(ln.Note) {
			return true
		}
	}
	return false
}

// printTicketLabels — ONLY fixed chrome for Mesa thermal tickets (station / pre-bill / receipt).
func printTicketLabels(locale string) ticketLabels {
	return labelsFor(normalizePrintLocale(locale))
}

// receiptTicketNeedsBitmap — receipt/pre-bill; ignore restaurant_name Han for mode flip.
func receiptTicketNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	if normalizePrintLocale(p.Locale) == "pt" {
		return true
	}
	if escposenc.HasNonASCII(formatSplitPayerForReceipt(p.PayerName)) {
		return true
	}
	for _, ln := range p.Lines {
		if escposenc.HasNonASCII(ln.DisplayName) || escposenc.HasNonASCII(ln.Note) {
			return true
		}
	}
	return false
}

// connectionTestNeedsBitmap — test slips follow payload.locale.
func connectionTestNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	if normalizePrintLocale(p.Locale) == "pt" {
		return true
	}
	lab := printTicketLabels(p.Locale)
	return escposenc.HasNonASCII(p.venueName()) || escposenc.HasNonASCII(lab.connectionTest)
}

func encodeWindows1252(s string) []byte {
	return escposenc.Windows1252(s)
}
