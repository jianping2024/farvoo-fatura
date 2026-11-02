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

// ValidatePT enforces the AT transport rule; field names the address in the error.
func (a Address) ValidatePT(field string) error {
	n := a.Normalize()
	switch {
	case n.Detail == "" || n.City == "":
		return fmt.Errorf("%w: %s needs street and city", ErrAddressIncomplete, field)
	case n.Country != "PT":
		return fmt.Errorf("%w: %s must be in Portugal (country %s)", ErrAddressIncomplete, field, n.Country)
	case !ptPostalCode.MatchString(n.PostalCode):
		return fmt.Errorf("%w: %s postal code %q must be NNNN-NNN", ErrAddressIncomplete, field, n.PostalCode)
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
