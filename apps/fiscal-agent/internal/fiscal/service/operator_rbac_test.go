package service

import "testing"

func TestValidateOperatorRoleWrite(t *testing.T) {
	if err := validateOperatorRoleWrite("admin", "owner"); err != nil {
		t.Fatalf("admin creates owner: %v", err)
	}
	if err := validateOperatorRoleWrite("admin", "cashier"); err != nil {
		t.Fatalf("admin creates cashier: %v", err)
	}
	if err := validateOperatorRoleWrite("admin", "admin"); err == nil {
		t.Fatal("admin cannot create admin")
	}
	if err := validateOperatorRoleWrite("owner", "cashier"); err != nil {
		t.Fatalf("owner creates cashier: %v", err)
	}
	if err := validateOperatorRoleWrite("owner", "owner"); err == nil {
		t.Fatal("owner cannot create owner")
	}
	if err := validateOperatorRoleWrite("cashier", "cashier"); err == nil {
		t.Fatal("cashier forbidden")
	}
}
