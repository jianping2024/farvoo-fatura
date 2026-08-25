package ptnif

import "testing"

func TestValidMod11(t *testing.T) {
	if !Valid("123456789") {
		t.Fatal("123456789 should pass Mod-11")
	}
	if !Valid("517535009") {
		t.Fatal("517535009 should pass Mod-11")
	}
	if !Valid(FinalConsumer) {
		t.Fatal("final consumer must pass")
	}
	if Valid("111111111") {
		t.Fatal("111111111 must fail Mod-11")
	}
	if Valid("023456789") {
		t.Fatal("leading zero must fail")
	}
}

func TestNormalizeBuyer(t *testing.T) {
	got, err := NormalizeBuyer("123 456 789")
	if err != nil || got != "123456789" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeBuyer("111111111"); err == nil {
		t.Fatal("expected error")
	}
}
