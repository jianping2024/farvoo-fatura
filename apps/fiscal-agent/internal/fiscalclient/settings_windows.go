//go:build windows

package fiscalclient

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"sync"
	"text/template"

	"farvoo-fiscal-agent/internal/fiscalwebview"
)

//go:embed settings.html
var settingsHTMLTemplate string

// RunSettings opens the Agent connection settings UI. Returns agent_base or "" if cancelled.
// WebView2 lifecycle: ONLY fiscalwebview.RunHTMLWindow (UI thread).
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

	finish := func(base string) (bool, error) {
		mu.Lock()
		result = base
		mu.Unlock()
		return true, nil
	}

	bind := map[string]interface{}{
		"testConnection": func(h, p string) (string, error) {
			base, err := NormalizeAgentBase(h, p)
			if err != nil {
				return "", err
			}
			if err := ProbeHealth(base); err != nil {
				return "", err
			}
			return "连接成功", nil
		},
		"saveSettings": func(h, p string) (string, error) {
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
		},
		"closeSettings": finish,
	}

	if err := fiscalwebview.RunHTMLWindow(fiscalwebview.HTMLWindowOptions{
		Title:    "Farvoo 开票 · 设置",
		HTML:     buf.String(),
		DataPath: dataPath,
		Width:    520,
		Height:   420,
		Bind:     bind,
	}); err != nil {
		return "", err
	}

	mu.Lock()
	defer mu.Unlock()
	return result, nil
}
