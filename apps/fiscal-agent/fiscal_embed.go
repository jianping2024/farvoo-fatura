package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/service"
)

// embeddedFiscal is the single Agent-side fiscal runtime (tray / run / fiscal-standalone).
var (
	embeddedFiscalMu  sync.Mutex
	embeddedFiscal    *bootstrap.Runtime
	embeddedFiscalURL string // listen URL (may be http://0.0.0.0:17880); shell uses loopbackAdminURL
)

// fiscalBillSyncPuller returns a Puller bound to the embedded fiscal DB, or nil.
func fiscalBillSyncPuller(cfg *config) *billsync.Puller {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscal == nil || embeddedFiscal.DB == nil || cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.APIBase) == "" || strings.TrimSpace(cfg.AgentJWT) == "" {
		return nil
	}
	return &billsync.Puller{APIBase: cfg.APIBase, JWT: cfg.AgentJWT, DB: embeddedFiscal.DB}
}

// applyFiscalRuntimeFromConfig installs process env used by fiscal packages
// (local provision + AT env only). LAN listen is NOT via env — see resolveFiscalListenBind.
// ONLY agent-side applicator.
func applyFiscalRuntimeFromConfig(cfg *config) {
	if strings.TrimSpace(os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION")) == "" {
		allow := false
		if cfg != nil && cfg.FiscalAllowLocalProvision != nil {
			allow = *cfg.FiscalAllowLocalProvision
		}
		if allow {
			_ = os.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "1")
		} else {
			_ = os.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "0")
		}
	}
	if strings.TrimSpace(os.Getenv("FISCAL_AT_ENV")) == "" {
		at := "mock"
		if cfg != nil {
			if v := strings.TrimSpace(cfg.FiscalATEnv); v != "" {
				at = v
			}
		}
		_ = os.Setenv("FISCAL_AT_ENV", at)
	}
}

const defaultEmbedStoreID = "store-demo-001"

// resolveEmbedStoreID is the ONLY Agent-embed StoreID resolver (cold open before Mesa included).
// Order: FISCAL_STORE_ID → cfg.RestaurantID → disk config restaurant_id → defaultEmbedStoreID.
func resolveEmbedStoreID(cfg *config) string {
	if v := strings.TrimSpace(os.Getenv("FISCAL_STORE_ID")); v != "" {
		return v
	}
	if cfg != nil {
		if id := strings.TrimSpace(cfg.RestaurantID); id != "" {
			return id
		}
	}
	if c, err := loadConfig(defaultConfigPath()); err == nil && c != nil {
		if id := strings.TrimSpace(c.RestaurantID); id != "" {
			return id
		}
	}
	return defaultEmbedStoreID
}

// startEmbeddedFiscal starts Fiscal Core inside the Agent process — ONLY agent embed entry.
func startEmbeddedFiscal(cfg *config) error {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()

	applyFiscalRuntimeFromConfig(cfg)
	allowLAN, bind := resolveFiscalListenBind()
	wantURL := "http://" + bind

	if embeddedFiscal != nil {
		if embeddedFiscalURL == wantURL {
			if cfg != nil && embeddedFiscal.Service != nil {
				embeddedFiscal.Service.SetCloudProvision(service.CloudProvision{
					APIBase:  strings.TrimSpace(cfg.APIBase),
					JWT:      strings.TrimSpace(cfg.AgentJWT),
					DeviceID: strings.TrimSpace(cfg.DeviceID),
				})
				go embeddedFiscal.Service.TryPullCloudProvisionIfNeeded(context.Background())
			}
			return nil
		}
		// Bind intent changed (e.g. cold shell started loopback before disk LAN applied).
		log.Printf("fiscal: rebind %s → %s", embeddedFiscalURL, wantURL)
		_ = embeddedFiscal.Close()
		embeddedFiscal = nil
		embeddedFiscalURL = ""
	}

	dataRoot := agentDataDir()
	if dataRoot == "" {
		dataRoot = filepath.Join(os.TempDir(), "farvoo-fiscal-agent")
	}
	dbPath := filepath.Join(dataRoot, "fiscal.db")
	dataDir := filepath.Join(dataRoot, "fiscal-secure")
	if v := strings.TrimSpace(os.Getenv("FISCAL_DB")); v != "" {
		dbPath = v
	}
	if v := strings.TrimSpace(os.Getenv("FISCAL_DATA_DIR")); v != "" {
		dataDir = v
	}
	storeID := resolveEmbedStoreID(cfg)

	var stations map[string]string
	configPath := defaultConfigPath()
	if cfg != nil {
		stations = cfg.StationPrinters
	}
	stationsFn := func() map[string]string {
		c, err := loadConfig(configPath)
		if err != nil || c == nil {
			if stations == nil {
				return map[string]string{}
			}
			return stations
		}
		return c.StationPrinters
	}
	printBytes := func(printerRaw string, data []byte) error {
		t, err := parsePrinterTarget(printerRaw)
		if err != nil {
			return err
		}
		return printToTarget(t, data)
	}

	opts := bootstrap.Options{
		DBPath:            dbPath,
		DataDir:           dataDir,
		BindAddr:          bind,
		AllowLAN:          allowLAN,
		StoreID:           storeID,
		StationPrintersFn: stationsFn,
		StationMetaFn:     stationMetaFnForConfigPath(configPath),
		PrintBytesFn:      printBytes,
		Seed:              os.Getenv("FISCAL_SEED") == "1",
		UILocaleGet:           loadAgentUILocale,
		UILocaleSet:           setAgentUILocale,
		LanAccessGet:          agentLanAccessGet,
		LanAccessSet:          agentLanAccessSet,
		AutoSessionSecretFile: true, // ONLY Retail embed entry; see docs/fiscal-session-secret.zh.md
	}
	if opts.Seed {
		cwd, _ := os.Getwd()
		opts.SigningKeyPEMPath = filepath.Join(cwd, "internal", "fiscal", "testdata", "dev_signing_key.pem")
		if _, err := os.Stat(opts.SigningKeyPEMPath); err != nil {
			opts.SigningKeyPEMPath = filepath.Join("internal", "fiscal", "testdata", "dev_signing_key.pem")
		}
	}

	rt, err := bootstrap.Start(opts)
	if err != nil {
		return err
	}
	if cfg != nil {
		rt.Service.SetCloudProvision(service.CloudProvision{
			APIBase:  strings.TrimSpace(cfg.APIBase),
			JWT:      strings.TrimSpace(cfg.AgentJWT),
			DeviceID: strings.TrimSpace(cfg.DeviceID),
		})
		go rt.Service.TryPullCloudProvisionIfNeeded(context.Background())
	}
	embeddedFiscal = rt
	embeddedFiscalURL = wantURL
	log.Printf("fiscal: embedded on %s db=%s store=%s", embeddedFiscalURL, dbPath, storeID)
	return nil
}

func stopEmbeddedFiscal() {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscal != nil {
		_ = embeddedFiscal.Close()
		embeddedFiscal = nil
	}
	embeddedFiscalURL = ""
}

func fiscalAdminBaseURL() string {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscalURL != "" {
		// Shell always uses loopback; LAN bind 0.0.0.0 is for remote Clients only.
		return loopbackAdminURL(embeddedFiscalURL)
	}
	return "http://127.0.0.1:17880"
}

// loopbackAdminURL is the ONLY mapper from listen URL → WebView/base URL (0.0.0.0 → 127.0.0.1).
func loopbackAdminURL(listenURL string) string {
	u := strings.TrimRight(strings.TrimSpace(listenURL), "/")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	host, port, err := net.SplitHostPort(u)
	if err != nil {
		return "http://127.0.0.1:17880"
	}
	host = strings.Trim(host, "[]")
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "17880"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// runFiscalStandalone runs only embedded fiscal (no cloud print loops) for M2 UAT on Mac.
// Invoked as: farvoo-fiscal-agent -fiscal-standalone
func runFiscalStandalone(args []string) {
	fs := flag.NewFlagSet("fiscal-standalone", flag.ExitOnError)
	_ = fs.Bool("fiscal-standalone", true, "run embedded fiscal only")
	cfgPath := fs.String("config", "", "optional config for station_printers / restaurant_id")
	_ = fs.Parse(args)

	path := strings.TrimSpace(*cfgPath)
	if path == "" {
		path = defaultConfigPath()
	} else {
		// Sole redirect so StationPrintersFn reloads the same file (not ~/.config).
		configPathOverride = path
		defer func() { configPathOverride = "" }()
	}

	var cfg *config
	if c, err := loadConfig(path); err == nil {
		cfg = c
	}

	if err := startEmbeddedFiscal(cfg); err != nil {
		log.Fatal(err)
	}
	defer stopEmbeddedFiscal()
	log.Printf("fiscal-standalone ready: %s/  (Ctrl+C to exit)", fiscalAdminBaseURL())

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func wantsFiscalStandalone(args []string) bool {
	for _, a := range args {
		if a == "-fiscal-standalone" || a == "--fiscal-standalone" {
			return true
		}
	}
	return false
}

// ensureFiscalStarted is called from agent run loops; ignores duplicate start.
func ensureFiscalStarted(ctx context.Context, sess *agentSession) {
	_ = ctx
	var cfg *config
	if sess != nil {
		cfg = sess.cfg
	}
	if err := startEmbeddedFiscal(cfg); err != nil {
		log.Printf("fiscal: embed failed: %v", err)
	}
}
