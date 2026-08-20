package main

import (
	"context"
	"strings"
	"time"
)

// waitForAgentPairing blocks until config has AgentJWT.
// When tray local HTTP is already up (ConfigureWizardPort), open /pair there — one surface with Dashboard.
// Otherwise (console / CLI / non-Windows) use the standalone pairing wizard.
func waitForAgentPairing(ctx context.Context, configPath, prefillAPI string) error {
	if addr := trayLocal.listenAddr(); addr != "" {
		return waitPairingOnListenAddr(ctx, configPath, prefillAPI, addr)
	}
	return runPairingWizard(ctx, configPath, prefillAPI)
}

func waitPairingOnListenAddr(ctx context.Context, configPath, prefillAPI, listenAddr string) error {
	baseURL := pairWizardBaseURL(listenAddr, prefillAPI)
	agentLogLocale(localeFromConfigPath(configPath), "log_wizard_open", baseURL)
	announceWizardURL(productName+" 配对", baseURL)

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		cfg, err := loadConfig(configPath)
		if err == nil && cfg != nil && strings.TrimSpace(cfg.AgentJWT) != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
