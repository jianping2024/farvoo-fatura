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

func TestIssueGR_Annul_LinkFT(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal-gr.db")
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
	) VALUES ('series-gr-1', 'store-demo-001', 'GR', 'GR2026DEMO01', 'GRVAL001', 2026, 0, '', 'ACTIVE', ?, ?, ?)`,
		now, now, now)
	if err != nil {
		t.Fatal(err)
	}

	moveAt := "2026-10-15T14:00:00"
	grParams := store.IssueGRParams{
		StoreID: "store-demo-001", RequestID: "gr-1", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ShipFromAddress: "From", ShipToAddress: "To", MovementStartAt: moveAt,
		RecipientIsVATPayer: true, ATDocCodeID: "MOCKABCD1234",
		NowUTC: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gr-sale-1", ScopeType: "manual", ScopeID: "gr-sale-1",
			FiscalPurpose: "delivery_note",
			Customer: domain.CustomerInput{
				TaxID: "509442013", CompanyName: "Cliente GR",
				AddressDetail: "Rua A 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT",
			},
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "100.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
		},
	}
	rec, err := db.IssueGR(context.Background(), sig, grParams)
	if err != nil {
		t.Fatal(err)
	}
	if rec.DocumentType != domain.DocumentGR || rec.DocumentStatus != domain.WorkStatusNormal {
		t.Fatalf("got %+v", rec)
	}
	if !strings.Contains(rec.InvoiceNo, "GR") {
		t.Fatalf("invoice_no %s", rec.InvoiceNo)
	}

	again, err := db.IssueGR(context.Background(), sig, grParams)
	if err != nil || !again.IdempotentHit {
		t.Fatalf("idempotent: %+v err=%v", again, err)
	}

	ft, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-link", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		DeliveryNoteIDs: []string{rec.DocumentID},
		NowUTC:          time.Date(2026, 10, 15, 13, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "ft-1", ScopeType: "manual", ScopeID: "ft-1", FiscalPurpose: "sale",
			Lines:    grParams.Snapshot.Lines,
			Customer: grParams.Snapshot.Customer,
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "100.00"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	extra, err := db.LoadDeliveryExtra(rec.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if !extra.RecipientIsVATPayer || extra.ATDocCodeID != "MOCKABCD1234" {
		t.Fatalf("extra %+v", extra)
	}
	if len(extra.LinkedSaleNumbers) != 1 || extra.LinkedSaleNumbers[0] != ft.InvoiceNo {
		t.Fatalf("linked %+v want %s", extra.LinkedSaleNumbers, ft.InvoiceNo)
	}
	var st string
	if err := db.SQL.QueryRow(`SELECT document_status FROM invoices WHERE id=?`, rec.DocumentID).Scan(&st); err != nil {
		t.Fatal(err)
	}
	if st != string(domain.WorkStatusInvoiced) {
		t.Fatalf("status %s", st)
	}

	_, err = db.AnnulGR(context.Background(), store.AnnulGRParams{
		StoreID: "store-demo-001", RequestID: "annul-f", DocumentID: rec.DocumentID,
		OperatorID: "op-demo-cashier", Reason: "Nope",
	})
	if err != store.ErrDeliveryNotAllowed {
		t.Fatalf("expected ErrDeliveryNotAllowed, got %v", err)
	}

	gr2, err := db.IssueGR(context.Background(), sig, store.IssueGRParams{
		StoreID: "store-demo-001", RequestID: "gr-2", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ShipFromAddress: "From", ShipToAddress: "To", MovementStartAt: moveAt,
		RecipientIsVATPayer: true, ATDocCodeID: "MOCK2",
		NowUTC: time.Date(2026, 10, 15, 12, 30, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gr-sale-2", ScopeType: "manual", ScopeID: "gr-sale-2",
			FiscalPurpose: "delivery_note",
			Customer:      grParams.Snapshot.Customer,
			Lines:         grParams.Snapshot.Lines,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ann, err := db.AnnulGR(context.Background(), store.AnnulGRParams{
		StoreID: "store-demo-001", RequestID: "annul-n", DocumentID: gr2.DocumentID,
		OperatorID: "op-demo-cashier", Reason: "Cancelado",
	})
	if err != nil || ann.DocumentStatus != domain.WorkStatusAnnulled {
		t.Fatalf("annul %+v err=%v", ann, err)
	}
}
