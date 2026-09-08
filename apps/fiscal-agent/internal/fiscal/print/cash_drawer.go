package print

import (
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// NormalizeCashDrawerPin is the ONLY pin normalizer: 5 → pin5, everything else → pin2.
func NormalizeCashDrawerPin(pin int) int {
	if pin == 5 {
		return 5
	}
	return 2
}

// CashDrawerKickBytes is the ONLY ESC/POS drawer-kick builder (ESC p m t1 t2).
func CashDrawerKickBytes(pin int) []byte {
	m := byte(0) // pin2
	if NormalizeCashDrawerPin(pin) == 5 {
		m = 1 // pin5
	}
	return []byte{0x1B, 0x70, m, 0x19, 0xFA}
}

// AppendCashDrawerKick is the ONLY appender of drawer kick after receipt bytes.
func AppendCashDrawerKick(data []byte, pin int) []byte {
	return append(append([]byte(nil), data...), CashDrawerKickBytes(pin)...)
}

// PayloadIncludesCashTender is the ONLY cash-tender detector for drawer kick.
// True when any payment row is CASH or MIXED (UI single-code mixed).
func PayloadIncludesCashTender(p *Payload) bool {
	if p == nil {
		return false
	}
	if len(p.Payments) == 0 {
		return true // issue path freezes empty → CASH
	}
	for _, pay := range p.Payments {
		m := domain.NormalizePaymentMethod(pay.Method)
		if m == domain.PaymentCash || m == domain.PaymentMixed {
			return true
		}
	}
	return false
}

// ShouldAppendCashDrawerKick is the ONLY gate for auto kick on fiscal print.
// ORIGINAL FT/FS with cash tender only — not REPRINT, not NC/ND.
func ShouldAppendCashDrawerKick(p *Payload) bool {
	if p == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(p.PrintPurpose), string(domain.PrintOriginal)) {
		return false
	}
	dt := strings.ToUpper(strings.TrimSpace(p.DocumentType))
	if dt != "FT" && dt != "FS" {
		return false
	}
	return PayloadIncludesCashTender(p)
}
