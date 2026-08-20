package main

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrintersAPIForceRefresh(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/printers?refresh=1", nil)
	if !printersAPIForceRefresh(r) {
		t.Fatal("expected refresh")
	}
	r = httptest.NewRequest("GET", "/api/printers", nil)
	if printersAPIForceRefresh(r) {
		t.Fatal("expected no refresh")
	}
}

func TestDiscoverPrintersForAPICache(t *testing.T) {
	printerDiscoverMu.Lock()
	printerDiscover = printerDiscoverSnapshot{
		tcp:      []printerListEntry{{Addr: "tcp:1.2.3.4:9100", Label: "x"}},
		winspool: nil,
		at:       time.Now(),
	}
	printerDiscoverMu.Unlock()

	tcp, win, err := discoverPrintersForAPI(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tcp) != 1 || tcp[0].Addr != "tcp:1.2.3.4:9100" {
		t.Fatalf("cache tcp: %+v", tcp)
	}
	if len(win) != 0 {
		t.Fatalf("cache win: %+v", win)
	}
}
