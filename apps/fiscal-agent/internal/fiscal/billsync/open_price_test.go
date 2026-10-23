package billsync_test

import (
	"errors"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
)

func TestValidateAndDedupeProducts_OpenPrice(t *testing.T) {
	takeaway := func(price string, open bool) billsync.Line {
		return billsync.Line{
			ItemCode:       "TAKEAWAY",
			Name:           "TAKE AWAY (PAGA À PARTE).",
			Qty:            "1.00",
			UnitPriceGross: price,
			LineGross:      price,
			VATRate:        "13.00",
			OpenPrice:      open,
		}
	}

	tests := []struct {
		name         string
		lines        []billsync.Line
		wantConflict bool
		wantProducts int
		wantPrice    string
	}{
		{
			name:         "open price lines with different prices share one product",
			lines:        []billsync.Line{takeaway("0.10", true), takeaway("0.20", true), takeaway("0.30", true)},
			wantProducts: 1,
			wantPrice:    "0.10",
		},
		{
			name:         "catalog lines with different prices still conflict",
			lines:        []billsync.Line{takeaway("0.10", false), takeaway("0.20", false)},
			wantConflict: true,
		},
		{
			name:         "mixed flag still conflicts",
			lines:        []billsync.Line{takeaway("0.10", true), takeaway("0.20", false)},
			wantConflict: true,
		},
		{
			name: "open price with different vat conflicts",
			lines: func() []billsync.Line {
				b := takeaway("0.20", true)
				b.VATRate = "23.00"
				return []billsync.Line{takeaway("0.10", true), b}
			}(),
			wantConflict: true,
		},
		{
			name: "open price with different name conflicts",
			lines: func() []billsync.Line {
				b := takeaway("0.20", true)
				b.Name = "Other"
				return []billsync.Line{takeaway("0.10", true), b}
			}(),
			wantConflict: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			products, err := billsync.ValidateAndDedupeProducts(tc.lines)
			if tc.wantConflict {
				var ie *billsync.IngestError
				if !errors.As(err, &ie) || ie.Code != billsync.CodeItemCodeConflict {
					t.Fatalf("want item_code_conflict, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(products) != tc.wantProducts {
				t.Fatalf("products = %d, want %d", len(products), tc.wantProducts)
			}
			if products[0].UnitPriceGross != tc.wantPrice {
				t.Fatalf("price = %s, want %s", products[0].UnitPriceGross, tc.wantPrice)
			}
		})
	}
}
