//go:build windows

package main

import (
	"log"
)

// startTrayConfigureWizard opens the local printer settings page in the default browser.
// launchQuery is optional (?api=…&code=…) from a dashboard deep link.
func (rt *trayRuntime) startTrayConfigureWizard(launchQuery string) {
	if rt == nil {
		return
	}
	path := defaultConfigPath()
	prefill := ""
	if loaded, err := loadConfig(path); err == nil && loaded.APIBase != "" {
		prefill = loaded.APIBase
	}

	addr := trayLocal.listenAddr()
	if addr == "" {
		// Should not happen after startTrayLocalHTTP in runAgentTrayFirst; port bind failed.
		log.Println("tray: configure: local http not ready")
		loc := rt.uiLocale()
		messageBoxOK(uiT(loc, "about_title"), uiT(loc, "pair_service_unreachable"))
		return
	}

	baseURL := configureWizardBaseURL(addr, prefill, launchQuery)
	if onConfigureWizardReady != nil {
		onConfigureWizardReady(baseURL)
	}
	announceWizardURL(productName+" 打印机设置", baseURL)
}

func (rt *trayRuntime) rememberConfigureWizardURL(url string) {
	if rt == nil || url == "" {
		return
	}
	rt.mu.Lock()
	rt.configureURL = url
	rt.mu.Unlock()
}
