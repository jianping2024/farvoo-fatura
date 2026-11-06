package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

var (
	testShipFrom = domain.Address{Detail: "Rua Armazem 1", City: "Lisboa", PostalCode: "1000-001"}
	testShipTo   = domain.Address{Detail: "Rua Entrega 10", City: "Porto", PostalCode: "4000-123"}
)

// GR load/unload points round-trip as structured columns; pre-017 rows (free text only)
// read back with the text as Detail.
func TestShipAddress_RoundTripAndLegacy(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	sig, err := signer.LoadPEMFile(keyPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "GR", "GR2026DEMO01", "GRVAL001", 2026); err != nil {
		t.Fatal(err)
	}
	rec, err := db.IssueGR(context.Background(), sig, store.IssueGRParams{
		StoreID: "store-demo-001", RequestID: "gr-addr-1", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ShipFrom: domain.Address{Detail: " Rua  Armazem 1 ", City: "Lisboa", PostalCode: "1000-001"},
		ShipTo:   testShipTo, MovementStartAt: "2026-10-15T14:00:00",
		RecipientIsVATPayer: true, ATDocCodeID: "MOCKABCD1234",
		NowUTC: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gr-addr-1", ScopeType: "manual", ScopeID: "gr-addr-1",
			FiscalPurpose: "delivery_note",
			Customer: domain.CustomerInput{TaxID: "509442013", CompanyName: "Cliente GR",
				AddressDetail: "Rua A 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT"},
			Lines: []domain.SaleLine{{ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "100.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	extra, err := db.LoadDeliveryExtra(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	want := testShipFrom.Normalize()
	if extra.ShipFrom != want || extra.ShipTo != testShipTo.Normalize() {
		t.Fatalf("round trip: from %+v to %+v", extra.ShipFrom, extra.ShipTo)
	}

	if _, err := db.SQL.Exec(`UPDATE invoices SET ship_from_detail=NULL, ship_from_city=NULL, ship_from_postal_code=NULL,
		ship_from_country=NULL, ship_from_address='Armazem Rua Demo 1, Lisboa' WHERE id=?`, rec.DocumentID); err != nil {
		t.Fatal(err)
	}
	extra, err = db.LoadDeliveryExtra(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if extra.ShipFrom != (domain.Address{Detail: "Armazem Rua Demo 1, Lisboa", Country: "PT"}) {
		t.Fatalf("legacy row: %+v", extra.ShipFrom)
	}
}
