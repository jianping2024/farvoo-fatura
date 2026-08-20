package compliance

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// MustDecimal parses a decimal string; panics only in tests — production uses ParseDecimal.
func ParseDecimal(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("compliance: invalid decimal %q: %w", s, err)
	}
	return d, nil
}

// Money2 formats with exactly two decimal places (GrossTotal / Hash / QR).
func Money2(d decimal.Decimal) string {
	return d.StringFixed(2)
}

// LineFromGross splits a gross line amount into net + tax given VAT rate (e.g. "0.23").
func LineFromGross(qty, unitGross, vatRate string) (lineGross, lineNet, lineTax, unitNet string, err error) {
	q, err := ParseDecimal(qty)
	if err != nil {
		return "", "", "", "", err
	}
	ug, err := ParseDecimal(unitGross)
	if err != nil {
		return "", "", "", "", err
	}
	rate, err := ParseDecimal(vatRate)
	if err != nil {
		return "", "", "", "", err
	}
	gross := q.Mul(ug).Round(2)
	// net = gross / (1+rate)
	onePlus := decimal.NewFromInt(1).Add(rate)
	net := gross.Div(onePlus).Round(2)
	tax := gross.Sub(net).Round(2)
	un := ug.Div(onePlus).Round(2)
	return Money2(gross), Money2(net), Money2(tax), Money2(un), nil
}

// TaxCodeForRate maps mainland PT VAT rate to SAF-T tax code.
func TaxCodeForRate(vatRate string) string {
	switch vatRate {
	case "0.06":
		return "RED"
	case "0.13":
		return "INT"
	case "0.23":
		return "NOR"
	case "0", "0.00":
		return "ISE"
	default:
		return "NOR"
	}
}
