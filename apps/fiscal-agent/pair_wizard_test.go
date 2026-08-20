package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAPIBase(t *testing.T) {
	got, err := normalizeAPIBase("https://restaurant-ordering-beryl-three.vercel.app/dashboard/settings/print-assistant")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://restaurant-ordering-beryl-three.vercel.app"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPairWizardBaseURL(t *testing.T) {
	if got := pairWizardBaseURL("127.0.0.1:17892", ""); got != "http://127.0.0.1:17892/pair" {
		t.Fatalf("empty prefill: got %q", got)
	}
	got := pairWizardBaseURL("127.0.0.1:17892", "http://192.168.0.141/")
	want := "http://127.0.0.1:17892/pair?api=http%3A%2F%2F192.168.0.141"
	if got != want {
		t.Fatalf("with api: got %q want %q", got, want)
	}
}

func TestDedupePrinterList(t *testing.T) {
	got := dedupePrinterList([]printerListEntry{
		{Addr: "winspool:UK56009", Label: "UK56009"},
		{Addr: " winspool:uk56009 ", Label: "duplicate"},
		{Addr: "tcp:192.168.1.50:9100", Label: "LAN"},
	})
	if len(got) != 2 {
		t.Fatalf("got %d entries: %#v", len(got), got)
	}
	if got[0].Addr != "winspool:UK56009" || got[1].Addr != "tcp:192.168.1.50:9100" {
		t.Fatalf("unexpected entries: %#v", got)
	}
}

func TestPairSuccessInvokesOnSuccess(t *testing.T) {
	prev := pairSuccessOnSuccessDelay
	pairSuccessOnSuccessDelay = 0
	defer func() { pairSuccessOnSuccessDelay = prev }()

	var gotAPIBase string
	var mesa *httptest.Server
	mesa = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/print-agent/claim" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			APIBase string `json:"api_base"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotAPIBase = body.APIBase
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agentjwt":      "test-jwt",
			"supabase_url":  mesa.URL,
			"restaurant_id": "00000000-0000-4000-8000-000000000001",
			"valid_until":   time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		})
	}))
	defer mesa.Close()

	called := 0
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := reloadConfig(path, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, path, cfgPtr, nil, func() { called++ })

	body := `{"api_base":"` + mesa.URL + `","code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/pair", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if called != 1 {
		t.Fatalf("onPairSuccess called %d times, want 1", called)
	}
	wantBase := strings.TrimRight(mesa.URL, "/")
	if gotAPIBase != wantBase {
		t.Fatalf("claim api_base=%q want %q", gotAPIBase, wantBase)
	}
}

func TestSchedulePairOnSuccessImmediate(t *testing.T) {
	prev := pairSuccessOnSuccessDelay
	pairSuccessOnSuccessDelay = 0
	defer func() { pairSuccessOnSuccessDelay = prev }()

	called := false
	schedulePairOnSuccess(func() { called = true })
	if !called {
		t.Fatal("expected immediate onSuccess when delay is 0")
	}
}

func TestSchedulePairOnSuccessDeferred(t *testing.T) {
	prev := pairSuccessOnSuccessDelay
	pairSuccessOnSuccessDelay = 30 * time.Millisecond
	defer func() { pairSuccessOnSuccessDelay = prev }()

	called := make(chan struct{}, 1)
	schedulePairOnSuccess(func() { called <- struct{}{} })
	select {
	case <-called:
		t.Fatal("onSuccess ran before delay")
	case <-time.After(5 * time.Millisecond):
	}
	select {
	case <-called:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("onSuccess not called after delay")
	}
}

func TestPairUISuccessNavigatesToConfigure(t *testing.T) {
	html := string(pairUIHTML)
	if !strings.Contains(html, "location.replace(configurePath())") {
		t.Fatal("pair_ui.html must location.replace to /configure on success")
	}
	if strings.Contains(html, "showConfigureLinkIfAvailable") {
		t.Fatal("pair_ui.html must not keep optional configure-link success path")
	}
	if strings.Contains(html, `id="ok"`) {
		t.Fatal("pair_ui.html must not keep parallel success panel")
	}
}
