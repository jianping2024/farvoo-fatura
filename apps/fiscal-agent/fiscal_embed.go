package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
)

// embeddedFiscal is the single Agent-side fiscal runtime (tray / run / fiscal-standalone).
var (
	embeddedFiscalMu sync.Mutex
	embeddedFiscal   *bootstrap.Runtime
	embeddedFiscalURL string
)

// startEmbeddedFiscal starts Fiscal Core inside the Agent process — ONLY agent embed entry.
func startEmbeddedFiscal(cfg *config) error {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscal != nil {
		return nil
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
	bind := strings.TrimSpace(os.Getenv("FISCAL_BIND"))
	if bind == "" {
		bind = "127.0.0.1:17880"
	}
	storeID := "store-demo-001"
	if cfg != nil && strings.TrimSpace(cfg.RestaurantID) != "" {
		storeID = strings.TrimSpace(cfg.RestaurantID)
	}
	if v := strings.TrimSpace(os.Getenv("FISCAL_STORE_ID")); v != "" {
		storeID = v
	}

	var stations map[string]string
	if cfg != nil {
		stations = cfg.StationPrinters
	}

	opts := bootstrap.Options{
		DBPath:          dbPath,
		DataDir:         dataDir,
		BindAddr:        bind,
		StoreID:         storeID,
		StationPrinters: stations,
		Seed:            os.Getenv("FISCAL_SEED") == "1",
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
	embeddedFiscal = rt
	embeddedFiscalURL = "http://" + bind
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
}

func fiscalAdminBaseURL() string {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscalURL != "" {
		return embeddedFiscalURL
	}
	return "http://127.0.0.1:17880"
}

// runFiscalStandalone runs only embedded fiscal (no cloud print loops) for M2 UAT on Mac.
// Invoked as: farvoo-fiscal-agent -fiscal-standalone
func runFiscalStandalone(args []string) {
	fs := flag.NewFlagSet("fiscal-standalone", flag.ExitOnError)
	_ = fs.Bool("fiscal-standalone", true, "run embedded fiscal only")
	cfgPath := fs.String("config", "", "optional config for station_printers / restaurant_id")
	_ = fs.Parse(args)

	var cfg *config
	path := *cfgPath
	if path == "" {
		path = defaultConfigPath()
	}
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
