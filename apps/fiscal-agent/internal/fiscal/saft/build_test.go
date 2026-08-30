package saft_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/saft"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"golang.org/x/text/encoding/charmap"
)

func TestBuildContainsFTAndNC(t *testing.T) {
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
	ftID := seedSAFTDemo(t, db, sig)
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}

	nc, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-saft-1", OriginalInvoiceID: ftID,
		OperatorID: "op-demo-cashier", Reason: "Devolucao SAFT", CreditFull: true,
		NowUTC: time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	taxpayer, err := db.GetTaxpayerSettings("store-demo-001")
	if err != nil {
		t.Fatal(err)
	}
	invoices, err := db.LoadSAFTInvoicesForPeriod("store-demo-001", "2026-08-01", "2026-08-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(invoices) < 2 {
		t.Fatalf("invoices %d", len(invoices))
	}

	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, Year: 2026, Month: 8,
		StartDate: "2026-08-01", EndDate: "2026-08-31", Invoices: invoices,
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.ValidationStatus != "VALID" {
		t.Fatalf("validation %s errs=%v", built.ValidationStatus, built.ValidationErrors)
	}
	xml := string(built.XMLBytes)
	if !strings.Contains(xml, "FT FT2026DEMO01/1") {
		t.Fatal("missing FT invoice_no")
	}
	if !strings.Contains(xml, nc.InvoiceNo) {
		t.Fatalf("missing NC %s", nc.InvoiceNo)
	}
	if !strings.Contains(xml, "Devolucao SAFT") {
		t.Fatal("missing NC reason")
	}
	if !strings.Contains(xml, "<InvoiceType>NC</InvoiceType>") {
		t.Fatal("missing NC type")
	}
}

func TestBuildPortugueseAccentsValid(t *testing.T) {
	taxpayer := &store.TaxpayerSettings{
		TaxRegistrationNumber: "517535009", LegalName: "Farvoo Demo Lda",
		AddressDetail: "Rua Demo 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT",
		SoftwareCertificateNumber: "0", ProductID: "Farvoo/InvoiceEngine",
	}
	inv := store.SAFTInvoice{
		DocumentType: "FT", InvoiceNo: "FT FT2026DEMO01/1", ATCUD: "X-1",
		InvoiceDate: "2026-08-21", SystemEntryDate: "2026-08-21T12:00:00",
		Hash: "abc", HashControl: 1, GrossTotal: "1.85", NetTotal: "1.50", TaxPayable: "0.35",
		SourceID: "op-demo-cashier",
		Customer: store.SAFTCustomer{CustomerTaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
		Lines: []store.SAFTLine{{
			LineNumber: 1, ProductCode: "001", ProductDescription: "Água 500ml",
			Quantity: "1", UnitOfMeasure: "UN", LineGross: "1.85", VATRate: "0.23",
			TaxType: "IVA", TaxCountryRegion: "PT", TaxCode: "NOR", ProductType: "P",
		}},
	}
	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, Year: 2026, Month: 8,
		StartDate: "2026-08-01", EndDate: "2026-08-31", Invoices: []store.SAFTInvoice{inv},
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.ValidationStatus != "VALID" {
		t.Fatalf("validation %s errs=%v", built.ValidationStatus, built.ValidationErrors)
	}
	xml, err := charmap.Windows1252.NewDecoder().String(string(built.XMLBytes))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xml, "Água 500ml") {
		t.Fatal("missing accented product in XML")
	}
}

func TestBuildRejectsNon1252Characters(t *testing.T) {
	taxpayer := &store.TaxpayerSettings{
		TaxRegistrationNumber: "517535009", LegalName: "Farvoo Demo Lda",
		AddressDetail: "Rua Demo 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT",
		SoftwareCertificateNumber: "0", ProductID: "Farvoo/InvoiceEngine",
	}
	inv := store.SAFTInvoice{
		DocumentType: "FT", InvoiceNo: "FT FT2026DEMO01/1", ATCUD: "X-1",
		InvoiceDate: "2026-08-21", SystemEntryDate: "2026-08-21T12:00:00",
		Hash: "abc", HashControl: 1, GrossTotal: "12.50", NetTotal: "10.16", TaxPayable: "2.34",
		SourceID: "op-demo-cashier",
		Customer: store.SAFTCustomer{CustomerTaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
		Lines: []store.SAFTLine{{
			LineNumber: 1, ProductCode: "CN1", ProductDescription: "茉莉茶",
			Quantity: "1", UnitOfMeasure: "UN", LineGross: "12.50", VATRate: "0.23",
			TaxType: "IVA", TaxCountryRegion: "PT", TaxCode: "NOR", ProductType: "P",
		}},
	}
	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, Year: 2026, Month: 8,
		StartDate: "2026-08-01", EndDate: "2026-08-31", Invoices: []store.SAFTInvoice{inv},
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.ValidationStatus != "INVALID" {
		t.Fatalf("expected INVALID got %s", built.ValidationStatus)
	}
}

func seedSAFTDemo(t *testing.T, db *store.DB, sig *signer.PEMSigner) string {
	t.Helper()
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
	rec, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-saft-1", DocType: domain.DocumentFT,
		OperatorID: "op-demo-cashier", NowUTC: time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-saft-1", ScopeType: "session", ScopeID: "s1", FiscalPurpose: "sale",
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
	return rec.DocumentID
}
