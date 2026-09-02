package store_test

import (
	"errors"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestBootstrapOwner_EmptyThenRejectSecond(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const storeID = "store-bootstrap-1"
	if err := db.UpsertOperator("op-legacy", storeID, "cashier", "Legacy", "mesa-legacy"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.BootstrapOwner(storeID, "Owner", "123456"); err == nil {
		t.Fatal("expected bootstrap_not_empty when operators exist")
	} else if !errors.Is(err, store.ErrBootstrapNotEmpty) {
		t.Fatalf("got %v", err)
	}

	if _, err := db.SQL.Exec(`DELETE FROM operators WHERE store_id=?`, storeID); err != nil {
		t.Fatal(err)
	}
	id, err := db.BootstrapOwner(storeID, "Owner One", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected operator id")
	}
	var role string
	if err := db.SQL.QueryRow(`SELECT role FROM operators WHERE id=?`, id).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Fatalf("bootstrap role=%q want admin", role)
	}
	n, err := db.CountActiveOperatorsWithPIN(storeID)
	if err != nil || n != 1 {
		t.Fatalf("CountActiveOperatorsWithPIN=%d err=%v", n, err)
	}
	if _, err := db.BootstrapOwner(storeID, "Owner Two", "654321"); !errors.Is(err, store.ErrBootstrapNotEmpty) {
		t.Fatalf("second bootstrap: %v", err)
	}
}

func TestListOperatorsRoleOrder(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const storeID = "store-sort-1"
	for _, spec := range []struct {
		id, role, name string
	}{
		{"op-admin", "admin", "Z Admin"},
		{"op-owner", "owner", "M Owner"},
		{"op-cash", "cashier", "A Cashier"},
	} {
		if err := db.UpsertOperator(spec.id, storeID, spec.role, spec.name, "local-"+spec.id); err != nil {
			t.Fatal(err)
		}
		if err := db.SetOperatorPIN(storeID, spec.id, "123456"); err != nil {
			t.Fatal(err)
		}
	}
	login, err := db.ListOperatorsForLogin(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(login) != 3 {
		t.Fatalf("login rows=%d", len(login))
	}
	wantLogin := []string{"cashier", "owner", "admin"}
	for i, row := range login {
		if row.Role != wantLogin[i] {
			t.Fatalf("login[%d] role=%q want %q", i, row.Role, wantLogin[i])
		}
	}
	manage, err := db.ListOperatorsForManage(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(manage) != 3 {
		t.Fatalf("manage rows=%d", len(manage))
	}
	wantManage := []string{"cashier", "owner", "admin"}
	for i, row := range manage {
		if row.Role != wantManage[i] {
			t.Fatalf("manage[%d] role=%q want %q", i, row.Role, wantManage[i])
		}
	}
}

func TestCountActiveOperatorsWithPIN_IgnoresUnpinned(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const storeID = "store-pin-gate"
	if err := db.UpsertOperator("op-nopin", storeID, "owner", "No PIN", "mesa-nopin"); err != nil {
		t.Fatal(err)
	}
	n, err := db.CountActiveOperatorsWithPIN(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("unpinned operator must not count, got %d", n)
	}
}
