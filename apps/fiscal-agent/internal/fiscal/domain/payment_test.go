package domain

import "testing"

func TestKnownPaymentMethodsStable(t *testing.T) {
	want := []string{
		PaymentCash, PaymentCard, PaymentMBWay, PaymentMultibanco, PaymentMixed, PaymentOther,
	}
	got := KnownPaymentMethods()
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
	got[0] = "MUTATED"
	if KnownPaymentMethods()[0] != PaymentCash {
		t.Fatal("KnownPaymentMethods must return a copy")
	}
}

func TestIsKnownPaymentMethod(t *testing.T) {
	for _, code := range KnownPaymentMethods() {
		if !IsKnownPaymentMethod(code) {
			t.Fatalf("%s should be known", code)
		}
		if !IsKnownPaymentMethod(stringsLower(code)) {
			t.Fatalf("lowercase %s should be known", code)
		}
	}
	if IsKnownPaymentMethod("BITCOIN") {
		t.Fatal("unknown method")
	}
	if NormalizePaymentMethod("") != PaymentCash {
		t.Fatal("empty normalizes to CASH")
	}
}

func stringsLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
