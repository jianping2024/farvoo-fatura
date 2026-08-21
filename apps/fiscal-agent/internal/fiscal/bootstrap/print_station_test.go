package bootstrap_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestPrintBytesFn_UsesStationMapping(t *testing.T) {
	dir := t.TempDir()
	pem := filepath.Join("..", "testdata", "dev_signing_key.pem")
	var got atomic.Value
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath: filepath.Join(dir, "f.db"), DataDir: filepath.Join(dir, "sec"),
		StoreID: "store-demo-001", Seed: true, SigningKeyPEMPath: pem,
		PrintSink: &worker.MemorySink{},
		StationPrintersFn: func() map[string]string {
			return map[string]string{"st-usb": "winspool:UK56009"}
		},
		PrintBytesFn: func(printerRaw string, data []byte) error {
			got.Store(printerRaw)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	_, err = rt.Service.IssueDocument(context.Background(), domain.IssueRequest{
		StoreID: "store-demo-001", RequestID: "req-print-1", OperatorID: "op-demo-cashier",
		StationID: "st-usb",
		Snapshot: domain.SaleSnapshot{
			SourceSystem: "farvoo", SourceSaleID: "sale-p", ScopeType: "whole_table", ScopeID: "sale-p",
			FiscalPurpose: "sale",
			Lines: []domain.SaleLine{{
				ProductCode: "P1", DisplayName: "Item", SaftName: "Item", Quantity: "1",
				UnitPriceGross: "1.00", VATRate: "0.23", ProductType: "P", UnitOfMeasure: "UN",
			}},
			Payments: []domain.PaymentInput{{Method: "CASH", Amount: "1.00"}},
			Customer: domain.CustomerInput{TaxID: "999999990", CompanyName: "Consumidor Final", Country: "PT"},
		},
	}, domain.DocumentFT)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if v := got.Load(); v != nil {
			if v.(string) != "winspool:UK56009" {
				t.Fatalf("got raw %q", v)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("PrintBytesFn not called")
}

func TestIssueFromBillDraft_RequiresStationID(t *testing.T) {
	dir := t.TempDir()
	pem := filepath.Join("..", "testdata", "dev_signing_key.pem")
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath: filepath.Join(dir, "f.db"), DataDir: filepath.Join(dir, "sec"),
		StoreID: "store-demo-001", Seed: true, SigningKeyPEMPath: pem,
		PrintSink: &worker.MemorySink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	_, err = rt.Service.IssueFromBillDraft(context.Background(), service.IssueBillDraftInput{
		DraftID: "x", OperatorID: "op-demo-cashier", Mode: "whole_table",
	})
	if err == nil {
		t.Fatal("expected validation_failed for empty station_id")
	}
	var ce *service.CodedError
	if !errors.As(err, &ce) || ce.Code != "validation_failed" {
		t.Fatalf("got %v", err)
	}
}
