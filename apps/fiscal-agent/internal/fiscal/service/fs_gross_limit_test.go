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

func TestIssueDocument_FSOver100BecomesFT(t *testing.T) {
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
	svc := service.New(db, sig, nil, dir, "store-demo-001")

	atLimit, err := svc.IssueDocument(context.Background(), issueReq("req-fs-100", "100.00"), domain.DocumentFS)
	if err != nil {
		t.Fatal(err)
	}
	if atLimit.DocumentType != domain.DocumentFS || atLimit.InvoiceNo != "FS FS2026DEMO01/1" {
		t.Fatalf("at limit: type=%s no=%s", atLimit.DocumentType, atLimit.InvoiceNo)
	}

	over, err := svc.IssueDocument(context.Background(), issueReq("req-fs-100-01", "60.00", "40.01"), domain.DocumentFS)
	if err != nil {
		t.Fatal(err)
	}
	if over.DocumentType != domain.DocumentFT || over.InvoiceNo != "FT FT2026DEMO01/1" {
		t.Fatalf("over limit: type=%s no=%s", over.DocumentType, over.InvoiceNo)
	}

	again, err := svc.IssueDocument(context.Background(), issueReq("req-fs-100-01", "60.00", "40.01"), domain.DocumentFS)
	if err != nil {
		t.Fatal(err)
	}
	if !again.IdempotentHit || again.DocumentID != over.DocumentID || again.DocumentType != domain.DocumentFT {
		t.Fatalf("replay: hit=%v id=%s type=%s", again.IdempotentHit, again.DocumentID, again.DocumentType)
	}

	if _, err := db.SQL.Exec(`UPDATE series SET status='CLOSED' WHERE document_type='FT'`); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueDocument(context.Background(), issueReq("req-fs-no-ft", "100.01"), domain.DocumentFS)
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != service.ErrCodeSeriesMissing {
		t.Fatalf("missing FT series: %v", err)
	}
}

func issueReq(requestID string, grosses ...string) domain.IssueRequest {
	lines := make([]domain.SaleLine, 0, len(grosses))
	for _, g := range grosses {
		lines = append(lines, domain.SaleLine{
			ProductCode: "P1", DisplayName: "Prato", SaftName: "Prato", Quantity: "1",
			UnitPriceGross: g, VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
		})
	}
	return domain.IssueRequest{
		RequestID: requestID, OperatorID: "op-demo-cashier", StationID: "st",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: requestID, ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
			Lines:    lines,
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "1.00"}},
		},
	}
}
