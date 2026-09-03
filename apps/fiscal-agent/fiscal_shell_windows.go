//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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

// openFiscalShellFromTray is the sole tray/IPC/desktop-open path for the fiscal shell.
// Starts embed if needed, waits until /local/v1/health is OK, then opens or focuses.
func (rt *trayRuntime) openFiscalShellFromTray() {
	var sess *agentSession
	if s, _, done := rt.snapshot(); done {
		sess = s
	}
	ensureFiscalStarted(rt.ctx, sess)
	base := fiscalAdminBaseURL()
	if err := waitFiscalHTTPReady(base); err != nil {
		log.Println("fiscal shell: not ready:", err)
		loc := loadAgentUILocale()
		messageBoxOK(uiT(loc, "about_title"), fmt.Sprintf("Fiscal UI not ready:\n%s", err.Error()))
		return
	}
	openFiscalShellWindow(base)
}

// waitFiscalHTTPReady is the sole pre-open gate: poll public health until OK or timeout.
func waitFiscalHTTPReady(baseURL string) error {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return fmt.Errorf("empty fiscal base URL")
	}
	url := base + "/local/v1/health"
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("HTTP %d", res.StatusCode)
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return fmt.Errorf("health %s: %w", url, last)
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
		if err := fiscalipc.RequestOpenFiscal(); err != nil {
			log.Println("fiscal:", err)
		}
		return
	}
	openFiscalOnTrayStart = true
	runAgent(nil)
}
