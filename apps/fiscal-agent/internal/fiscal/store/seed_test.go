package store_test

import (
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestSeedDemoOperatorLoginPIN(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const storeID = "store-demo-001"
	err = db.SeedDemo(store.SeedDemoParams{
		StoreID: storeID, TaxpayerNIF: "123456789", LegalName: "Demo",
		Address: "Rua", City: "Lisboa", PostalCode: "1000-001",
		SeriesCode: "FT2026A", ValidationCode: "ABCD1234", FiscalYear: 2026,
		OperatorID: "op-demo-cashier", OperatorName: "Demo Cashier",
		PublicKeyPEM: "pub", WrappedPrivateKey: "wrap",
		InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "dpk",
	})
	if err != nil {
		t.Fatal(err)
	}
	login, err := db.ListOperatorsForLogin(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(login) != 1 {
		t.Fatalf("seed must expose 1 loginable operator, got %d", len(login))
	}
	if login[0].ID != "op-demo-cashier" || !login[0].HasPIN {
		t.Fatalf("unexpected login row: %+v", login[0])
	}
	if err := db.VerifyOperatorPIN(storeID, "op-demo-cashier", store.SeedDemoOperatorPIN, "127.0.0.1"); err != nil {
		t.Fatalf("SeedDemoOperatorPIN must verify: %v", err)
	}
}

func TestSeedDemoBackfillsPinlessOperator(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const storeID = "store-demo-001"
	if err := db.UpsertOperator("op-demo-cashier", storeID, "cashier", "Demo Cashier", "mesa-op-demo-cashier"); err != nil {
		t.Fatal(err)
	}
	login, err := db.ListOperatorsForLogin(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(login) != 0 {
		t.Fatalf("pinless operator must not appear in login list, got %d", len(login))
	}
	err = db.SeedDemo(store.SeedDemoParams{
		StoreID: storeID, TaxpayerNIF: "123456789", LegalName: "Demo",
		Address: "Rua", City: "Lisboa", PostalCode: "1000-001",
		SeriesCode: "FT2026A", ValidationCode: "ABCD1234", FiscalYear: 2026,
		OperatorID: "op-demo-cashier", OperatorName: "Demo Cashier",
		PublicKeyPEM: "pub", WrappedPrivateKey: "wrap",
		InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "dpk",
	})
	if err != nil {
		t.Fatal(err)
	}
	login, err = db.ListOperatorsForLogin(storeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(login) != 1 {
		t.Fatalf("re-seed must backfill PIN for login, got %d", len(login))
	}
}
