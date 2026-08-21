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

func payloadNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	if hasHan(p.RestaurantName) || hasHan(p.stationName()) {
		return true
	}
	for _, ln := range p.Lines {
		if hasHan(ln.DisplayName) || hasHan(ln.Note) {
			return true
		}
	}
	return false
}

// stationTicketNeedsBitmap — internal station slips use a fixed English header, but menu text may be Chinese.
func stationTicketNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	for _, ln := range p.Lines {
		if hasHan(ln.CategoryGroupHeader) || hasHan(ln.DisplayName) || hasHan(ln.Note) {
			return true
		}
	}
	return false
}

// printTicketLabels — station + receipt/pre-bill fixed chrome. zh → Chinese; else English
// (same rule for all ticket types; dish lines still come from payload).
func printTicketLabels(locale string) ticketLabels {
	if printLocaleIsZh(locale) {
		return labelsFor("zh")
	}
	return labelsFor("en")
}

// receiptTicketNeedsBitmap — receipt/pre-bill paper does not print restaurant_name; do not switch
// because of a Chinese venue name in payload.
func receiptTicketNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) {
		return true
	}
	if hasHan(formatSplitPayerForReceipt(p.PayerName)) {
		return true
	}
	for _, ln := range p.Lines {
		if hasHan(ln.DisplayName) || hasHan(ln.Note) {
			return true
		}
	}
	return false
}

// connectionTestNeedsBitmap — local test slips must render「打印测试」for zh UI even when venue is ASCII.
func connectionTestNeedsBitmap(p jobPayload) bool {
	if printLocaleIsZh(p.Locale) || normalizeUILocale(p.Locale) == "zh" {
		return true
	}
	return hasHan(p.venueName()) || hasHan(labelsFor(p.Locale).connectionTest)
}

// labelsASCII strips accents for printers when headers stay Latin (pt/en).
func labelsASCII(lab ticketLabels) ticketLabels {
	return ticketLabels{
		connectionTest: "TESTE IMPRESSAO",
		guestOrder:     "Pedido",
		receipt:        "Recibo",
		preBill:        "Pre-Conta",
		tableNo:        "Mesa n.",
		guest:          "Conv.",
		items:          "Artigos",
		qty:            "Qtd",
		originalPrice:  "Preco",
		feeDetails:     "Detalhe taxas",
		originalTotal:  "Preco original",
		subtotal:       "Subtotal",
		amountDue:      "A pagar",
		orderTime:      "Hora pedido",
		printedBy:      "Impresso por",
		printTime:      "Hora impressao",
		printedByVal:   "Cliente/Estabelecimento",
		orderedBy:      "Pedido por",
		amountPaid:     "Valor pago",
		station:        "Estacao",
	}
}

func encodeWindows1252(s string) []byte {
	return escposenc.Windows1252(s)
}
