package bootstrap_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestCashIssueAppendsDrawerKick_CardDoesNot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fiscal.db")
	key := filepath.Join("..", "testdata", "dev_signing_key.pem")
	var last []byte
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath: dbPath, DataDir: filepath.Join(dir, "secure"), StoreID: "store-demo-001",
		SigningKeyPEMPath: key, Seed: true,
		StationPrintersFn: func() map[string]string { return map[string]string{"st": "tcp:127.0.0.1:9100"} },
		PrintBytesFn: func(printerRaw string, data []byte) error {
			last = append([]byte(nil), data...)
			return nil
		},
		CashDrawerPinGet: func() int { return 2 },
		CashDrawerPinSet: func(int) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	_, err = rt.Service.UpsertLocalProduct(store.LocalProductInput{
		ProductCode: "T1", DisplayName: "Agua", SaftName: "Agua",
		UnitPriceGross: "1.00", VATRate: "13", TaxCode: "RED",
	})
	if err != nil {
		t.Fatal(err)
	}

	issue := func(pay, req string) {
		t.Helper()
		last = nil
		_, err := rt.Service.IssueManualFT(context.Background(), catalog.ManualIssueInput{
			RequestID: req, DocumentType: "FS", PaymentMethod: pay,
			CustomerName: "Consumidor Final",
			Lines:        []catalog.ManualLineInput{{ProductCode: "T1", Quantity: "1"}},
		}, "op-demo-cashier", "st", domain.LoopbackFiscalTerminalID, "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if len(last) > 0 {
				return
			}
			_, _ = rt.Worker.RunOnce(context.Background())
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("print bytes not captured")
	}

	issue(domain.PaymentCash, "req-cash-1")
	kick := fiscalprint.CashDrawerKickBytes(2)
	if !bytes.HasSuffix(last, kick) {
		t.Fatalf("cash print must end with kick; len=%d suffix=%x", len(last), last[max(0, len(last)-5):])
	}
	cashLen := len(last)

	issue(domain.PaymentCard, "req-card-1")
	if bytes.HasSuffix(last, kick) {
		t.Fatal("card print must not end with kick")
	}
	if len(last) >= cashLen {
		t.Fatalf("card len %d should be shorter than cash %d by kick size", len(last), cashLen)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
