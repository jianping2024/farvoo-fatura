//go:build windows

package main

import (
	"fmt"
	"path/filepath"
)

// maybeNotifyTrayReady shows a dialog every time the tray starts so the user knows launch succeeded.
func maybeNotifyTrayReady() {
	logDir := filepath.Join(agentDataDir(), "agent.log")
	cfg, err := loadConfig(defaultConfigPath())
	loc := loadTrayUILocale()
	bodyKey := "tray_startup_body_unpaired"
	if err == nil && cfg != nil && cfg.AgentJWT != "" {
		loc = cfg.uiLocale()
		bodyKey = "tray_startup_body"
	}
	messageBoxOK(
		uiT(loc, "tray_startup_title"),
		fmt.Sprintf(uiT(loc, bodyKey), logDir),
	)
}
