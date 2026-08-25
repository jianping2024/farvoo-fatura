// Package ptnif is the ONLY Portuguese buyer NIF validator for Fiscal Agent (Mod-11).
package ptnif

import (
	"fmt"
	"strings"
	"unicode"
)

// FinalConsumer is the AT Consumidor Final tax id (always accepted).
const FinalConsumer = "999999990"

// Digits keeps at most 9 decimal digits.
func Digits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
			if b.Len() == 9 {
				break
			}
		}
	}
	return b.String()
}

// Valid reports whether nif passes Portuguese Mod-11 (or is FinalConsumer).
// Empty is invalid here — callers treat empty as “use final consumer” separately.
func Valid(nif string) bool {
	nif = Digits(nif)
	if nif == FinalConsumer {
		return true
	}
	if len(nif) != 9 || nif[0] == '0' {
		return false
	}
	sum := 0
	for i := 0; i < 8; i++ {
		sum += int(nif[i]-'0') * (9 - i)
	}
	expected := 11 - (sum % 11)
	if expected >= 10 {
		expected = 0
	}
	return int(nif[8]-'0') == expected
}

// NormalizeBuyer validates a non-empty buyer NIF.
// Returns canonical 9 digits or a clear error (Chinese).
func NormalizeBuyer(raw string) (string, error) {
	nif := Digits(raw)
	if nif == "" {
		return "", fmt.Errorf("购方 NIF 无效")
	}
	if len(nif) != 9 {
		return "", fmt.Errorf("购方 NIF「%s」须为 9 位数字", strings.TrimSpace(raw))
	}
	if !Valid(nif) {
		return "", fmt.Errorf("购方 NIF「%s」校验位不正确；请核对号码", nif)
	}
	return nif, nil
}
