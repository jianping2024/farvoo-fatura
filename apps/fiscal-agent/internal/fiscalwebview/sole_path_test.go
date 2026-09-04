package fiscalwebview_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestWebViewSoleConstructionPath ensures WebView2 is only created inside fiscalwebview (0.4.67 UI thread).
func TestWebViewSoleConstructionPath(t *testing.T) {
	root := moduleRoot(t)
	agent := filepath.Join(root, "apps", "fiscal-agent")

	forbidden := []string{
		filepath.Join(agent, "fiscal_shell_windows.go"),
		filepath.Join(agent, "agent_entry_windows.go"),
	}
	for _, p := range forbidden {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		s := string(b)
		if strings.Contains(s, "go-webview2") || strings.Contains(s, "webview2.New") {
			t.Fatalf("%s must not construct WebView2 directly; use fiscalwebview.RequestOpen only", filepath.Base(p))
		}
	}

	openWin := filepath.Join(agent, "internal", "fiscalwebview", "open_windows.go")
	b, err := os.ReadFile(openWin)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Count(s, "func newWebView(") != 1 {
		t.Fatalf("open_windows.go must define newWebView exactly once, got %d", strings.Count(s, "func newWebView("))
	}
	if !strings.Contains(s, "takeExistingShellHWND") {
		t.Fatal("open_windows.go must use takeExistingShellHWND for already-open shell")
	}
	if !strings.Contains(s, "rememberShellHWND") || !strings.Contains(s, "clearShellHWND") {
		t.Fatal("open_windows.go must use rememberShellHWND + clearShellHWND")
	}
	if strings.Contains(s, "func trackHWND") {
		t.Fatal("trackHWND removed — defer-on-return cleared activeHWND while Run still alive")
	}
	// clearShellHWND must be deferred around Run, not inside rememberShellHWND
	rememberIdx := strings.Index(s, "func rememberShellHWND")
	clearFnIdx := strings.Index(s, "func clearShellHWND")
	if rememberIdx < 0 || clearFnIdx < 0 {
		t.Fatal("missing remember/clear helpers")
	}
	rememberBody := s[rememberIdx:clearFnIdx]
	if strings.Contains(rememberBody, "defer ") {
		t.Fatal("rememberShellHWND must not defer-clear HWND (bug: HWND gone while window still open)")
	}
	if !strings.Contains(s, "defer clearShellHWND()") {
		t.Fatal("runWindowOnThread must defer clearShellHWND around Run()")
	}
	if strings.Contains(s, "hwndResponsive(") {
		t.Fatal("open_windows.go must not call hwndResponsive (breaks minimized restore)")
	}
	if !strings.Contains(s, "startUIThread") {
		t.Fatal("open_windows.go must route through startUIThread")
	}
	if strings.Contains(s, "go func()") && strings.Contains(s, "RunWindow(opts)") {
		t.Fatal("open_windows.go must not spawn RunWindow on a pool goroutine")
	}

	focusFile := filepath.Join(agent, "internal", "fiscalwebview", "focus_hwnd_windows.go")
	fb, err := os.ReadFile(focusFile)
	if err != nil {
		t.Fatal(err)
	}
	fs := string(fb)
	if strings.Count(fs, "func focusHWND(") != 1 {
		t.Fatal("focusHWND must be defined exactly once in focus_hwnd_windows.go")
	}
	if !strings.Contains(fs, "scRestore") || !strings.Contains(fs, "IsIconic") {
		t.Fatal("focusHWND must restore minimized via IsIconic + SC_RESTORE")
	}
	if !strings.Contains(fs, "GetAncestor") && !strings.Contains(fs, "gaRoot") {
		t.Fatal("focusHWND must resolve top-level HWND (GetAncestor/gaRoot)")
	}

	// No second focusHWND / ShowWindow restore stacks outside focus_hwnd_windows.go
	for _, name := range []string{"open_windows.go", "focus_title_windows.go", "fiscal_shell_windows.go", "agent_entry_windows.go"} {
		p := filepath.Join(agent, "internal", "fiscalwebview", name)
		if name == "fiscal_shell_windows.go" || name == "agent_entry_windows.go" {
			p = filepath.Join(agent, name)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if strings.Contains(body, "func focusHWND(") {
			t.Fatalf("%s must not redefine focusHWND", name)
		}
	}
	if _, err := os.Stat(filepath.Join(agent, "internal", "fiscalwebview", "hwnd_windows.go")); err == nil {
		t.Fatal("hwnd_windows.go removed — hwndResponsive must not gate shell restore")
	}
	uiThread := filepath.Join(agent, "internal", "fiscalwebview", "ui_thread_windows.go")
	if _, err := os.Stat(uiThread); err != nil {
		t.Fatalf("missing ui_thread_windows.go: %v", err)
	}

	settings := filepath.Join(agent, "internal", "fiscalclient", "settings_windows.go")
	sb, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	ss := string(sb)
	if strings.Contains(ss, "go-webview2") || strings.Contains(ss, "webview2.New") {
		t.Fatal("settings_windows.go must use fiscalwebview.RunHTMLWindow only")
	}
	if !strings.Contains(ss, "finishSettings :=") {
		t.Fatal("settings must define finishSettings as ONLY result+close path")
	}
	if strings.Count(ss, "closeDialog()") < 1 {
		t.Fatal("settings finishSettings must call closeDialog()")
	}
	if strings.Contains(ss, "(bool, error)") {
		t.Fatal("settings must not use (bool, error) as fake close signal")
	}
	if !strings.Contains(ss, "Bind: func(closeDialog func())") {
		t.Fatal("settings must use HTMLWindowOptions.Bind(closeDialog) factory")
	}

	html := filepath.Join(agent, "internal", "fiscalclient", "settings.html")
	hb, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	hs := string(hb)
	if strings.Contains(hs, "closeSettings(base)") || strings.Contains(hs, "await closeSettings") {
		t.Fatal("settings.html save must not double-call closeSettings after saveSettings")
	}
	if !strings.Contains(hs, "await saveSettings(") {
		t.Fatal("settings.html save must call saveSettings")
	}
	if !strings.Contains(hs, "closeSettings('')") {
		t.Fatal("settings.html cancel must call closeSettings('')")
	}

	if !strings.Contains(s, "closeHTMLDialog") {
		t.Fatal("open_windows.go must define closeHTMLDialog as ONLY Bind-side HTML exit")
	}
	if strings.Count(s, "wv.Terminate()") != 1 {
		t.Fatalf("HTML dialog Terminate must appear exactly once in open_windows.go, got %d", strings.Count(s, "wv.Terminate()"))
	}
	if !strings.Contains(s, "opts.Bind(closeHTMLDialog)") {
		t.Fatal("runHTMLWindowOnThread must pass closeHTMLDialog into Bind factory")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
