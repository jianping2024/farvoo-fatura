package domain

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// SAF-T(PT) 1.04_01 XSD text limits in characters — the ONLY table. Entry validation (status and
// correction reasons) and the SAF-T writer (saft.writeText) both read it.
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

var (
	ErrReasonRequired = errors.New("reason required")
	ErrReasonTooLong  = errors.New("reason length must be 1-50")
)

// ValidateStatusReason is the ONLY rule for reasons that reach SAF-T Reason (annul GR/GT/PF,
// NC/ND correction reason). Returns the trimmed reason.
func ValidateStatusReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "", ErrReasonRequired
	}
	if utf8.RuneCountInString(r) > SAFTMaxReason {
		return "", ErrReasonTooLong
	}
	return r, nil
}
