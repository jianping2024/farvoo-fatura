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

// stationTicketNeedsBitmap — only when content goes beyond WPC1252 (Han, …); pt accents stay Font A.
func stationTicketNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	for _, ln := range p.Lines {
		if escposenc.NeedsRaster(ln.CategoryGroupHeader) || escposenc.NeedsRaster(ln.DisplayName) || escposenc.NeedsRaster(ln.Note) {
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
	if escposenc.NeedsRaster(formatSplitPayerForReceipt(p.PayerName)) {
		return true
	}
	for _, ln := range p.Lines {
		if escposenc.NeedsRaster(ln.DisplayName) || escposenc.NeedsRaster(ln.Note) {
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
	lab := printTicketLabels(p.Locale)
	return escposenc.NeedsRaster(p.venueName()) || escposenc.NeedsRaster(lab.connectionTest)
}

func encodeWindows1252(s string) []byte {
	return escposenc.Windows1252(s)
}
