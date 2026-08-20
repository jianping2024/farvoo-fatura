package main

import (
	"context"
	_ "embed"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed configure_ui.html
var configureUIHTML []byte

func registerConfigureWizardRoutes(mux *http.ServeMux, configPath string, cfgPtr **config, done chan<- error, onPairSuccess func()) {
	mux.HandleFunc("/configure", func(w http.ResponseWriter, r *http.Request) {
		writeBrandHTML(w, configureUIHTML)
	})

	registerPairWebRoutes(mux, configPath, cfgPtr, "configure wizard", onPairSuccess)
	registerPrinterWizardRoutes(mux, configPath, cfgPtr, "configure wizard")
	registerUILocaleRoute(mux, configPath, cfgPtr) // pair page at /pair

	mux.HandleFunc("/api/configure-state", func(w http.ResponseWriter, r *http.Request) {
		*cfgPtr = reloadConfig(configPath, *cfgPtr)
		writeConfigureState(w, *cfgPtr)
	})

	mux.HandleFunc("/api/configure-done", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writePairJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		if done != nil {
			select {
			case done <- nil:
			default:
			}
		}
	})
}

func configureWizardBaseURL(listenAddr, prefillAPI, rawQuery string) string {
	baseURL := "http://" + listenAddr + "/configure"
	q := url.Values{}
	if api, err := normalizeAPIBase(prefillAPI); err == nil && api != "" {
		q.Set("api", api)
	}
	if rawQuery != "" {
		if incoming, err := url.ParseQuery(rawQuery); err == nil {
			if api := strings.TrimSpace(incoming.Get("api")); api != "" {
				q.Set("api", api)
			}
			if code := strings.TrimSpace(incoming.Get("code")); code != "" {
				q.Set("code", code)
			}
		}
	}
	if len(q) > 0 {
		baseURL += "?" + q.Encode()
	}
	return baseURL
}

// runConfigureWizard serves printer mapping (/configure) and /pair on the same local server (standalone CLI).
func runConfigureWizard(ctx context.Context, configPath string, prefillAPI, rawQuery string) error {
	listenAddr, err := pickLocalListenAddr(ConfigureWizardPort)
	if err != nil {
		return err
	}

	cfg := reloadConfig(configPath, &config{})
	cfgPtr := &cfg

	done := make(chan error, 1)
	mux := http.NewServeMux()
	registerConfigureWizardRoutes(mux, configPath, cfgPtr, done, nil)

	srv := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			done <- err
		}
	}()

	baseURL := configureWizardBaseURL(listenAddr, prefillAPI, rawQuery)
	agentLogLocale(localeFromConfigPath(configPath), "log_wizard_open", baseURL)
	if onConfigureWizardReady != nil {
		onConfigureWizardReady(baseURL)
	}
	announceWizardURL(productName+" 打印机设置", baseURL)

	return waitLocalWizard(ctx, srv, done)
}
