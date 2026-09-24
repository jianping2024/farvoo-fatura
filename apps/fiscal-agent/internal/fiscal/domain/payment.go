package domain

import "strings"

const (
	PaymentCash       = "CASH"
	PaymentCard       = "CARD"
	PaymentMBWay      = "MBWAY"
	PaymentMultibanco = "MULTIBANCO"
	PaymentMixed      = "MIXED"
	PaymentOther      = "OTHER"
	PaymentAccount    = "ACCOUNT" // FT credit sale; does NOT settle at issue (RG later)
)

// knownPaymentMethods is the ONLY ordered schema list for payment_method codes.
var knownPaymentMethods = []string{
	PaymentCash,
	PaymentCard,
	PaymentMBWay,
	PaymentMultibanco,
	PaymentMixed,
	PaymentOther,
	PaymentAccount,
}

// IsSettlingPaymentMethod reports whether a payment counts toward settled-at-issue.
// ACCOUNT is hang-account only; real settlement is via later RG (or non-ACCOUNT at issue).
func IsSettlingPaymentMethod(m string) bool {
	return NormalizePaymentMethod(m) != PaymentAccount
}

// KnownPaymentMethods returns schema-allowed payment_method codes (copy).
func KnownPaymentMethods() []string {
	out := make([]string, len(knownPaymentMethods))
	copy(out, knownPaymentMethods)
	return out
}

// NormalizePaymentMethod uppercases and defaults empty to CASH.
func NormalizePaymentMethod(m string) string {
	m = strings.ToUpper(strings.TrimSpace(m))
	if m == "" {
		return PaymentCash
	}
	return m
}

// IsKnownPaymentMethod reports schema-allowed payment_method values.
func IsKnownPaymentMethod(m string) bool {
	m = NormalizePaymentMethod(m)
	for _, k := range knownPaymentMethods {
		if m == k {
			return true
		}
	}
	return false
}
