package store

import (
	"path/filepath"
	"testing"
)

// Migration 018: every operator gets a per-store short SAF-T user code; new rows via trigger.
func TestOperatorSAFTUserCode(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"op-a", "op-b"} {
		if err := db.UpsertOperator(id, "store-1", "cashier", id, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.UpsertOperator("op-x", "store-2", "cashier", "x", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertOperator("op-a", "store-1", "cashier", "renamed", ""); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"op-a": "U001", "op-b": "U002", "op-x": "U001"}
	for id, code := range want {
		var got string
		if err := db.SQL.QueryRow(`SELECT saft_user_code FROM operators WHERE id=?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != code {
			t.Fatalf("%s: saft_user_code %q want %q", id, got, code)
		}
	}
}
