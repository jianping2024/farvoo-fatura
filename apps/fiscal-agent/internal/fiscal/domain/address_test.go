package domain

import (
	"errors"
	"testing"
)

func TestAddressValidatePT(t *testing.T) {
	ok := Address{Detail: " Rua  A 1 ", City: "Lisboa", PostalCode: "1000-001"}
	if err := ok.ValidatePT("ship_from"); err != nil {
		t.Fatal(err)
	}
	if got := ok.String(); got != "Rua A 1, 1000-001 Lisboa" {
		t.Fatalf("String: %q", got)
	}

	mustPart := func(a Address, field, part string) {
		t.Helper()
		err := a.ValidatePT(field)
		var ia *IncompleteAddressError
		if !errors.As(err, &ia) || !errors.Is(err, ErrAddressIncomplete) {
			t.Fatalf("%+v: want IncompleteAddressError, got %v", a, err)
		}
		if ia.Field != field || ia.Part != part {
			t.Fatalf("%+v: want field=%s part=%s, got field=%s part=%s", a, field, part, ia.Field, ia.Part)
		}
	}

	mustPart(Address{City: "Lisboa", PostalCode: "1000-001"}, "ship_to", "detail")
	mustPart(Address{Detail: "Rua A 1", PostalCode: "1000-001"}, "ship_to", "city")
	mustPart(Address{Detail: "Rua A 1", City: "Lisboa"}, "customer", "postal_code")
	mustPart(Address{Detail: "Rua A 1", City: "Lisboa", PostalCode: "311258"}, "ship_from", "postal_code")
	mustPart(Address{Detail: "Rua A 1", City: "Madrid", PostalCode: "1000-001", Country: "ES"}, "ship_to", "country")

	if !(Address{}).IsZero() || ok.IsZero() {
		t.Fatal("IsZero")
	}
}
