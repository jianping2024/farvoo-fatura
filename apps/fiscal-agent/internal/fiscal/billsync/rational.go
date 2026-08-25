package billsync

import (
	"fmt"
	"math/big"
	"strings"
)

// Rational is a reduced rational quantity (num/den), aligned with restaurant rational-qty.
type Rational struct {
	Num int64 `json:"num"`
	Den int64 `json:"den"`
}

func gcdInt64(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// NormalizeRational reduces the fraction; den==0 → 0/1.
func NormalizeRational(r Rational) Rational {
	if r.Den == 0 {
		return Rational{Num: 0, Den: 1}
	}
	sign := int64(1)
	if (r.Num < 0) != (r.Den < 0) {
		sign = -1
	}
	an, ad := r.Num, r.Den
	if an < 0 {
		an = -an
	}
	if ad < 0 {
		ad = -ad
	}
	g := gcdInt64(an, ad)
	return Rational{Num: sign * (an / g), Den: ad / g}
}

func RationalFromInt(n int64) Rational {
	return NormalizeRational(Rational{Num: n, Den: 1})
}

func AddRationals(a, b Rational) Rational {
	a, b = NormalizeRational(a), NormalizeRational(b)
	return NormalizeRational(Rational{Num: a.Num*b.Den + b.Num*a.Den, Den: a.Den * b.Den})
}

func SumRationals(vals []Rational) Rational {
	out := Rational{Num: 0, Den: 1}
	for _, v := range vals {
		out = AddRationals(out, v)
	}
	return out
}

func CompareRationals(a, b Rational) int {
	a, b = NormalizeRational(a), NormalizeRational(b)
	diff := NormalizeRational(Rational{Num: a.Num*b.Den - b.Num*a.Den, Den: a.Den * b.Den})
	if diff.Num == 0 {
		return 0
	}
	if diff.Num < 0 {
		return -1
	}
	return 1
}

func RationalLTE(a, b Rational) bool { return CompareRationals(a, b) <= 0 }

// ParseQtyString parses "2", "1/2", "2 1/3" (restaurant-aligned subset).
func ParseQtyString(raw string) (Rational, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return RationalFromInt(1), nil
	}
	var whole, num, den int64
	if _, err := fmt.Sscanf(s, "%d %d/%d", &whole, &num, &den); err == nil && den > 0 {
		return NormalizeRational(Rational{Num: whole*den + num, Den: den}), nil
	}
	if _, err := fmt.Sscanf(s, "%d/%d", &num, &den); err == nil && den > 0 {
		return NormalizeRational(Rational{Num: num, Den: den}), nil
	}
	if _, err := fmt.Sscanf(s, "%d", &whole); err == nil {
		return RationalFromInt(whole), nil
	}
	// decimal fallback e.g. 0.5
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil && f >= 0 {
		r := new(big.Rat).SetFloat64(f)
		if r == nil {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
		}
		num64 := r.Num().Int64()
		den64 := r.Denom().Int64()
		return NormalizeRational(Rational{Num: num64, Den: den64}), nil
	}
	return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
}

func FormatRational(r Rational) string {
	r = NormalizeRational(r)
	if r.Den == 1 {
		return fmt.Sprintf("%d", r.Num)
	}
	sign := ""
	n := r.Num
	if n < 0 {
		sign = "-"
		n = -n
	}
	whole := n / r.Den
	rem := n % r.Den
	if whole > 0 && rem > 0 {
		return fmt.Sprintf("%s%d %d/%d", sign, whole, rem, r.Den)
	}
	if whole > 0 {
		return fmt.Sprintf("%s%d", sign, whole)
	}
	return fmt.Sprintf("%s%d/%d", sign, rem, r.Den)
}

// ValidateQtyParts mirrors restaurant validateQtyParts (whole + num/den; improper fraction rejected).
func ValidateQtyParts(whole, num, den string) (Rational, error) {
	w := strings.TrimSpace(whole)
	n := strings.TrimSpace(num)
	d := strings.TrimSpace(den)
	hasW, hasN, hasD := w != "", n != "", d != ""
	if !hasW && !hasN && !hasD {
		return Rational{}, ingestErr(CodeValidationFailed, "empty qty")
	}
	if hasN != hasD {
		return Rational{}, ingestErr(CodeValidationFailed, "missing_den")
	}
	var wholeN, numN, denN int64
	if hasW {
		if _, err := fmt.Sscanf(w, "%d", &wholeN); err != nil || wholeN < 0 {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid whole")
		}
	}
	if hasN {
		if _, err := fmt.Sscanf(n, "%d", &numN); err != nil || numN < 0 {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid num")
		}
		if _, err := fmt.Sscanf(d, "%d", &denN); err != nil {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid den")
		}
		if denN == 0 {
			return Rational{}, ingestErr(CodeValidationFailed, "zero_den")
		}
		if numN >= denN {
			return Rational{}, ingestErr(CodeValidationFailed, "improper_fraction")
		}
		return NormalizeRational(Rational{Num: wholeN*denN + numN, Den: denN}), nil
	}
	return RationalFromInt(wholeN), nil
}
