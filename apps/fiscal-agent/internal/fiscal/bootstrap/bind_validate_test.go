package bootstrap

import "testing"

func TestValidateBindAddr_LoopbackOK(t *testing.T) {
	if err := validateBindAddr("127.0.0.1:17880", false); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBindAddr_LANRequiresAllow(t *testing.T) {
	if err := validateBindAddr("0.0.0.0:17880", false); err == nil {
		t.Fatal("expected deny without AllowLAN")
	}
	if err := validateBindAddr("0.0.0.0:17880", true); err != nil {
		t.Fatal(err)
	}
	if err := validateBindAddr("10.0.0.5:17880", false); err == nil {
		t.Fatal("expected deny non-loopback without AllowLAN")
	}
	if err := validateBindAddr("10.0.0.5:17880", true); err != nil {
		t.Fatal(err)
	}
}
