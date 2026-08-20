//go:build windows

package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// trayLocalHTTP serves /api/health and configure/pair routes while the tray agent runs.
type trayLocalHTTP struct {
	mu           sync.Mutex
	rt           *trayRuntime
	srv          *http.Server
	addr         string
	configureMux *http.ServeMux
}

var trayLocal trayLocalHTTP

func (t *trayLocalHTTP) listenAddr() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.addr
}

func (t *trayLocalHTTP) mountConfigureRoutes(configPath string) {
	cfg := reloadConfig(configPath, &config{})
	cfgPtr := &cfg
	mux := http.NewServeMux()
	// done=nil: tray keeps routes until exit; configure-done only closes the browser tab.
	// Pair while already Connected: rebindTrayAgentWork (in-process), not process restart.
	onPairSuccess := func() {
		if t.rt != nil {
			t.rt.onPairConfigSaved()
		}
	}
	registerConfigureWizardRoutes(mux, configPath, cfgPtr, nil, onPairSuccess)
	t.mu.Lock()
	t.configureMux = mux
	t.mu.Unlock()
}

func startTrayLocalHTTP(rt *trayRuntime) {
	trayLocal.mu.Lock()
	if trayLocal.srv != nil {
		trayLocal.mu.Unlock()
		return
	}
	trayLocal.rt = rt
	trayLocal.mu.Unlock()

	configPath := defaultConfigPath()
	addr, err := pickLocalListenAddr(ConfigureWizardPort)
	if err != nil {
		log.Println("tray: local http:", err)
		return
	}
	trayLocal.mountConfigureRoutes(configPath)

	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(trayLocal.serveHTTP),
		ReadHeaderTimeout: 10 * time.Second,
	}
	trayLocal.mu.Lock()
	trayLocal.addr = addr
	trayLocal.srv = srv
	trayLocal.mu.Unlock()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("tray: local http:", err)
		}
	}()
	log.Println("tray: local http on", addr)
}

func shutdownTrayLocalHTTP() {
	trayLocal.mu.Lock()
	srv := trayLocal.srv
	trayLocal.srv = nil
	trayLocal.configureMux = nil
	trayLocal.addr = ""
	trayLocal.mu.Unlock()
	if srv == nil {
		return
	}
	shutdownHTTPServer(srv, 2*time.Second)
}

func setTrayLocalCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if strings.EqualFold(r.Header.Get("Access-Control-Request-Private-Network"), "true") {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
}

func (t *trayLocalHTTP) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		setTrayLocalCORS(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/health" {
		setTrayLocalCORS(w, r)
		writePairJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"version": Version,
		})
		return
	}

	if isTrayConfigurePath(r.URL.Path) {
		setTrayLocalCORS(w, r)
		t.mu.Lock()
		mux := t.configureMux
		t.mu.Unlock()
		if mux == nil {
			writePairJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "print agent still starting; try again in a few seconds",
			})
			return
		}
		mux.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

// runConfigureSession is kept for non-tray fallbacks; tray uses permanently mounted routes.
func (t *trayLocalHTTP) runConfigureSession(ctx context.Context, configPath, prefillAPI, rawQuery string) error {
	if t.srv == nil {
		return runConfigureWizard(ctx, configPath, prefillAPI, rawQuery)
	}
	addr := t.listenAddr()
	if addr == "" {
		return runConfigureWizard(ctx, configPath, prefillAPI, rawQuery)
	}
	baseURL := configureWizardBaseURL(addr, prefillAPI, rawQuery)
	if onConfigureWizardReady != nil {
		onConfigureWizardReady(baseURL)
	}
	announceWizardURL(productName+" 打印机设置", baseURL)
	<-ctx.Done()
	return ctx.Err()
}
