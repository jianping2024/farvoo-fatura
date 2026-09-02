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
	if !strings.Contains(s, "startUIThread") {
		t.Fatal("open_windows.go must route through startUIThread")
	}
	if strings.Contains(s, "go func()") && strings.Contains(s, "RunWindow(opts)") {
		t.Fatal("open_windows.go must not spawn RunWindow on a pool goroutine")
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
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
