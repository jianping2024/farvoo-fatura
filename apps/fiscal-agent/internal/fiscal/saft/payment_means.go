package saft

import (
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"strings"
)

// PaymentMechanism is the ONLY mapper from domain payment_method → SAF-T PaymentMechanism.
func PaymentMechanism(method string) string {
	switch domain.NormalizePaymentMethod(method) {
	case domain.PaymentCash:
		return "NU"
	case domain.PaymentCard:
		return "CC"
	case domain.PaymentMBWay, domain.PaymentMultibanco:
		return "MB"
	case domain.PaymentMixed, domain.PaymentOther, domain.PaymentAccount:
		return "OU"
	default:
		m := strings.TrimSpace(method)
		if m == "" {
			return "NU"
		}
		return "OU"
	}
}
