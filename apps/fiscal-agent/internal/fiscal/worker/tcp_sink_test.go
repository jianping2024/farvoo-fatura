package worker_test

import (
	"os"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestResolveFiscalPrinterTCP(t *testing.T) {
	t.Setenv("FISCAL_PRINTER_TCP", "")
	_ = os.Unsetenv("FISCAL_PRINTER_TCP")
	if got := worker.ResolveFiscalPrinterTCP(map[string]string{
		"fiscal_receipt_printer": "tcp:127.0.0.1:9100",
	}); got != "127.0.0.1:9100" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("FISCAL_PRINTER_TCP", "10.0.0.9:9100")
	if got := worker.ResolveFiscalPrinterTCP(nil); got != "10.0.0.9:9100" {
		t.Fatalf("env override got %q", got)
	}
}
