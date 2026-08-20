//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maybeNotifyVersionUpgrade shows at most one dialog per local day when Mesa recommends a newer build.
func maybeNotifyVersionUpgrade(recommended string) {
	recommended = strings.TrimSpace(recommended)
	if recommended == "" || !agentVersionOlderThan(Version, recommended) {
		return
	}
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil || strings.TrimSpace(cfg.AgentJWT) == "" {
		return
	}
	now := time.Now()
	marker := filepath.Join(agentDataDir(), ".version_reminder_"+now.Format("20060102"))
	if _, err := os.Stat(marker); err == nil {
		return
	}
	_ = os.MkdirAll(agentDataDir(), 0o700)
	_ = os.WriteFile(marker, []byte("1"), 0o644)

	loc := cfg.uiLocale()
	messageBoxOK(
		uiT(loc, "version_toast_title"),
		fmt.Sprintf(uiT(loc, "version_toast_body"), Version, recommended),
	)
}
