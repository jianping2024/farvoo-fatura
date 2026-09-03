//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func agentLogPath() string {
	return filepath.Join(agentDataDir(), "agent.log")
}

func openAgentLog() error {
	path := agentLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
}

func openAgentLogFolder() error {
	dir := agentDataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return exec.Command("explorer", dir).Start()
}

func confirmTrayExit(locale string) bool {
	return messageBoxYesNo(
		uiT(locale, "exit_confirm_title"),
		uiT(locale, "exit_confirm_body"),
	)
}

func confirmTrayRestart(locale string) bool {
	return messageBoxYesNo(
		uiT(locale, "restart_confirm_title"),
		uiT(locale, "restart_confirm_body"),
	)
}

func trayAboutText(rt *trayRuntime, locale string) string {
	text := uiT(locale, "about_title") + " " + Version + "\n\n" + uiT(locale, "about_config") + "\n" + defaultConfigPath()
	text += "\n\n" + uiT(locale, "about_log") + "\n" + agentLogPath()
	cfg, _ := loadConfig(defaultConfigPath())
	if cfg == nil {
		if sess, _, done := rt.snapshot(); done && sess != nil && sess.cfg != nil {
			cfg = sess.cfg
		}
	}
	if cfg != nil {
		if line := cfg.credentialAboutLine(locale, time.Now()); line != "" {
			text += "\n\n" + line
		}
		if strings.TrimSpace(cfg.APIBase) != "" {
			text += "\n\n" + uiT(locale, "about_mesa") + " " + cfg.APIBase
		}
	}
	return text
}

func applyTrayUILocaleSubmenu(mUILang, mLangZh, mLangEn, mLangPt interface {
	SetTitle(string)
	SetTooltip(string)
}, locale string) {
	mUILang.SetTitle(uiT(locale, "menu_ui_locale"))
	mUILang.SetTooltip(uiT(locale, "menu_ui_locale_tip"))
	mLangZh.SetTitle(uiLocaleOptionTitle(locale, "zh"))
	mLangEn.SetTitle(uiLocaleOptionTitle(locale, "en"))
	mLangPt.SetTitle(uiLocaleOptionTitle(locale, "pt"))
}

func applyTrayMenuLabels(mStatus, mSettings, mFiscal, mOpenLog, mOpenLogDir, mAbout, mRestart, mQuit interface {
	SetTitle(string)
	SetTooltip(string)
}, locale string) {
	mStatus.SetTitle(uiT(locale, "menu_status_prefix") + "…")
	mSettings.SetTitle(uiT(locale, "menu_settings"))
	mSettings.SetTooltip(uiT(locale, "menu_settings_tip"))
	mFiscal.SetTitle(uiT(locale, "menu_fiscal"))
	mFiscal.SetTooltip(uiT(locale, "menu_fiscal_tip"))
	mOpenLog.SetTitle(uiT(locale, "menu_open_log"))
	mOpenLog.SetTooltip(uiT(locale, "menu_open_log_tip"))
	mOpenLogDir.SetTitle(uiT(locale, "menu_open_log_dir"))
	mOpenLogDir.SetTooltip(uiT(locale, "menu_open_log_dir_tip"))
	mAbout.SetTitle(uiT(locale, "menu_about"))
	mAbout.SetTooltip(uiT(locale, "menu_about_tip"))
	mRestart.SetTitle(uiT(locale, "menu_restart"))
	mRestart.SetTooltip(uiT(locale, "menu_restart_tip"))
	mQuit.SetTitle(uiT(locale, "menu_quit"))
	mQuit.SetTooltip(uiT(locale, "menu_quit_tip"))
}

func confirmTrayUninstall(locale string) bool {
	return messageBoxYesNo(
		uiT(locale, "uninstall_confirm_title"),
		uiT(locale, "uninstall_confirm_body"),
	)
}
