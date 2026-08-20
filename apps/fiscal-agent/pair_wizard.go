package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PairWizardPort is the localhost HTTP port for the pairing web UI (dashboard links must match).
const PairWizardPort = 17890

// SetupWizardPort is the localhost port for printer setup UI (dashboard need not link).
const SetupWizardPort = 17891

// ConfigureWizardPort is the printer mapping UI (/configure).
const ConfigureWizardPort = 17892

//go:embed pair_ui.html
var pairUIHTML []byte

// registerPairWebRoutes serves /pair and POST /api/pair on the given mux (configure tray session or pair wizard).
func registerPairWebRoutes(mux *http.ServeMux, configPath string, cfg **config, logPrefix string, onSuccess func()) {
	mux.HandleFunc("/pair", func(w http.ResponseWriter, r *http.Request) {
		writeBrandHTML(w, pairUIHTML)
	})
	registerPairWizardRoute(mux, configPath, cfg, logPrefix, onSuccess)
}

func normalizeAPIBase(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%s 网址不能为空", productName)
	}
	if idx := strings.Index(s, "/dashboard"); idx > 0 {
		s = strings.TrimRight(s[:idx], "/")
	}
	if idx := strings.Index(s, "/auth"); idx > 0 {
		s = strings.TrimRight(s[:idx], "/")
	}
	s = strings.TrimRight(s, "/")
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return "", fmt.Errorf("网址须以 http:// 或 https:// 开头")
	}
	return s, nil
}

func pickLocalListenAddr(startPort int) (string, error) {
	for port := startPort; port < startPort+8; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = ln.Close()
		return addr, nil
	}
	return "", fmt.Errorf("no free port near %d", startPort)
}

// runPairingWizard serves a local web UI until pairing + optional configure-done, or ctx cancel.
// prefillAPI is optional (e.g. from -api flag); query ?api= and ?code= override in the browser.
// Same mux as tray: /pair and /configure so pair success can location.replace to /configure.
func runPairingWizard(ctx context.Context, configPath, prefillAPI string) error {
	listenAddr, err := pickLocalListenAddr(PairWizardPort)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	mux := http.NewServeMux()
	var cfg *config
	cfgPtr := &cfg
	registerConfigureWizardRoutes(mux, configPath, cfgPtr, done, nil)
	registerUILocaleRoute(mux, configPath, cfgPtr)

	srv := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			done <- err
		}
	}()

	baseURL := pairWizardBaseURL(listenAddr, prefillAPI)

	agentLogLocale(localeFromConfigPath(configPath), "log_wizard_open", baseURL)
	announceWizardURL(productName+" 配对", baseURL)

	return waitLocalWizard(ctx, srv, done)
}

// pairWizardBaseURL is the single local pairing page URL builder (tray 17892 or CLI 17890).
func pairWizardBaseURL(listenAddr, prefillAPI string) string {
	baseURL := "http://" + listenAddr + "/pair"
	if api, err := normalizeAPIBase(prefillAPI); err == nil && api != "" {
		return "http://" + listenAddr + "/pair?api=" + url.QueryEscape(api)
	}
	return baseURL
}

func writePairJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// pairSuccessOnSuccessDelay is for tests that need to observe async onSuccess.
// Product path runs onSuccess immediately (rebind does not kill :17892).
var pairSuccessOnSuccessDelay = time.Duration(0)

func schedulePairOnSuccess(onSuccess func()) {
	if onSuccess == nil {
		return
	}
	d := pairSuccessOnSuccessDelay
	if d <= 0 {
		onSuccess()
		return
	}
	time.AfterFunc(d, onSuccess)
}
