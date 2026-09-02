package service

import (
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

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

func TestUpsertOperatorWithActorCannotChangeOwnRole(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const storeID = "store-demo-001"
	const adminID = "op-admin-self"
	if err := db.UpsertOperator(adminID, storeID, "admin", "Admin", "local-"+adminID); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil, nil, dir, storeID)
	if err := svc.UpsertOperatorWithActor("admin", adminID, adminID, storeID, "owner", "Admin"); err == nil {
		t.Fatal("expected forbidden when changing own role")
	} else if !strings.Contains(err.Error(), "cannot change own role") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.UpsertOperatorWithActor("admin", adminID, adminID, storeID, "admin", "Admin Renamed"); err != nil {
		t.Fatalf("self rename: %v", err)
	}
}

func TestSetOperatorPINWithActor(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const storeID = "store-demo-001"
	if err := db.UpsertOperator("op-admin", storeID, "admin", "Admin", "local-op-admin"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertOperator("op-cash", storeID, "cashier", "Cash", "local-op-cash"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetOperatorPIN(storeID, "op-cash", "123456"); err != nil {
		t.Fatal(err)
	}
	svc := New(db, nil, nil, dir, storeID)
	if err := svc.SetOperatorPINWithActor("owner", "op-owner", storeID, "op-admin", "654321"); err == nil {
		t.Fatal("owner cannot reset admin pin")
	}
	if err := svc.SetOperatorPINWithActor("owner", "op-owner", storeID, "op-cash", "654321"); err != nil {
		t.Fatalf("owner reset cashier pin: %v", err)
	}
	if err := svc.SetOperatorPINWithActor("admin", "op-admin", storeID, "op-cash", "111111"); err != nil {
		t.Fatalf("admin reset cashier pin: %v", err)
	}
}
