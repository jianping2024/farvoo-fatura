package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSoleFiscalShellOpenPath locks: health-before-open + HWND lifetime + one open entry.
func TestSoleFiscalShellOpenPath(t *testing.T) {
	agent := filepath.Join(moduleRoot(t), "apps", "fiscal-agent")

	shell, err := os.ReadFile(filepath.Join(agent, "fiscal_shell_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	ss := string(shell)
	if strings.Count(ss, "func waitFiscalHTTPReady(") != 1 {
		t.Fatal("waitFiscalHTTPReady must be defined exactly once")
	}
	if strings.Count(ss, "func (rt *trayRuntime) openFiscalShellFromTray(") != 1 {
		t.Fatal("openFiscalShellFromTray must be defined exactly once")
	}
	openBody := ss[strings.Index(ss, "func (rt *trayRuntime) openFiscalShellFromTray("):]
	openBody = openBody[:strings.Index(openBody, "\nfunc ")]
	if !strings.Contains(openBody, "waitFiscalHTTPReady") || !strings.Contains(openBody, "openFiscalShellWindow") {
		t.Fatal("openFiscalShellFromTray must waitFiscalHTTPReady then openFiscalShellWindow")
	}
	healthIdx := strings.Index(openBody, "waitFiscalHTTPReady")
	openIdx := strings.Index(openBody, "openFiscalShellWindow")
	if healthIdx < 0 || openIdx < 0 || healthIdx > openIdx {
		t.Fatal("health gate must run before openFiscalShellWindow")
	}
	if strings.Count(ss, "ensureFiscalStarted") != 1 {
		t.Fatal("open path should call ensureFiscalStarted exactly once in fiscal_shell_windows.go")
	}

	entry, err := os.ReadFile(filepath.Join(agent, "agent_entry_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	es := string(entry)
	tray := es[strings.Index(es, "func runAgentTrayFirst"):]
	if i := strings.Index(tray, "\nfunc onTrayReady"); i > 0 {
		tray = tray[:i]
	}
	connIdx := strings.Index(tray, "Connected — accepting print jobs")
	if connIdx < 0 {
		t.Fatal("missing Connected log anchor in runAgentTrayFirst")
	}
	tail := tray[connIdx:]
	if i := strings.Index(tail, "\n\t}()"); i > 0 {
		tail = tail[:i]
	}
	if strings.Contains(tail, "openFiscalShellFromTray") {
		t.Fatal("must not open fiscal shell in the Connected block; cold open is openFiscalOnTrayStart before init")
	}
	initIdx := strings.Index(tray, "initAgentSession")
	flagOpenIdx := strings.Index(tray, "if openFiscalOnTrayStart")
	if flagOpenIdx < 0 || initIdx < 0 || flagOpenIdx > initIdx {
		t.Fatal("openFiscalOnTrayStart open must run before initAgentSession (not wait for Mesa)")
	}
	if !strings.Contains(tray, "openFiscalShellFromTray()") {
		t.Fatal("cold fiscal start must call openFiscalShellFromTray")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
