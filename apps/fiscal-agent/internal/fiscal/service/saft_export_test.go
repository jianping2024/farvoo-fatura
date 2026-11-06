package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// Each export is its own immutable file; a period is any start..end inside one fiscal year.
func TestExportSAFTArchivesEachExportAndTakesDateRange(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
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
	ft, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-saft-export", DocType: domain.DocumentFT,
		OperatorID:       "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		NowUTC: time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-saft-export", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
				UnitPriceGross: "12.50", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "12.50"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, sig, nil, filepath.Join(dir, "secure"), "store-demo-001")

	in := service.ExportSAFTInput{StartDate: "2026-08-15", EndDate: "2026-08-25", OperatorID: "op-demo-cashier"}
	first, err := svc.ExportSAFT(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ExportSAFT(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.FilePath == second.FilePath {
		t.Fatalf("re-export overwrote %s", first.FilePath)
	}
	for _, res := range []*service.ExportSAFTResult{first, second} {
		data, err := os.ReadFile(res.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		row, err := svc.GetSAFTExport(res.ExportID)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(sum[:]) != row.FileSHA256 || row.StartDate != "2026-08-15" || row.EndDate != "2026-08-25" {
			t.Fatalf("archive row %+v does not match its file", row)
		}
		if !strings.Contains(string(data), "<StartDate>2026-08-15</StartDate><EndDate>2026-08-25</EndDate>") {
			t.Fatal("Header period must be the requested range")
		}
	}

	for _, bad := range []service.ExportSAFTInput{
		{StartDate: "2026-12-20", EndDate: "2027-01-05"}, // two fiscal years
		{StartDate: "2026-08-25", EndDate: "2026-08-15"}, // reversed
		{StartDate: "2026-08", EndDate: "2026-08-31"},
	} {
		if _, err := svc.ExportSAFT(context.Background(), bad); err == nil {
			t.Fatalf("period %s..%s must be rejected", bad.StartDate, bad.EndDate)
		}
	}

	// SAF-T Reason ≤50 characters: a correction reason one over the limit is refused at entry.
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueCreditNote(context.Background(), domain.CreditNoteRequest{
		RequestID: "nc-long-reason", OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		OriginalInvoiceID: ft.DocumentID, Reason: strings.Repeat("a", 51), CreditFull: true,
	})
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeValidationFailed {
		t.Fatalf("51-char NC reason: want validation_failed, got %v", err)
	}
}
