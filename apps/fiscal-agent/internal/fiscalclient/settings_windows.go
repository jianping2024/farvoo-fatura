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
// Dialog exit: ONLY closeDialog from Bind (Terminate) — never pretend return-true closes.
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

	if err := fiscalwebview.RunHTMLWindow(fiscalwebview.HTMLWindowOptions{
		Title:    "Farvoo 开票 · 设置",
		HTML:     buf.String(),
		DataPath: dataPath,
		Width:    520,
		Height:   420,
		Bind: func(closeDialog func()) map[string]interface{} {
			// finishSettings is the ONLY settings result+close path (save and cancel).
			finishSettings := func(base string) error {
				mu.Lock()
				result = base
				mu.Unlock()
				closeDialog()
				return nil
			}
			return map[string]interface{}{
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
					if err := finishSettings(base); err != nil {
						return "", err
					}
					return base, nil
				},
				"closeSettings": finishSettings,
			}
		},
	}); err != nil {
		return "", err
	}

	mu.Lock()
	defer mu.Unlock()
	return result, nil
}
