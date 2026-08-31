package domain

import "strings"

const (
	PaymentCash        = "CASH"
	PaymentCard        = "CARD"
	PaymentMBWay       = "MBWAY"
	PaymentMultibanco  = "MULTIBANCO"
	PaymentMixed       = "MIXED"
	PaymentOther       = "OTHER"
)

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
	switch NormalizePaymentMethod(m) {
	case PaymentCash, PaymentCard, PaymentMBWay, PaymentMultibanco, PaymentMixed, PaymentOther:
		return true
	default:
		return false
	}
}
