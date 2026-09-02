package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLoginIPRateLimit(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	storeID := "store-ip-001"
	opID := "op-1"
	ip := "192.168.1.100"

	for i := 0; i < loginFailureIPMax; i++ {
		if err := db.RecordLoginFailures(storeID, opID, ip); err != nil {
			t.Fatal(err)
		}
	}
	limited, err := db.IsLoginIPRateLimited(ip)
	if err != nil || !limited {
		t.Fatalf("expected IP limited after %d failures, limited=%v err=%v", loginFailureIPMax, limited, err)
	}

	if _, err := db.SQL.Exec(`DELETE FROM audit_log WHERE entity_type=? AND entity_id=?`,
		loginFailureEntityIP, ip); err != nil {
		t.Fatal(err)
	}
	limited, err = db.IsLoginIPRateLimited(ip)
	if err != nil || limited {
		t.Fatalf("expected IP not limited after cleanup, limited=%v err=%v", limited, err)
	}
}

func TestVerifyOperatorPIN_IPRateLimited(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	storeID := "store-ip-002"
	if _, err := db.BootstrapOwner(storeID, "Admin", "123456"); err != nil {
		t.Fatal(err)
	}
	ops, err := db.ListOperatorsForLogin(storeID)
	if err != nil || len(ops) != 1 {
		t.Fatal(ops, err)
	}
	opID := ops[0].ID
	ip := "10.0.0.50"

	for i := 0; i < loginFailureIPMax; i++ {
		if err := db.RecordLoginFailures(storeID, opID, ip); err != nil {
			t.Fatal(err)
		}
	}
	err = db.VerifyOperatorPIN(storeID, opID, "000000", ip)
	if !errors.Is(err, ErrIPRateLimited) {
		t.Fatalf("want ErrIPRateLimited, got %v", err)
	}
}

func TestClientIPFromRemoteAddr(t *testing.T) {
	if got := ClientIPFromRemoteAddr("192.168.1.5:17880"); got != "192.168.1.5" {
		t.Fatalf("got %q", got)
	}
	if got := ClientIPFromRemoteAddr("[::1]:1234"); got != "::1" {
		t.Fatalf("got %q", got)
	}
}
