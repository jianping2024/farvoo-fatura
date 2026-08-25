// Package vatpercent is the ONLY percent IVA normalizer for Fiscal Agent.
package vatpercent

import (
	"fmt"
	"strconv"
	"strings"
)

// Normalize accepts percent IVA as humans type it and returns canonical "23.00".
// Examples: "23", "23.0", "23.00" → "23.00".
// Rejects decimal rates like "0.23" and empty/non-numeric input.
func Normalize(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", fmt.Errorf("IVA 税率不能为空；请填百分数，例如 23 或 23.00")
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return "", fmt.Errorf("IVA 税率无效：「%s」；请填百分数，例如 23 或 23.00", raw)
	}
	if f < 0 || f > 100 {
		return "", fmt.Errorf("IVA 税率无效：「%s」；须在 0～100 之间", raw)
	}
	// Decimal rate mistaken for percent (e.g. 0.23 meaning 23%).
	if f > 0 && f < 1 {
		return "", fmt.Errorf("IVA 税率「%s」像是小数税率；请填百分数，例如 23 或 23.00（不要填 0.23）", raw)
	}
	return strconv.FormatFloat(f, 'f', 2, 64), nil
}
