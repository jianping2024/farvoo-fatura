//go:build windows

package fiscalclient

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"text/template"

	"farvoo-fiscal-agent/internal/fiscalwebview"

	"github.com/jchv/go-webview2"
)

//go:embed settings.html
var settingsHTMLTemplate string

// RunSettings opens the Agent connection settings UI. Returns agent_base or "" if cancelled.
func RunSettings(host, port string) (string, error) {
	dataPath, err := WebViewDataDir()
	if err != nil {
		return "", err
	}
	if port == "" {
		port = "17880"
	}

	var (
		mu     sync.Mutex
		result string
		wv     webview2.WebView
	)

	tmpl, err := template.New("settings").Parse(settingsHTMLTemplate)
	if err != nil {
		return "", err
	}
	hostJSON, _ := json.Marshal(host)
	portJSON, _ := json.Marshal(port)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"HostJSON": string(hostJSON),
		"PortJSON": string(portJSON),
	}); err != nil {
		return "", err
	}

	wv = webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  dataPath,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Farvoo 开票 · 设置",
			Width:  520,
			Height: 420,
			IconId: fiscalwebview.WindowIconID,
			Center: true,
		},
	})
	if wv == nil {
		return "", fmt.Errorf("fiscal client: failed to create WebView2 — install Microsoft Edge WebView2 Runtime")
	}

	finish := func(base string) (bool, error) {
		mu.Lock()
		result = base
		mu.Unlock()
		wv.Destroy()
		return true, nil
	}

	_ = wv.Bind("testConnection", func(h, p string) (string, error) {
		base, err := NormalizeAgentBase(h, p)
		if err != nil {
			return "", err
		}
		if err := ProbeHealth(base); err != nil {
			return "", err
		}
		return "连接成功", nil
	})
	_ = wv.Bind("saveSettings", func(h, p string) (string, error) {
		base, err := NormalizeAgentBase(h, p)
		if err != nil {
			return "", err
		}
		if err := ProbeHealth(base); err != nil {
			return "", err
		}
		if err := SaveConfig(Config{AgentBase: base}); err != nil {
			return "", err
		}
		_, _ = finish(base)
		return base, nil
	})
	_ = wv.Bind("closeSettings", finish)

	wv.SetHtml(buf.String())
	wv.Run()

	mu.Lock()
	defer mu.Unlock()
	return result, nil
}
