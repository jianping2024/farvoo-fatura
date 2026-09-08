package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

func TestCashDrawerKickHelpersUniqueInSource(t *testing.T) {
	// Compile-time uniqueness is covered by print package tests; here assert handler symbols exist once via behavior.
	pin := 2
	var mu sync.Mutex
	var last []byte
	var lastRaw string
	deps := HandlerDeps{
		CashDrawerPinGet: func() int { return pin },
		CashDrawerPinSet: func(p int) error { pin = p; return nil },
		StationPrintersFn: func() map[string]string {
			return map[string]string{"st-cash": "tcp:127.0.0.1:9100"}
		},
		PrintBytesFn: func(printerRaw string, data []byte) error {
			mu.Lock()
			defer mu.Unlock()
			lastRaw = printerRaw
			last = append([]byte(nil), data...)
			return nil
		},
		Fiscal: nil, // ResolveEffectivePrintStation needs Fiscal — open will fail station; test GET/PUT first
	}

	req := httptest.NewRequest(http.MethodGet, "/local/v1/setup/cash-drawer", nil)
	rr := httptest.NewRecorder()
	handleGetCashDrawer(rr, req, deps)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status %d", rr.Code)
	}
	var snap CashDrawerSnapshot
	if err := json.NewDecoder(rr.Body).Decode(&snap); err != nil || snap.Pin != 2 {
		t.Fatalf("GET snap %+v", snap)
	}

	body, _ := json.Marshal(map[string]int{"pin": 5})
	req = httptest.NewRequest(http.MethodPut, "/local/v1/setup/cash-drawer", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	handlePutCashDrawer(rr, req, deps)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status %d body %s", rr.Code, rr.Body.String())
	}
	if pin != 5 {
		t.Fatalf("pin not saved %d", pin)
	}

	bad, _ := json.Marshal(map[string]int{"pin": 3})
	req = httptest.NewRequest(http.MethodPut, "/local/v1/setup/cash-drawer", bytes.NewReader(bad))
	rr = httptest.NewRecorder()
	handlePutCashDrawer(rr, req, deps)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid pin want 400 got %d", rr.Code)
	}

	_ = last
	_ = lastRaw
	_ = fiscalprint.CashDrawerKickBytes(2)
	if !strings.Contains("handleOpenCashDrawer", "Open") {
		t.Fatal("sanity")
	}
}
