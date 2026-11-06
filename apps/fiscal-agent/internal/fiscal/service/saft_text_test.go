package service_test

import (
	"context"
	"errors"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func wantSAFTText(t *testing.T, err error, field, reason string) {
	t.Helper()
	var te *domain.SAFTTextError
	if !errors.As(err, &te) || te.Field != field || te.Reason != reason {
		t.Fatalf("want SAF-T text %s/%s, got %v", field, reason, err)
	}
}

func ftRequest(id string, line domain.SaleLine, cust domain.CustomerInput) domain.IssueRequest {
	line.Quantity, line.UnitPriceGross, line.VATRate = "1", "10.00", "0.23"
	return domain.IssueRequest{
		StoreID: "store-demo-001", RequestID: id, OperatorID: "op-demo-cashier",
		FiscalTerminalID: domain.LoopbackFiscalTerminalID, FiscalTerminalLabel: "127.0.0.1",
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "manual", SourceSaleID: id, ScopeType: "manual", ScopeID: id, FiscalPurpose: "sale",
			Lines:    []domain.SaleLine{line},
			Customer: cust,
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "10.00"}},
		},
	}
}

// New sales: a SAF-T text the file cannot hold is refused before signing, so no number is taken;
// the frozen line keeps the cleaned (NFC, trimmed) name.
func TestIssueDocument_RefusesNonSAFTText(t *testing.T) {
	svc, db, _ := transportFixture(t)
	ctx := context.Background()
	walkIn := domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"}

	_, err := svc.IssueDocument(ctx, ftRequest("ft-cjk", domain.SaleLine{ProductCode: "jincha", DisplayName: "金叉套餐"}, walkIn), domain.DocumentFT)
	wantSAFTText(t, err, "lines.1.saft_name", domain.SAFTTextEncoding)
	_, err = svc.IssueDocument(ctx, ftRequest("ft-short", domain.SaleLine{ProductCode: "P", DisplayName: "花茶", SaftName: "P"}, walkIn), domain.DocumentFT)
	wantSAFTText(t, err, "lines.1.saft_name", domain.SAFTTextTooShort)
	_, err = svc.IssueDocument(ctx, ftRequest("ft-code", domain.SaleLine{ProductCode: "金叉", SaftName: "Garfo dourado"}, walkIn), domain.DocumentFT)
	wantSAFTText(t, err, "lines.1.product_code", domain.SAFTTextEncoding)
	buyer := domain.CustomerInput{TaxID: "509442013", CompanyName: "Cliente Lda", City: "里斯本", Country: "PT"}
	_, err = svc.IssueDocument(ctx, ftRequest("ft-city", domain.SaleLine{ProductCode: "P1", SaftName: "Prato"}, buyer), domain.DocumentFT)
	wantSAFTText(t, err, "customer.city", domain.SAFTTextEncoding)

	res, err := svc.IssueDocument(ctx, ftRequest("ft-ok", domain.SaleLine{ProductCode: "P1", DisplayName: "花茶", SaftName: " Chá "}, walkIn), domain.DocumentFT)
	if err != nil {
		t.Fatal(err)
	}
	if res.InvoiceNo != "FT FT2026DEMO01/1" {
		t.Fatalf("refused sales must not take numbers: got %s", res.InvoiceNo)
	}
	var desc string
	if err := db.SQL.QueryRow(`SELECT product_description FROM invoice_lines WHERE invoice_id = ?`, res.DocumentID).Scan(&desc); err != nil {
		t.Fatal(err)
	}
	if desc != "Chá" {
		t.Fatalf("frozen description %q, want NFC trimmed Chá", desc)
	}
}

// GR: bad line or load/unload text is refused before AT is called.
func TestIssueDeliveryNote_SAFTTextNeverReachesAT(t *testing.T) {
	svc, _, ct := transportFixture(t)
	req := grRequest("gr-cjk-line")
	req.Snapshot.Lines[0].SaftName = "金叉套餐"
	_, err := svc.IssueDeliveryNote(context.Background(), req)
	wantSAFTText(t, err, "lines.1.saft_name", domain.SAFTTextEncoding)

	req = grRequest("gr-cjk-ship")
	req.ShipTo = domain.Address{Detail: "仓库 1", City: "Porto", PostalCode: "4000-001"}
	_, err = svc.IssueDeliveryNote(context.Background(), req)
	wantSAFTText(t, err, "ship_to.address_detail", domain.SAFTTextEncoding)
	if len(ct.docs) != 0 {
		t.Fatalf("AT must not be called (%d calls)", len(ct.docs))
	}
}

// Master data: product, customer and store settings refuse SAF-T text they could not export.
func TestMasterData_RefusesNonSAFTText(t *testing.T) {
	svc, _, _ := transportFixture(t)
	_, err := svc.UpsertLocalProduct(store.LocalProductInput{
		ProductCode: "jincha", DisplayName: "金叉套餐", UnitPriceGross: "10.00", VATRate: "23.00",
	})
	wantSAFTText(t, err, "product.saft_name", domain.SAFTTextEncoding)
	row, err := svc.UpsertLocalProduct(store.LocalProductInput{
		ProductCode: "jincha", DisplayName: "金叉套餐", SaftName: "Menu garfo dourado", UnitPriceGross: "10.00", VATRate: "23.00",
	})
	if err != nil || row.SaftName != "Menu garfo dourado" || row.DisplayName != "金叉套餐" {
		t.Fatalf("pt saft name with cjk display: %+v %v", row, err)
	}

	_, err = svc.UpsertLocalCustomer(store.LocalCustomerInput{CustomerTaxID: "509442013", CompanyName: "王氏贸易"})
	wantSAFTText(t, err, "customer.company_name", domain.SAFTTextEncoding)

	err = svc.UpsertTaxpayer(store.TaxpayerInput{
		TaxRegistrationNumber: "519655290", LegalName: "金叉餐厅", AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001",
	})
	wantSAFTText(t, err, "taxpayer.legal_name", domain.SAFTTextEncoding)
}
