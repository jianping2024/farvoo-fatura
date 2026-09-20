package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestIssuePersistsTenderedChange(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sig, err := signer.LoadPEMFile(keyPath, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := sig.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}

	pay := domain.PaymentInput{Method: "CASH", Amount: "12.50"}
	if err := domain.ApplyCashTender(&pay, "20"); err != nil {
		t.Fatal(err)
	}
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "req-tender-1", DocType: domain.DocumentFS,
		OperatorID: "op-demo-cashier",
		NowUTC:     time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "sale-tender-1",
			ScopeType: "manual", ScopeID: "sale-tender-1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item",
				Quantity: "1", UnitPriceGross: "12.50", VATRate: "0.23",
				ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{pay},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var amount, tendered, changeDue string
	err = db.SQL.QueryRow(`SELECT amount, IFNULL(tendered,''), IFNULL(change_due,'')
		FROM invoice_payments WHERE invoice_id = ? ORDER BY rowid LIMIT 1`, rec.DocumentID).
		Scan(&amount, &tendered, &changeDue)
	if err != nil {
		t.Fatal(err)
	}
	if amount != "12.50" || tendered != "20.00" || changeDue != "7.50" {
		t.Fatalf("db amount=%q tendered=%q change=%q", amount, tendered, changeDue)
	}

	var payloadJSON string
	err = db.SQL.QueryRow(`SELECT payload_json FROM local_print_jobs WHERE invoice_id = ?`, rec.DocumentID).Scan(&payloadJSON)
	if err != nil {
		t.Fatal(err)
	}
	var payload print.Payload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Payments) != 1 || payload.Payments[0].Tendered != "20.00" || payload.Payments[0].ChangeDue != "7.50" {
		t.Fatalf("payload payments=%+v", payload.Payments)
	}
}
