package store

import (
	"database/sql"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// shipAddressSelect is the ONLY column list for reading a GR/GT load (prefix "ship_from")
// or unload ("ship_to") point; scan it with scanShipAddress.
func shipAddressSelect(prefix string) string {
	return prefix + "_detail, " + prefix + "_city, " + prefix + "_postal_code, " + prefix + "_country, " + prefix + "_address"
}

// shipAddressCols holds one scanned shipAddressSelect.
type shipAddressCols struct {
	detail, city, postal, country, legacy sql.NullString
}

func (c *shipAddressCols) dest() []any {
	return []any{&c.detail, &c.city, &c.postal, &c.country, &c.legacy}
}

// address returns the structured value; pre-017 rows only carry free text (kept as Detail).
func (c *shipAddressCols) address() domain.Address {
	a := domain.Address{Detail: c.detail.String, City: c.city.String, PostalCode: c.postal.String, Country: c.country.String}
	if a.IsZero() && c.legacy.String != "" {
		a = domain.Address{Detail: c.legacy.String}
	}
	return a.Normalize()
}

// shipAddressArgs are the INSERT values for <prefix>_detail, _city, _postal_code, _country.
func shipAddressArgs(a domain.Address) []any {
	n := a.Normalize()
	return []any{n.Detail, n.City, n.PostalCode, n.Country}
}
