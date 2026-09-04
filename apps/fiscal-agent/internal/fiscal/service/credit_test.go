package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestIssueCreditNoteOperatorDenied(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	pub, _ := sig.PublicKeyPEM()
	if err := db.SeedDemoFromKeyFile(store.SeedDemoParams{
		StoreID: "store-demo-001", TaxpayerNIF: "517535009", LegalName: "Demo Lda",
		Address: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0", SeriesCode: "FT2026DEMO01", ValidationCode: "CSDF7T5H",
		FiscalYear: 2026, OperatorID: "op-demo-cashier", OperatorName: "Cashier",
		SigningKeyVersion: 1, InstallationID: "inst-1", DeviceID: "dev-1", DevicePublicKey: "x",
	}, keyPath, pub); err != nil {
		t.Fatal(err)
	}
	// SeedDemo is always admin; demote for cashier NC-denied case.
	if err := db.UpsertOperator("op-demo-cashier", "store-demo-001", "cashier", "Cashier", "mesa-op-demo-cashier"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetOperatorCanIssueNC("store-demo-001", "op-demo-cashier", false); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-req-perm", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier",
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-perm", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
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

	svc := service.New(db, sig, nil, dir, "store-demo-001")
	_, err = svc.IssueCreditNote(context.Background(), domain.CreditNoteRequest{
		RequestID: "nc-perm-1", OperatorID: "op-demo-cashier", OriginalInvoiceID: rec.DocumentID,
		Reason: "Denied", CreditFull: true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeCreditNotAllowed {
		t.Fatalf("want credit_not_allowed, got %v", err)
	}
	if ce.Msg != "operator cannot issue credit notes" {
		t.Fatalf("msg %q", ce.Msg)
	}
}
