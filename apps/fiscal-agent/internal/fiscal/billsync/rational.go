package billsync

import (
	"fmt"
	"math/big"
	"strconv"
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

// ParseQtyString is the ONLY qty-string → Rational parser for bill-sync.
// Accepts: "" (→1), integers, fractions ("1/2"), mixed ("2 1/3"), and
// contract decimal strings ("0.33", "1.00", "2.50"). Full-string only —
// never fmt.Sscanf("%d") (stops at '.' → "0.33" becomes 0 → share qty must be positive).
func ParseQtyString(raw string) (Rational, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return RationalFromInt(1), nil
	}
	if wholeStr, frac, cut := strings.Cut(s, " "); cut {
		whole, err := strconv.ParseInt(wholeStr, 10, 64)
		if err != nil || whole < 0 {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
		}
		frac = strings.TrimSpace(frac)
		// Mixed form is only "N n/d" (FormatRational); bare "1 2" is invalid.
		if frac == "" || !strings.Contains(frac, "/") || strings.ContainsAny(frac, " \t") {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
		}
		fr, ok := new(big.Rat).SetString(frac)
		if !ok || fr.Sign() < 0 {
			return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
		}
		return rationalFromBigRat(new(big.Rat).Add(big.NewRat(whole, 1), fr), s)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() < 0 {
		return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+s)
	}
	return rationalFromBigRat(r, s)
}

func rationalFromBigRat(r *big.Rat, raw string) (Rational, error) {
	if r == nil {
		return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+raw)
	}
	num, den := r.Num(), r.Denom()
	if !num.IsInt64() || !den.IsInt64() {
		return Rational{}, ingestErr(CodeValidationFailed, "invalid qty "+raw)
	}
	return NormalizeRational(Rational{Num: num.Int64(), Den: den.Int64()}), nil
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
