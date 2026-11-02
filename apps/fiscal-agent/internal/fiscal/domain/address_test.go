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
	for _, bad := range []Address{
		{Detail: "Rua A 1", City: "Lisboa"},                                        // no postal
		{Detail: "Rua A 1", City: "Hangzhou", PostalCode: "311258"},                // non-PT postal
		{Detail: "Rua A 1", PostalCode: "1000-001"},                                // no city
		{Detail: "Rua A 1", City: "Madrid", PostalCode: "1000-001", Country: "ES"}, // not PT
	} {
		if err := bad.ValidatePT("ship_to"); !errors.Is(err, ErrAddressIncomplete) {
			t.Fatalf("%+v: want ErrAddressIncomplete, got %v", bad, err)
		}
	}
	if !(Address{}).IsZero() || ok.IsZero() {
		t.Fatal("IsZero")
	}
}
