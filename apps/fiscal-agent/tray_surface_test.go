package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Simulates the tray mux surface (17892 routes) without Windows tray build tags.
func TestTrayConfigureSurfaceHealthAndPair(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := reloadConfig(path, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, path, cfgPtr, nil, nil)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/health" {
			writePairJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
			return
		}
		if isTrayConfigurePath(r.URL.Path) {
			mux.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Fatalf("health: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/pair", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("pair status %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "pairForm") {
		t.Fatal("pair page missing form")
	}

	req = httptest.NewRequest(http.MethodGet, "/configure", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("configure status %d", rr.Code)
	}
}

func TestLiveTraySurfaceListenAndWaitPair(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := reloadConfig(path, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, path, cfgPtr, nil, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/health" {
				writePairJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
				return
			}
			if isTrayConfigurePath(r.URL.Path) {
				mux.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go srv.Serve(ln)
	defer srv.Close()

	client := &http.Client{Timeout: 3 * time.Second}
	base := "http://" + addr
	for _, p := range []string{"/api/health", "/pair", "/configure"} {
		res, err := client.Get(base + p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d %s", p, res.StatusCode, body)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- waitPairingOnListenAddr(ctx, path, "http://192.168.0.141", addr)
	}()
	time.Sleep(200 * time.Millisecond)
	if err := saveConfig(path, &config{APIBase: "http://192.168.0.141", AgentJWT: "jwt", DeviceID: "d1"}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	wantPrefix := "http://" + addr + "/pair?api="
	got := pairWizardBaseURL(addr, "http://192.168.0.141/")
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("pair url %q want prefix %q", got, wantPrefix)
	}
}

func TestWaitPairingCancels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := waitPairingOnListenAddr(ctx, path, "", "127.0.0.1:17892")
	if err == nil {
		t.Fatal("expected cancel error")
	}
}
