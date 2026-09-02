package domain

import "strings"

const (
	PaymentCash       = "CASH"
	PaymentCard       = "CARD"
	PaymentMBWay      = "MBWAY"
	PaymentMultibanco = "MULTIBANCO"
	PaymentMixed      = "MIXED"
	PaymentOther      = "OTHER"
)

// knownPaymentMethods is the ONLY ordered schema list for payment_method codes.
var knownPaymentMethods = []string{
	PaymentCash,
	PaymentCard,
	PaymentMBWay,
	PaymentMultibanco,
	PaymentMixed,
	PaymentOther,
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
