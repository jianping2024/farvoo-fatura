package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Address is a structured postal address — the ONLY address shape for GR/GT load/unload
// points (SAF-T AddressStructure, AT AddressStructurePT). Never parsed from free text.
type Address struct {
	Detail     string `json:"address_detail"`
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country,omitempty"` // ISO 3166-1 alpha-2; empty → PT
}

// ErrAddressIncomplete: AT rejects GR/GT (-1 "A morada … deve ser completa") unless street,
// city and a Portuguese NNNN-NNN postal code are present.
var ErrAddressIncomplete = errors.New("address incomplete")

// IncompleteAddressError names which address role and which part failed ValidatePT.
// Part is one of: detail | city | postal_code | country.
type IncompleteAddressError struct {
	Field string // customer | ship_from | ship_to (caller-chosen role key)
	Part  string
}

func (e *IncompleteAddressError) Error() string {
	if e == nil {
		return ErrAddressIncomplete.Error()
	}
	return fmt.Sprintf("%s: %s incomplete (%s)", ErrAddressIncomplete.Error(), e.Field, e.Part)
}

func (e *IncompleteAddressError) Unwrap() error { return ErrAddressIncomplete }

var ptPostalCode = regexp.MustCompile(`^\d{4}-\d{3}$`)

// Normalize trims fields, collapses inner whitespace and defaults Country to PT.
func (a Address) Normalize() Address {
	clean := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	a.Detail, a.City, a.PostalCode = clean(a.Detail), clean(a.City), clean(a.PostalCode)
	a.Country = strings.ToUpper(clean(a.Country))
	if a.Country == "" {
		a.Country = "PT"
	}
	return a
}

// IsZero reports an address with no street, city or postal code.
func (a Address) IsZero() bool {
	return strings.TrimSpace(a.Detail) == "" && strings.TrimSpace(a.City) == "" && strings.TrimSpace(a.PostalCode) == ""
}

// ValidatePT enforces the AT transport rule; field names the address role in the error.
// Checks parts in order: street → city → country → postal code (first failure wins).
func (a Address) ValidatePT(field string) error {
	n := a.Normalize()
	switch {
	case n.Detail == "":
		return &IncompleteAddressError{Field: field, Part: "detail"}
	case n.City == "":
		return &IncompleteAddressError{Field: field, Part: "city"}
	case n.Country != "PT":
		return &IncompleteAddressError{Field: field, Part: "country"}
	case !ptPostalCode.MatchString(n.PostalCode):
		return &IncompleteAddressError{Field: field, Part: "postal_code"}
	}
	return nil
}

// String formats the address for display / print ("Rua A 1, 1000-001 Lisboa").
func (a Address) String() string {
	n := a.Normalize()
	tail := strings.TrimSpace(n.PostalCode + " " + n.City)
	switch {
	case n.Detail == "":
		return tail
	case tail == "":
		return n.Detail
	default:
		return n.Detail + ", " + tail
	}
}
