package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// SAF-T(PT) 1.04_01 XSD text limits in characters — the ONLY table. Entry validation
// (ValidateSAFTText callers) and the SAF-T writer (saft.writeText) both read it.
const (
	SAFTMaxReason         = 50  // DocumentStatus/Reason, Line/References/Reason
	SAFTMaxCompanyName    = 100 // Header and Customer CompanyName
	SAFTMaxBusinessName   = 60
	SAFTMaxAddressDetail  = 210
	SAFTMaxCity           = 50
	SAFTMaxPostalCode     = 20
	SAFTMaxID             = 30 // CustomerID, CustomerTaxID, AccountID, SourceID
	SAFTMaxProductCode    = 60
	SAFTMaxProductDesc    = 200 // ProductDescription (min 2)
	SAFTMinProductDesc    = 2
	SAFTMaxLineDesc       = 200 // Line/Description
	SAFTMaxReference      = 60  // References/Reference, OriginatingON
	SAFTMaxUnitOfMeasure  = 20
	SAFTMaxTaxDescription = 255 // TaxTableEntry/Description
)

// SAFTTextError.Reason values.
const (
	SAFTTextEncoding = "encoding"  // a character Windows-1252 cannot encode (CJK, emoji, …)
	SAFTTextTooShort = "too_short" // fewer than Min characters
	SAFTTextTooLong  = "too_long"  // more than Max characters
)

// SAFTTextError names which free-text field failed ValidateSAFTText and why.
// Field is a dotted path chosen by the caller: product.saft_name, lines.3.saft_name,
// customer.city, ship_to.address_detail, taxpayer.legal_name, reason, …
type SAFTTextError struct {
	Field  string
	Reason string
	Min    int
	Max    int
}

func (e *SAFTTextError) Error() string {
	return fmt.Sprintf("saft text %s: %s (Windows-1252, %d-%d characters)", e.Field, e.Reason, e.Min, e.Max)
}

// EncodesWindows1252 reports whether every character of s exists in Windows-1252, the SAF-T
// file encoding. The ONLY encodability test (entry validation and saft.writeText).
func EncodesWindows1252(s string) bool {
	_, err := charmap.Windows1252.NewEncoder().String(s)
	return err == nil
}

// ValidateSAFTText is the ONLY rule for free text that reaches SAF-T: NFC (so a decomposed
// "a" + U+0303 from macOS input is the encodable "ã") and trim, then Windows-1252, then
// min..max characters. min 0 means optional: empty passes. Returns the text to store.
func ValidateSAFTText(field, s string, min, max int) (string, error) {
	t := strings.TrimSpace(norm.NFC.String(s))
	n := utf8.RuneCountInString(t)
	if n == 0 && min == 0 {
		return "", nil
	}
	reason := ""
	switch {
	case !EncodesWindows1252(t):
		reason = SAFTTextEncoding
	case n < min:
		reason = SAFTTextTooShort
	case n > max:
		reason = SAFTTextTooLong
	}
	if reason != "" {
		return "", &SAFTTextError{Field: field, Reason: reason, Min: min, Max: max}
	}
	return t, nil
}

// ValidateStatusReason is the ONLY rule for reasons that reach SAF-T Reason (annul GR/GT/PF,
// NC/ND correction reason). Returns the cleaned reason.
func ValidateStatusReason(reason string) (string, error) {
	return ValidateSAFTText("reason", reason, 1, SAFTMaxReason)
}
