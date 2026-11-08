package store_test

import (
	"context"
	"errors"
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
		SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
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
		ShipFrom: testShipFrom, ShipTo: testShipTo, MovementStartAt: moveAt,
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
		OperatorID:       "op-demo-cashier",
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
		ShipFrom: testShipFrom, ShipTo: testShipTo, MovementStartAt: moveAt,
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
	// Annulled by another operator: SAF-T SourceID keeps the creator, DocumentStatus/SourceID
	// names who annulled (Portaria 302/2016 4.2.3.x vs 4.2.3.3.x).
	if err := db.UpsertOperator("op-annul", "store-demo-001", "admin", "Annuller", ""); err != nil {
		t.Fatal(err)
	}
	ann, err := db.AnnulGR(context.Background(), store.AnnulGRParams{
		StoreID: "store-demo-001", RequestID: "annul-n", DocumentID: gr2.DocumentID,
		OperatorID: "op-annul", Reason: "Cancelado",
	})
	if err != nil || ann.DocumentStatus != domain.WorkStatusAnnulled {
		t.Fatalf("annul %+v err=%v", ann, err)
	}
	var creator, statusBy string
	if err := db.SQL.QueryRow(`SELECT source_id, status_source_id FROM invoices WHERE id=?`, gr2.DocumentID).Scan(&creator, &statusBy); err != nil {
		t.Fatal(err)
	}
	if creator != "op-demo-cashier" || statusBy != "op-annul" {
		t.Fatalf("source_id=%s status_source_id=%s", creator, statusBy)
	}
	moves, err := db.LoadSAFTStockMovementsForPeriod("store-demo-001", "2000-01-01", "2100-12-31")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range moves {
		if m.ID == gr2.DocumentID && (m.SourceID == m.StatusSourceID || m.SourceID == "" || m.StatusSourceID == "") {
			t.Fatalf("SAF-T SourceID %q / StatusSourceID %q must differ", m.SourceID, m.StatusSourceID)
		}
	}

	// FT → GR picker: linked (F) and normal (N) are offered, annulled (A) is not.
	gr3, err := db.IssueGR(context.Background(), sig, store.IssueGRParams{
		StoreID: "store-demo-001", RequestID: "gr-3", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		ShipFrom: testShipFrom, ShipTo: testShipTo, MovementStartAt: moveAt,
		RecipientIsVATPayer: true, ATDocCodeID: "MOCK3",
		NowUTC: time.Date(2026, 10, 15, 12, 45, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "gr-sale-3", ScopeType: "manual", ScopeID: "gr-sale-3",
			FiscalPurpose: "delivery_note",
			Customer:      grParams.Snapshot.Customer,
			Lines:         grParams.Snapshot.Lines,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	linkable, err := db.ListLinkableDeliveryNotes("store-demo-001", "509442013")
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{"", "123456789"} {
		if got, err := db.ListLinkableDeliveryNotes("store-demo-001", other); err != nil || len(got) != 0 {
			t.Fatalf("buyer %q must see no GR of 509442013: %+v %v", other, got, err)
		}
	}
	got := map[string]string{}
	for _, it := range linkable {
		got[it.DocumentID] = it.DocumentStatus
	}
	if len(got) != 2 || got[rec.DocumentID] != "F" || got[gr3.DocumentID] != "N" {
		t.Fatalf("linkable GR %+v (want F %s + N %s, not annulled %s)", got, rec.DocumentID, gr3.DocumentID, gr2.DocumentID)
	}

	// "Issue FT from GR" source: original lines in manual-issue shape + buyer snapshot.
	items, err := db.LoadInvoiceItemLines(gr3.DocumentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != (store.InvoiceItemLine{DisplayName: "Item", SaftName: "Item",
		UnitPriceGross: "100.00", VATRatePercent: "23.00", Quantity: "1"}) {
		t.Fatalf("item lines %+v", items)
	}
	cust, err := db.LoadInvoiceCustomer(gr3.DocumentID)
	if err != nil || cust == nil || cust.TaxID != "509442013" || cust.CompanyName != "Cliente GR" || cust.PostalCode != "1000-001" {
		t.Fatalf("customer %+v err=%v", cust, err)
	}

	_, err = db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-other-buyer", DocType: domain.DocumentFT,
		OperatorID:       "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		DeliveryNoteIDs: []string{gr3.DocumentID},
		NowUTC:          time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "ft-other", ScopeType: "manual", ScopeID: "ft-other", FiscalPurpose: "sale",
			Lines: grParams.Snapshot.Lines,
			Customer: domain.CustomerInput{
				TaxID: "502757191", CompanyName: "Outro", Country: "PT",
			},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "100.00"}},
		},
	})
	if !errors.Is(err, store.ErrDeliveryNotAllowed) {
		t.Fatalf("other buyer must be refused, got %v", err)
	}
	var gr3Status string
	if err := db.SQL.QueryRow(`SELECT document_status FROM invoices WHERE id = ?`, gr3.DocumentID).Scan(&gr3Status); err != nil || gr3Status != "N" {
		t.Fatalf("GR status after refused link: %s %v", gr3Status, err)
	}

	nv := grParams
	nv.RequestID = "gr-nv"
	nv.RecipientIsVATPayer = false
	nv.Snapshot.SourceSaleID = "gr-nv"
	nv.Snapshot.ScopeID = "gr-nv"
	nv.Snapshot.Customer = domain.CustomerInput{
		CompanyName: "Maria Costa", AddressDetail: "Rua A 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT",
	}
	grNV, err := db.IssueGR(context.Background(), sig, nv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-nv-ok", DocType: domain.DocumentFT,
		OperatorID:       "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		DeliveryNoteIDs: []string{grNV.DocumentID},
		NowUTC:          time.Date(2026, 10, 15, 15, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "ft-nv", ScopeType: "manual", ScopeID: "ft-nv", FiscalPurpose: "sale",
			Lines:    nv.Snapshot.Lines,
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Maria Costa", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "100.00"}},
		},
	}); err != nil {
		t.Fatalf("same unnamed recipient: %v", err)
	}
	_, err = db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-nv-bad", DocType: domain.DocumentFT,
		OperatorID:       "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		DeliveryNoteIDs: []string{grNV.DocumentID},
		NowUTC:          time.Date(2026, 10, 15, 16, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: "ft-nv-bad", ScopeType: "manual", ScopeID: "ft-nv-bad", FiscalPurpose: "sale",
			Lines:    nv.Snapshot.Lines,
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Joao Silva", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "100.00"}},
		},
	})
	if !errors.Is(err, store.ErrDeliveryNotAllowed) {
		t.Fatalf("different unnamed recipient must be refused, got %v", err)
	}
}
