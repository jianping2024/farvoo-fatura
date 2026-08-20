package main

import (
	"context"
	"flag"
	"strings"
	"time"
)

type agentSession struct {
	cfgPath string
	cfg     *config
	pc      *pollController
	hb      heartbeatSnapshot
}

func initAgentSession(runCtx context.Context, args []string) (*agentSession, bool, error) {
	if runCtx == nil {
		runCtx = context.Background()
	}
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	apiBase := fs.String("api", "http://127.0.0.1:3000", productName+" base URL")
	code := fs.String("code", "", "6-digit pairing code (first run)")
	defaultPrinter := fs.String("default-printer", "", "Default host:port for receipts / pre-bill (also sets legacy printer_host)")
	cfgPath := fs.String("config", "", "Config file path")
	showConsole := fs.Bool("console", false, "Keep console window visible (Windows debug)")
	_ = fs.Parse(args)

	path := *cfgPath
	if path == "" {
		path = defaultConfigPath()
	}

	cfg, err := loadConfig(path)
	if err != nil || cfg.AgentJWT == "" {
		if *code != "" {
			deviceID := deviceIDForPairing(path)
			cfg, err = claim(*apiBase, strings.TrimSpace(*code), deviceID)
			if err != nil {
				return nil, *showConsole, err
			}
			if dp := strings.TrimSpace(*defaultPrinter); dp != "" {
				cfg.DefaultPrinter = dp
				cfg.PrinterHost = dp
			}
			if err := saveConfig(path, cfg); err != nil {
				return nil, *showConsole, err
			}
			agentLogLocale(localeFromConfigPath(path), "log_saved_config", path)
		} else {
			prefill := strings.TrimSpace(*apiBase)
			if prefill == "http://127.0.0.1:3000" {
				prefill = ""
			}
			// Tray local HTTP (17892) should already be up; otherwise standalone 17890 wizard.
			agentLogLocale(localeFromConfigPath(path), "log_pairing_required")
			ctx, cancel := context.WithTimeout(runCtx, 20*time.Minute)
			defer cancel()
			if err := waitForAgentPairing(ctx, path, prefill); err != nil {
				if runCtx.Err() != nil {
					return nil, *showConsole, runCtx.Err()
				}
				return nil, *showConsole, err
			}
			cfg, err = loadConfig(path)
			if err != nil {
				return nil, *showConsole, err
			}
			if cfg.AgentJWT == "" {
				return nil, *showConsole, errPairingIncomplete
			}
		}
	}
	if cfg.APIBase == "" {
		cfg.APIBase = *apiBase
	}
	persistAlignedSupabaseURL(path, cfg)
	if dp := strings.TrimSpace(*defaultPrinter); dp != "" {
		cfg.DefaultPrinter = dp
		cfg.PrinterHost = dp
		_ = saveConfig(path, cfg)
	}

	agentLog(cfg, "log_bootstrap_runtime_config")
	t0 := time.Now()
	applyCloudRuntimeConfig(cfg, cfg.APIBase)
	agentLog(cfg, "log_bootstrap_phase_done", "runtime-config", time.Since(t0).Round(time.Millisecond))

	if !cfg.hasPrinterRouting() {
		agentLogLocale(localeFromConfigPath(path), "log_setup_wizard")
		setupCtx, setupCancel := context.WithTimeout(runCtx, 30*time.Minute)
		defer setupCancel()
		if err := runSetupWizard(setupCtx, path, cfg); err != nil {
			if runCtx.Err() != nil {
				return nil, *showConsole, runCtx.Err()
			}
			return nil, *showConsole, err
		}
		cfg, err = loadConfig(path)
		if err != nil {
			return nil, *showConsole, err
		}
	}
	if !cfg.hasPrinterRouting() {
		agentLog(cfg, "log_no_station_mapping")
	}

	agentLog(cfg, "log_bootstrap_routing_sync")
	t1 := time.Now()
	syncRoutingToCloud(cfg, false)
	agentLog(cfg, "log_bootstrap_phase_done", "routing-sync", time.Since(t1).Round(time.Millisecond))

	stationCount := 0
	if cfg.StationPrinters != nil {
		stationCount = len(cfg.StationPrinters)
	}
	pc, err := newPollController(cfg.Schedule, cfg.Poll)
	if err != nil {
		return nil, *showConsole, err
	}
	agentLog(cfg, "log_agent_startup", cfg.APIBase, stationCount)
	logAgentStartup(cfg, cfg.APIBase, stationCount)

	return &agentSession{cfgPath: path, cfg: cfg, pc: pc}, *showConsole, nil
}

var errPairingIncomplete = errString("pairing did not save configuration")

type errString string

func (e errString) Error() string { return string(e) }
