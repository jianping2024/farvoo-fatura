//go:build windows

package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"farvoo-fiscal-agent/internal/fiscalipc"
	"farvoo-fiscal-agent/internal/fiscalwebview"
)

var openFiscalOnTrayStart bool

func agentFiscalWebViewDataDir() string {
	return filepath.Join(agentDataDir(), "fiscal-webview")
}

func openFiscalShellWindow(baseURL string) {
	url := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/"
	if err := fiscalwebview.RequestOpen(fiscalwebview.Options{
		URL:      url,
		DataPath: agentFiscalWebViewDataDir(),
	}); err != nil {
		log.Println("fiscal shell:", err)
		loc := loadTrayUILocale()
		messageBoxOK(uiT(loc, "about_title"), err.Error())
	}
}

func (rt *trayRuntime) openFiscalShellFromTray() {
	ensureFiscalStarted(rt.ctx, nil)
	if sess, _, done := rt.snapshot(); done {
		ensureFiscalStarted(rt.ctx, sess)
	}
	openFiscalShellWindow(fiscalAdminBaseURL())
}

func startFiscalIPC(ctx context.Context, open func()) {
	fiscalipc.ServeAgentCommands(ctx, open)
}

func runFiscalCommand() {
	if fiscalipc.AgentInstanceRunning() {
		if err := fiscalipc.RequestOpenFiscal(); err != nil {
			log.Fatal(err)
		}
		return
	}
	guardMainAgentSingleInstance()
	openFiscalOnTrayStart = true
	runAgent(nil)
}
