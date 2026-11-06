package saft_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/saft"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/software"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/charmap"
)

// xsdFixtureEnv: scripts/fiscal-saft-xsd-check.sh sets it to a path; the fixture XML is written
// there and validated against the official AT XSD 1.1 (saft/testdata/SAFTPT1.04_01.xsd).
const xsdFixtureEnv = "SAFT_XSD_FIXTURE_OUT"

var uuidSourceID = regexp.MustCompile(`<SourceID>[0-9a-f]{8}-[0-9a-f]{4}-`)

type auditFile struct {
	Header struct {
		TaxAccountingBasis  string `xml:"TaxAccountingBasis"`
		FiscalYear          string `xml:"FiscalYear"`
		DateCreated         string `xml:"DateCreated"`
		ProductCompanyTaxID string `xml:"ProductCompanyTaxID"`
		CertificateNumber   string `xml:"SoftwareCertificateNumber"`
		ProductID           string `xml:"ProductID"`
		ProductVersion      string `xml:"ProductVersion"`
	} `xml:"Header"`
	MovementOfGoods struct {
		Lines    int    `xml:"NumberOfMovementLines"`
		Quantity string `xml:"TotalQuantityIssued"`
		Docs     []struct {
			Status string     `xml:"DocumentStatus>MovementStatus"`
			Lines  []saftLine `xml:"Line"`
			Totals docTotals  `xml:"DocumentTotals"`
		} `xml:"StockMovement"`
	} `xml:"SourceDocuments>MovementOfGoods"`
	SalesInvoices struct {
		TotalDebit  string `xml:"TotalDebit"`
		TotalCredit string `xml:"TotalCredit"`
		Invoices    []struct {
			InvoiceNo   string     `xml:"InvoiceNo"`
			Period      string     `xml:"Period"`
			InvoiceType string     `xml:"InvoiceType"`
			Lines       []saftLine `xml:"Line"`
			Totals      docTotals  `xml:"DocumentTotals"`
		} `xml:"Invoice"`
	} `xml:"SourceDocuments>SalesInvoices"`
	WorkingDocuments struct {
		TotalCredit string `xml:"TotalCredit"`
		Docs        []struct {
			Status string     `xml:"DocumentStatus>WorkStatus"`
			Lines  []saftLine `xml:"Line"`
			Totals docTotals  `xml:"DocumentTotals"`
		} `xml:"WorkDocument"`
	} `xml:"SourceDocuments>WorkingDocuments"`
}

type saftLine struct {
	Quantity         string  `xml:"Quantity"`
	UnitPrice        string  `xml:"UnitPrice"`
	DebitAmount      *string `xml:"DebitAmount"`
	CreditAmount     *string `xml:"CreditAmount"`
	SettlementAmount *string `xml:"SettlementAmount"`
}

type docTotals struct {
	TaxPayable string `xml:"TaxPayable"`
	NetTotal   string `xml:"NetTotal"`
	GrossTotal string `xml:"GrossTotal"`
}

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := compliance.ParseDecimal(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Portaria 302/2016: line amounts are net of VAT, NC lines on the debit side, everything else on
// the credit side; control totals sum the lines; Net+Tax=Gross per document.
func TestBuildLineAmountsNetAndSided(t *testing.T) {
	built := buildFixture(t)
	if built.ValidationStatus != "VALID" {
		t.Fatalf("validation %s errs=%v", built.ValidationStatus, built.ValidationErrors)
	}
	if out := os.Getenv(xsdFixtureEnv); out != "" {
		if err := os.WriteFile(out, built.XMLBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if x := string(built.XMLBytes); !strings.Contains(x, "<SourceID>U001</SourceID>") || uuidSourceID.MatchString(x) {
		t.Fatal("SourceID must be the operator saft_user_code, not the operators.id UUID")
	}
	var af auditFile
	d := xml.NewDecoder(bytes.NewReader(built.XMLBytes))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return charmap.Windows1252.NewDecoder().Reader(r), nil }
	if err := d.Decode(&af); err != nil {
		t.Fatal(err)
	}

	h := af.Header
	if h.TaxAccountingBasis != "F" || h.FiscalYear != "2026" || h.DateCreated != "2026-09-02" ||
		h.ProductCompanyTaxID != software.ProducerTaxID || h.CertificateNumber != software.CertificateNumber ||
		h.ProductID != "Farvoo/FAINA VAIDOSA, LDA" || h.ProductVersion != software.Version() {
		t.Fatalf("Header (Portaria 1.4/1.12/1.14–1.16): %+v", h)
	}
	// 4.2.1 counts every line incl. annulled; 4.2.2 sums quantities of non-annulled documents.
	wantLines, wantQty := 0, decimal.Zero
	for _, m := range af.MovementOfGoods.Docs {
		for _, ln := range m.Lines {
			wantLines++
			if m.Status != "A" {
				wantQty = wantQty.Add(dec(t, ln.Quantity))
			}
		}
	}
	if af.MovementOfGoods.Lines != wantLines || !dec(t, af.MovementOfGoods.Quantity).Equal(wantQty) {
		t.Fatalf("MovementOfGoods lines/qty %d/%s want %d/%s", af.MovementOfGoods.Lines, af.MovementOfGoods.Quantity, wantLines, wantQty)
	}

	checkLines := func(label, docType string, lines []saftLine, tot docTotals) decimal.Decimal {
		t.Helper()
		sum := decimal.Zero
		for i, ln := range lines {
			if ln.SettlementAmount != nil {
				t.Fatalf("%s line %d: SettlementAmount without a discount", label, i+1)
			}
			wantDebit := docType == "NC"
			if (ln.DebitAmount != nil) != wantDebit || (ln.CreditAmount != nil) == wantDebit {
				t.Fatalf("%s (%s) line %d: wrong side debit=%v credit=%v", label, docType, i+1, ln.DebitAmount, ln.CreditAmount)
			}
			amt := ln.CreditAmount
			if wantDebit {
				amt = ln.DebitAmount
			}
			a := dec(t, *amt)
			if drift := dec(t, ln.Quantity).Mul(dec(t, ln.UnitPrice)).Sub(a).Abs(); drift.GreaterThan(decimal.RequireFromString("0.01")) {
				t.Fatalf("%s line %d: Qty×UnitPrice drifts %s from line amount %s", label, i+1, drift, a)
			}
			sum = sum.Add(a)
		}
		if !sum.Equal(dec(t, tot.NetTotal)) {
			t.Fatalf("%s: Σ line amounts %s != NetTotal %s", label, sum, tot.NetTotal)
		}
		if !dec(t, tot.NetTotal).Add(dec(t, tot.TaxPayable)).Equal(dec(t, tot.GrossTotal)) {
			t.Fatalf("%s: Net+Tax != Gross (%+v)", label, tot)
		}
		return sum
	}

	var debit, credit decimal.Decimal
	types := map[string]bool{}
	for _, inv := range af.SalesInvoices.Invoices {
		types[inv.InvoiceType] = true
		if inv.Period != "8" {
			t.Fatalf("%s Period %q: month 1–12 without leading zero", inv.InvoiceNo, inv.Period)
		}
		s := checkLines(inv.InvoiceNo, inv.InvoiceType, inv.Lines, inv.Totals)
		if inv.InvoiceType == "NC" {
			debit = debit.Add(s)
		} else {
			credit = credit.Add(s)
		}
	}
	if !types["FT"] || !types["NC"] {
		t.Fatalf("fixture must cover FT and NC, got %v", types)
	}
	if !debit.Equal(dec(t, af.SalesInvoices.TotalDebit)) || !credit.Equal(dec(t, af.SalesInvoices.TotalCredit)) {
		t.Fatalf("SalesInvoices totals debit=%s/%s credit=%s/%s", af.SalesInvoices.TotalDebit, debit, af.SalesInvoices.TotalCredit, credit)
	}
	wd := decimal.Zero
	for _, w := range af.WorkingDocuments.Docs {
		s := checkLines("PF", "PF", w.Lines, w.Totals)
		if w.Status != "A" {
			wd = wd.Add(s)
		}
	}
	if len(af.WorkingDocuments.Docs) == 0 || !wd.Equal(dec(t, af.WorkingDocuments.TotalCredit)) {
		t.Fatalf("WorkingDocuments TotalCredit %s want %s", af.WorkingDocuments.TotalCredit, wd)
	}
	if len(af.MovementOfGoods.Docs) != 3 {
		t.Fatalf("movements %d", len(af.MovementOfGoods.Docs))
	}
	for _, m := range af.MovementOfGoods.Docs {
		checkLines("movement", "GT", m.Lines, m.Totals)
	}
}

// buildFixture issues real FT/NC through the store (amounts from compliance.LineFromGross) and
// adds PF, GR, GT, RG and a 0% exempt FT built from those lines, so every SAF-T section is present.
func buildFixture(t *testing.T) *saft.BuildResult {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sig, err := signer.LoadPEMFile(filepath.Join("..", "testdata", "dev_signing_key.pem"), 1)
	if err != nil {
		t.Fatal(err)
	}
	seedSAFTDemo(t, db, sig)
	ft, err := db.IssueFT(context.Background(), sig, store.IssueParams{
		StoreID: "store-demo-001", RequestID: "ft-saft-multi", DocType: domain.DocumentFT,
		OperatorID:       "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		NowUTC: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "LOCAL", SourceSaleID: "sale-saft-multi", ScopeType: "session", ScopeID: "s2", FiscalPurpose: "sale",
			Lines: []domain.SaleLine{
				{ProductCode: "C1", DisplayName: "Café", SaftName: "Café", Quantity: "3", UnitPriceGross: "0.85", VATRate: "0.13", ProductType: "P", UnitOfMeasure: "UN"},
				{ProductCode: "B1", DisplayName: "Coca-Cola", SaftName: "Coca-Cola", Quantity: "7", UnitPriceGross: "2.20", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN"},
				{ProductCode: "P2", DisplayName: "Pão", SaftName: "Pão", Quantity: "2", UnitPriceGross: "1.15", VATRate: "0.06", ProductType: "P", UnitOfMeasure: "UN"},
			},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
			Payments: []domain.PaymentInput{{Method: "CARD", Amount: "20.25"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertActiveSeries("store-demo-001", "NC", "NC2026DEMO01", "NCVAL1234", 2026); err != nil {
		t.Fatal(err)
	}
	if _, err := db.IssueNC(context.Background(), sig, store.IssueNCParams{
		StoreID: "store-demo-001", RequestID: "nc-saft-multi", OriginalInvoiceID: ft.DocumentID,
		OperatorID: "op-demo-cashier", FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Reason: "Devolucao", CreditFull: true, NowUTC: time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC),
	}); err != nil {
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
	var multi store.SAFTInvoice
	for _, inv := range invoices {
		if inv.ID == ft.DocumentID {
			multi = inv
		}
	}
	if len(multi.Lines) != 3 {
		t.Fatalf("multi-line FT not loaded: %+v", multi)
	}
	if !strings.Contains(multi.Lines[0].UnitPriceNet, ".") || len(strings.SplitN(multi.Lines[0].UnitPriceNet, ".", 2)[1]) != compliance.UnitPriceDecimals {
		t.Fatalf("UnitPriceNet %q not %d dp", multi.Lines[0].UnitPriceNet, compliance.UnitPriceDecimals)
	}

	exempt := multi
	exempt.ID, exempt.InvoiceNo, exempt.ATCUD = "ft-exempt", "FT FT2026DEMO01/99", "CSDF7T5H-99"
	exempt.Lines = []store.SAFTLine{{
		LineNumber: 1, ProductCode: "E1", ProductDescription: "Serviço isento", Quantity: "1", UnitOfMeasure: "UN",
		UnitPriceNet: "10.000000", LineNet: "10.00", LineTax: "0.00", LineGross: "10.00", VATRate: "0",
		TaxType: "IVA", TaxCountryRegion: "PT", TaxCode: "ISE", ProductType: "S",
		TaxExemptionCode: "M07", TaxExemptionReason: "Isento artigo 9.º do CIVA",
	}}
	exempt.NetTotal, exempt.TaxPayable, exempt.GrossTotal = "10.00", "0.00", "10.00"
	invoices = append(invoices, exempt)

	pf := store.SAFTWorkingDocument{
		ID: "pf-1", DocumentNumber: "PF PF2026DEMO01/1", ATCUD: "PFVAL123-1", WorkDate: multi.InvoiceDate,
		SystemEntryDate: multi.SystemEntryDate, WorkStatus: "N", WorkStatusDate: multi.SystemEntryDate, StatusSourceID: multi.SourceID,
		Hash: multi.Hash, HashControl: multi.HashControl, SourceID: multi.SourceID,
		GrossTotal: multi.GrossTotal, NetTotal: multi.NetTotal, TaxPayable: multi.TaxPayable,
		Customer: multi.Customer, Lines: multi.Lines,
	}
	pfCancelled := pf
	pfCancelled.ID, pfCancelled.DocumentNumber, pfCancelled.ATCUD = "pf-2", "PF PF2026DEMO01/2", "PFVAL123-2"
	pfCancelled.WorkStatus, pfCancelled.StatusReason = "A", "Cliente desistiu"
	shop := domain.Address{Detail: "Rua 1", City: "Lisboa", PostalCode: "1000-001"}
	movement := func(no, typ string) store.SAFTStockMovement {
		return store.SAFTStockMovement{
			ID: no, DocumentNumber: typ + " " + typ + "2026DEMO01/1", ATCUD: typ + "VAL123-1", MovementType: typ,
			MovementDate: multi.InvoiceDate, SystemEntryDate: multi.SystemEntryDate,
			MovementStatus: "N", MovementStatusDate: multi.SystemEntryDate, StatusSourceID: multi.SourceID,
			Hash: multi.Hash, HashControl: multi.HashControl, SourceID: multi.SourceID,
			GrossTotal: multi.GrossTotal, NetTotal: multi.NetTotal, TaxPayable: multi.TaxPayable,
			ShipFrom: shop, ShipTo: domain.Address{Detail: "Rua 2", City: "Porto", PostalCode: "4000-001"},
			MovementStartAt: "2026-08-20T17:00:00", Customer: multi.Customer, Lines: multi.Lines,
		}
	}
	// Recipients without NIF as GR/GT snapshots keep them (empty CustomerTaxID).
	grNamed := movement("gr-1", "GR")
	grNamed.Customer = store.SAFTCustomer{CompanyName: "Maria Costa", AddressDetail: "Rua 3", City: "Porto", PostalCode: "4000-002", Country: "PT", AccountID: "Desconhecido"}
	gtWalkIn := movement("gt-1", "GT")
	gtWalkIn.Customer = store.SAFTCustomer{CompanyName: "Consumidor Final", AddressDetail: "Avenida da Liberdade", City: "Lisboa", PostalCode: "1250-147", Country: "PT", AccountID: "Desconhecido"}
	grAnnulled := movement("gr-2", "GR")
	grAnnulled.DocumentNumber, grAnnulled.ATCUD = "GR GR2026DEMO01/2", "GRVAL123-2"
	grAnnulled.MovementStatus, grAnnulled.StatusReason = "A", "Cliente desistiu"
	rg := store.SAFTPayment{
		ID: "rg-1", PaymentRefNo: "RG RG2026DEMO01/1", ATCUD: "RGVAL123-1", TransactionDate: multi.InvoiceDate,
		SystemEntryDate: multi.SystemEntryDate, SourceID: multi.SourceID, GrossTotal: multi.GrossTotal,
		PaymentMethod: "CARD", Customer: multi.Customer,
		OriginalInvoiceNo: multi.InvoiceNo, OriginalInvoiceDate: multi.InvoiceDate,
	}

	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, StartDate: "2026-08-01", EndDate: "2026-08-31", CreatedDate: "2026-09-02",
		Invoices:  invoices,
		WorkDocs:  []store.SAFTWorkingDocument{pf, pfCancelled},
		Movements: []store.SAFTStockMovement{grNamed, gtWalkIn, grAnnulled},
		Payments:  []store.SAFTPayment{rg},
	})
	if err != nil {
		t.Fatal(err)
	}
	return built
}

func TestBuildFlagsZeroRateWithoutExemption(t *testing.T) {
	taxpayer := &store.TaxpayerSettings{
		TaxRegistrationNumber: "517535009", LegalName: "Farvoo Demo Lda",
		AddressDetail: "Rua Demo 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT",
	}
	inv := store.SAFTInvoice{
		DocumentType: "FT", InvoiceNo: "FT FT2026DEMO01/1", InvoiceDate: "2026-08-21",
		GrossTotal: "5.00", NetTotal: "5.00", TaxPayable: "0.00",
		Customer: store.SAFTCustomer{CustomerTaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
		Lines: []store.SAFTLine{{
			LineNumber: 1, ProductCode: "E1", ProductDescription: "Isento", Quantity: "1", UnitOfMeasure: "UN",
			LineNet: "5.00", LineGross: "5.00", VATRate: "0", TaxType: "IVA", TaxCountryRegion: "PT", TaxCode: "ISE", ProductType: "S",
		}},
	}
	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, StartDate: "2026-08-01", EndDate: "2026-08-31", CreatedDate: "2026-09-02", Invoices: []store.SAFTInvoice{inv},
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.ValidationStatus != "INVALID" || len(built.ValidationErrors) == 0 || !strings.Contains(built.ValidationErrors[0], "TaxExemption") {
		t.Fatalf("want INVALID for 0%% line without exemption, got %s %v", built.ValidationStatus, built.ValidationErrors)
	}
}
