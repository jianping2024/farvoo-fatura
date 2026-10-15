package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestIssueGT_Annul(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal-gt.db")
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
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.SQL.Exec(`INSERT INTO series (
		id, store_id, document_type, series_code, validation_code, fiscal_year,
		last_number, last_hash, status, registered_at, created_at, updated_at
	) VALUES ('series-gt-1', 'store-demo-001', 'GT', 'GT2026DEMO01', 'GTVAL001', 2026, 0, '', 'ACTIVE', ?, ?, ?)`,
		now, now, now)
	if err != nil {
		t.Fatal(err)
	}

	moveAt := "2026-10-15T14:00:00"
	gtParams := store.IssueGTParams{
		StoreID: "store-demo-001", RequestID: "gt-1", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		MovementPurpose: domain.MovementPurposeTransfer,
		ShipFromAddress: "From", ShipToAddress: "To", MovementStartAt: moveAt,
		RecipientIsVATPayer: true, ATDocCodeID: "MOCKABCD1234",
		NowUTC: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gt-sale-1", ScopeType: "manual", ScopeID: "gt-sale-1",
			FiscalPurpose: "transport_note",
			Customer: domain.CustomerInput{
				TaxID: "517535009", CompanyName: "Loja B",
				AddressDetail: "Rua B 1", City: "Porto", PostalCode: "4000-001", Country: "PT",
			},
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "100.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
		},
	}
	rec, err := db.IssueGT(context.Background(), sig, gtParams)
	if err != nil {
		t.Fatal(err)
	}
	if rec.DocumentType != domain.DocumentGT || rec.DocumentStatus != domain.WorkStatusNormal {
		t.Fatalf("got %+v", rec)
	}
	if !strings.Contains(rec.InvoiceNo, "GT") {
		t.Fatalf("invoice_no %s", rec.InvoiceNo)
	}

	again, err := db.IssueGT(context.Background(), sig, gtParams)
	if err != nil || !again.IdempotentHit {
		t.Fatalf("idempotent: %+v err=%v", again, err)
	}

	extra, err := db.LoadTransportExtra(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if extra.MovementPurpose != domain.MovementPurposeTransfer || extra.ATDocCodeID != "MOCKABCD1234" {
		t.Fatalf("extra %+v", extra)
	}

	ann, err := db.AnnulGT(context.Background(), store.AnnulGTParams{
		StoreID: "store-demo-001", RequestID: "annul-n", DocumentID: rec.DocumentID,
		OperatorID: "op-demo-cashier", Reason: "Cancelado",
	})
	if err != nil || ann.DocumentStatus != domain.WorkStatusAnnulled {
		t.Fatalf("annul %+v err=%v", ann, err)
	}

	gt2, err := db.IssueGT(context.Background(), sig, store.IssueGTParams{
		StoreID: "store-demo-001", RequestID: "gt-2", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		MovementPurpose: domain.MovementPurposeCustomer,
		ShipFromAddress: "From", ShipToAddress: "To", MovementStartAt: moveAt,
		RecipientIsVATPayer: true, ATDocCodeID: "MOCK2",
		NowUTC: time.Date(2026, 10, 15, 12, 30, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gt-sale-2", ScopeType: "manual", ScopeID: "gt-sale-2",
			FiscalPurpose: "transport_note",
			Customer: domain.CustomerInput{
				TaxID: "509442013", CompanyName: "Cliente GT",
				AddressDetail: "Rua C 1", City: "Lisboa", PostalCode: "1000-002", Country: "PT",
			},
			Lines: gtParams.Snapshot.Lines,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Force A then second annul must fail (status gate).
	if _, err := db.SQL.Exec(`UPDATE invoices SET document_status='A' WHERE id=?`, gt2.DocumentID); err != nil {
		t.Fatal(err)
	}
	_, err = db.AnnulGT(context.Background(), store.AnnulGTParams{
		StoreID: "store-demo-001", RequestID: "annul-a", DocumentID: gt2.DocumentID,
		OperatorID: "op-demo-cashier", Reason: "Again",
	})
	if err != store.ErrTransportNotAllowed {
		t.Fatalf("expected ErrTransportNotAllowed, got %v", err)
	}
}
