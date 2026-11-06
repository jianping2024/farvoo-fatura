package domain

import (
	"fmt"
	"strings"
)

// EffectiveSaftName is the line's SAF-T ProductDescription: saft_name, else display_name.
// The ONLY fallback (store.buildLines and SaleSnapshot.ValidateSAFTText).
func (l SaleLine) EffectiveSaftName() string {
	if strings.TrimSpace(l.SaftName) != "" {
		return l.SaftName
	}
	return l.DisplayName
}

// ValidateSAFTText checks every SAF-T text of a new sale (buyer + lines) before signing and
// returns the snapshot with the cleaned text. Line fields are lines.<n>.saft_name /
// lines.<n>.product_code (n from 1). An empty product_code passes (store numbers it LINEn).
func (s SaleSnapshot) ValidateSAFTText() (SaleSnapshot, error) {
	c, err := s.Customer.ValidateSAFTText("customer")
	if err != nil {
		return s, err
	}
	s.Customer = c
	lines := make([]SaleLine, len(s.Lines))
	for i, ln := range s.Lines {
		prefix := fmt.Sprintf("lines.%d.", i+1)
		if ln.SaftName, err = ValidateSAFTText(prefix+"saft_name", ln.EffectiveSaftName(), SAFTMinProductDesc, SAFTMaxProductDesc); err != nil {
			return s, err
		}
		if ln.ProductCode, err = ValidateSAFTText(prefix+"product_code", ln.ProductCode, 0, SAFTMaxProductCode); err != nil {
			return s, err
		}
		lines[i] = ln
	}
	s.Lines = lines
	return s, nil
}

// ValidateSAFTText checks the buyer fields written to SAF-T Customer; role prefixes the field
// (customer.company_name, …). Empty fields pass: issuance fills SAF-T defaults.
func (c CustomerInput) ValidateSAFTText(role string) (CustomerInput, error) {
	for _, f := range []struct {
		name string
		v    *string
		max  int
	}{
		{"company_name", &c.CompanyName, SAFTMaxCompanyName},
		{"address_detail", &c.AddressDetail, SAFTMaxAddressDetail},
		{"city", &c.City, SAFTMaxCity},
		{"postal_code", &c.PostalCode, SAFTMaxPostalCode},
	} {
		t, err := ValidateSAFTText(role+"."+f.name, *f.v, 0, f.max)
		if err != nil {
			return c, err
		}
		*f.v = t
	}
	return c, nil
}

// ValidateSAFTText checks a GR/GT load/unload point; role is ship_from | ship_to.
func (a Address) ValidateSAFTText(role string) (Address, error) {
	for _, f := range []struct {
		name string
		v    *string
		max  int
	}{
		{"address_detail", &a.Detail, SAFTMaxAddressDetail},
		{"city", &a.City, SAFTMaxCity},
		{"postal_code", &a.PostalCode, SAFTMaxPostalCode},
	} {
		t, err := ValidateSAFTText(role+"."+f.name, *f.v, 0, f.max)
		if err != nil {
			return a, err
		}
		*f.v = t
	}
	return a, nil
}
