//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maybeNotifyCredentialRenewal shows at most one reminder per local day when within the renewal window.
func maybeNotifyCredentialRenewal() {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil || strings.TrimSpace(cfg.AgentJWT) == "" {
		return
	}
	now := time.Now()
	if !cfg.credentialInReminderWindow(now) {
		return
	}
	days, _ := cfg.credentialDaysRemaining(now)
	if days <= 0 {
		return
	}
	marker := filepath.Join(agentDataDir(), ".credential_reminder_"+now.Format("20060102"))
	if _, err := os.Stat(marker); err == nil {
		return
	}
	_ = os.MkdirAll(agentDataDir(), 0o700)
	_ = os.WriteFile(marker, []byte("1"), 0o644)

	loc := cfg.uiLocale()
	until, _ := cfg.validUntilTime()
	dateStr := until.Local().Format("2006-01-02")
	messageBoxOK(
		uiT(loc, "credential_toast_title"),
		fmt.Sprintf(uiT(loc, "credential_toast_body"), dateStr, days),
	)
}
