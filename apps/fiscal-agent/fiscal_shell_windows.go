//go:build windows

package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"syscall"

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
		loc := loadAgentUILocale()
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

// allowSetForegroundAny lets the running Agent call SetForegroundWindow after this
// user-launched fiscal shortcut process hands off via IPC (best-effort).
func allowSetForegroundAny() {
	const asfwAny = ^uint32(0)
	user32 := syscall.NewLazyDLL("user32.dll")
	allow := user32.NewProc("AllowSetForegroundWindow")
	_, _, _ = allow.Call(uintptr(asfwAny))
}

// runFiscalCommand is the sole desktop "Farvoo 开票" entry (FarvooFiscalAgent fiscal).
// Never starts a second tray agent: open via IPC if mutex held, else become the sole agent.
func runFiscalCommand() {
	allowSetForegroundAny()
	if fiscalipc.AgentInstanceRunning() {
		if err := fiscalipc.RequestOpenFiscal(); err != nil {
			log.Println("fiscal:", err)
		}
		return
	}
	if !acquireAgentSingleInstance() {
		// Lost race: another agent appeared between check and CreateMutex.
		if err := fiscalipc.RequestOpenFiscal(); err != nil {
			log.Println("fiscal:", err)
		}
		return
	}
	openFiscalOnTrayStart = true
	runAgent(nil)
}
