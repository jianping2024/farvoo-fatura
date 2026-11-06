package domain

import (
	"errors"
	"strings"
	"testing"
)

func saftTextReason(t *testing.T, err error) string {
	t.Helper()
	var te *SAFTTextError
	if !errors.As(err, &te) {
		t.Fatalf("want *SAFTTextError, got %v", err)
	}
	return te.Reason
}

func TestValidateStatusReason(t *testing.T) {
	if _, err := ValidateStatusReason("  "); saftTextReason(t, err) != SAFTTextTooShort {
		t.Fatalf("blank: %v", err)
	}
	ok := strings.Repeat("é", SAFTMaxReason) // 50 characters, 100 bytes
	if r, err := ValidateStatusReason(" " + ok + " "); err != nil || r != ok {
		t.Fatalf("50 chars: %q %v", r, err)
	}
	if _, err := ValidateStatusReason(ok + "x"); saftTextReason(t, err) != SAFTTextTooLong {
		t.Fatalf("51 chars: %v", err)
	}
	if _, err := ValidateStatusReason("退货"); saftTextReason(t, err) != SAFTTextEncoding {
		t.Fatalf("cjk: %v", err)
	}
}

func TestValidateSAFTText(t *testing.T) {
	for _, tc := range []struct {
		name, in, want, reason string
		min, max               int
	}{
		{name: "portuguese accents", in: " Chá de limão, Pão & Açúcar ", want: "Chá de limão, Pão & Açúcar", min: 2, max: 200},
		{name: "euro and nbsp", in: "Menu 9,50 €", want: "Menu 9,50 €", min: 2, max: 200},
		{name: "nfd input composes", in: "Pão", want: "Pão", min: 2, max: 200},
		{name: "two chars", in: "Ab", want: "Ab", min: 2, max: 200},
		{name: "200 chars", in: strings.Repeat("ã", 200), want: strings.Repeat("ã", 200), min: 2, max: 200},
		{name: "optional empty", in: "  ", want: "", min: 0, max: 50},
		{name: "cjk", in: "金叉套餐", reason: SAFTTextEncoding, min: 2, max: 200},
		{name: "cjk single char", in: "茶", reason: SAFTTextEncoding, min: 2, max: 200},
		{name: "emoji", in: "Bolo 🎂", reason: SAFTTextEncoding, min: 2, max: 200},
		{name: "one char", in: " P ", reason: SAFTTextTooShort, min: 2, max: 200},
		{name: "required empty", in: "", reason: SAFTTextTooShort, min: 1, max: 50},
		{name: "201 chars", in: strings.Repeat("ã", 201), reason: SAFTTextTooLong, min: 2, max: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateSAFTText("f", tc.in, tc.min, tc.max)
			if tc.reason != "" {
				if r := saftTextReason(t, err); r != tc.reason {
					t.Fatalf("reason %q, want %q", r, tc.reason)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestSaleSnapshotValidateSAFTText(t *testing.T) {
	snap := SaleSnapshot{
		Customer: CustomerInput{CompanyName: " Cliente  Lda ", City: "Lisboa"},
		Lines: []SaleLine{
			{ProductCode: "tea", DisplayName: "花茶套餐", SaftName: "Chá sortido"},
			{ProductCode: "", DisplayName: "Pão"},
		},
	}
	got, err := snap.ValidateSAFTText()
	if err != nil {
		t.Fatal(err)
	}
	if got.Customer.CompanyName != "Cliente  Lda" || got.Lines[1].SaftName != "Pão" || got.Lines[1].ProductCode != "" {
		t.Fatalf("cleaned: %+v", got)
	}
	if snap.Lines[1].SaftName != "" {
		t.Fatal("input snapshot lines mutated")
	}

	snap.Lines[1].DisplayName = "金叉套餐"
	_, err = snap.ValidateSAFTText()
	var te *SAFTTextError
	if !errors.As(err, &te) || te.Field != "lines.2.saft_name" || te.Reason != SAFTTextEncoding {
		t.Fatalf("display fallback cjk: %v", err)
	}

	snap.Lines[1].DisplayName = "Pão"
	snap.Customer.City = "里斯本"
	_, err = snap.ValidateSAFTText()
	if !errors.As(err, &te) || te.Field != "customer.city" {
		t.Fatalf("customer city: %v", err)
	}
}

func TestAddressValidateSAFTText(t *testing.T) {
	a, err := Address{Detail: " Rua A 1 ", City: "Porto", PostalCode: "4000-001"}.ValidateSAFTText("ship_to")
	if err != nil || a.Detail != "Rua A 1" {
		t.Fatalf("%+v %v", a, err)
	}
	_, err = Address{Detail: "仓库", City: "Porto", PostalCode: "4000-001"}.ValidateSAFTText("ship_from")
	var te *SAFTTextError
	if !errors.As(err, &te) || te.Field != "ship_from.address_detail" || te.Reason != SAFTTextEncoding {
		t.Fatalf("cjk detail: %v", err)
	}
}
