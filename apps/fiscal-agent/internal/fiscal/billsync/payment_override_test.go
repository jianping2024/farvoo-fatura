package billsync

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func TestApplyPaymentOverride(t *testing.T) {
	sale := &domain.SaleSnapshot{
		Payments: []domain.PaymentInput{{Method: domain.PaymentCash, Amount: "12.50"}},
	}
	if err := ApplyPaymentOverride(sale, "multibanco", nil); err != nil {
		t.Fatal(err)
	}
	if len(sale.Payments) != 1 || sale.Payments[0].Method != domain.PaymentMultibanco || sale.Payments[0].Amount != "12.50" {
		t.Fatalf("got %+v", sale.Payments)
	}
	if err := ApplyPaymentOverride(sale, "", nil); err != nil {
		t.Fatal(err)
	}
	if sale.Payments[0].Method != domain.PaymentCash {
		t.Fatalf("empty → CASH, got %q", sale.Payments[0].Method)
	}
	if err := ApplyPaymentOverride(sale, "BITCOIN", nil); err == nil {
		t.Fatal("want unknown payment error")
	}
	if err := ApplyPaymentOverride(nil, "CASH", nil); err == nil {
		t.Fatal("want sale required")
	}
}

func TestApplyPaymentOverride_MixedRequiresLines(t *testing.T) {
	sale := &domain.SaleSnapshot{
		Payments: []domain.PaymentInput{{Method: domain.PaymentCash, Amount: "20.00"}},
	}
	if err := ApplyPaymentOverride(sale, "MIXED", nil); err == nil {
		t.Fatal("MIXED without lines must fail-closed")
	}
	lines := []PaymentLine{
		{Method: "MULTIBANCO", Amount: "15.00"},
		{Method: "CASH", Amount: "5.00"},
	}
	if err := ApplyPaymentOverride(sale, "MIXED", lines); err != nil {
		t.Fatal(err)
	}
	if len(sale.Payments) != 2 {
		t.Fatalf("want 2 payments, got %+v", sale.Payments)
	}
	if sale.Payments[0].Method != domain.PaymentMultibanco || sale.Payments[0].Amount != "15.00" {
		t.Fatalf("line0 %+v", sale.Payments[0])
	}
	if sale.Payments[1].Method != domain.PaymentCash || sale.Payments[1].Amount != "5.00" {
		t.Fatalf("line1 %+v", sale.Payments[1])
	}
}
