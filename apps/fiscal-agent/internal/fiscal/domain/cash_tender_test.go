package domain

import "testing"

func TestApplyCashTender(t *testing.T) {
	pay := PaymentInput{Method: PaymentCash, Amount: "8.00"}
	if err := ApplyCashTender(&pay, "10"); err != nil {
		t.Fatal(err)
	}
	if pay.Tendered != "10.00" || pay.ChangeDue != "2.00" {
		t.Fatalf("got tendered=%q change=%q", pay.Tendered, pay.ChangeDue)
	}
	if err := ApplyCashTender(&pay, ""); err != nil {
		t.Fatal(err)
	}
	if pay.Tendered != "" || pay.ChangeDue != "" {
		t.Fatalf("clear failed: %+v", pay)
	}
	pay.Method = PaymentCard
	if err := ApplyCashTender(&pay, "10.00"); err == nil {
		t.Fatal("expected card reject")
	}
	pay.Method = PaymentCash
	if err := ApplyCashTender(&pay, "7.99"); err == nil {
		t.Fatal("expected underpay reject")
	}
	if err := ApplyCashTender(&pay, "8.00"); err != nil {
		t.Fatal(err)
	}
	if pay.ChangeDue != "0.00" {
		t.Fatalf("exact change: %q", pay.ChangeDue)
	}
}

func TestApplyCashTenderToSale(t *testing.T) {
	sale := &SaleSnapshot{Payments: []PaymentInput{{Method: PaymentCash, Amount: "12.50"}}}
	if err := ApplyCashTenderToSale(sale, "20"); err != nil {
		t.Fatal(err)
	}
	if sale.Payments[0].Tendered != "20.00" || sale.Payments[0].ChangeDue != "7.50" {
		t.Fatalf("%+v", sale.Payments[0])
	}
	if err := ApplyCashTenderToSale(nil, "1"); err == nil {
		t.Fatal("nil sale")
	}
}
