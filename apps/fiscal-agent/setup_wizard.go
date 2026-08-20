package main

import (
	"context"
	_ "embed"
	"net/http"
	"time"
)

//go:embed setup_ui.html
var setupUIHTML []byte

// runSetupWizard serves printer selection UI until saved or ctx cancelled.
func runSetupWizard(ctx context.Context, configPath string, cfg *config) error {
	listenAddr, err := pickLocalListenAddr(SetupWizardPort)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	mux := http.NewServeMux()

	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		writeBrandHTML(w, setupUIHTML)
	})

	registerPrinterWizardRoutes(mux, configPath, &cfg, "setup wizard")
	registerUILocaleRoute(mux, configPath, &cfg)

	mux.HandleFunc("/api/setup-done", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writePairJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		done <- nil
	})

	srv := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			done <- err
		}
	}()

	baseURL := "http://" + listenAddr + "/setup"
	agentLogLocale(localeFromConfigPath(configPath), "log_wizard_open", baseURL)
	announceWizardURL(productName+" 打印机映射", baseURL)

	return waitLocalWizard(ctx, srv, done)
}
