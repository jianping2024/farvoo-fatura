package billsync

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func TestApplyPaymentOverride(t *testing.T) {
	sale := &domain.SaleSnapshot{
		Payments: []domain.PaymentInput{{Method: domain.PaymentCash, Amount: "12.50"}},
	}
	if err := ApplyPaymentOverride(sale, "card"); err != nil {
		t.Fatal(err)
	}
	if len(sale.Payments) != 1 || sale.Payments[0].Method != domain.PaymentCard || sale.Payments[0].Amount != "12.50" {
		t.Fatalf("got %+v", sale.Payments)
	}
	if err := ApplyPaymentOverride(sale, ""); err != nil {
		t.Fatal(err)
	}
	if sale.Payments[0].Method != domain.PaymentCash {
		t.Fatalf("empty → CASH, got %q", sale.Payments[0].Method)
	}
	if err := ApplyPaymentOverride(sale, "BITCOIN"); err == nil {
		t.Fatal("want unknown payment error")
	}
	if err := ApplyPaymentOverride(nil, "CASH"); err == nil {
		t.Fatal("want sale required")
	}
}
