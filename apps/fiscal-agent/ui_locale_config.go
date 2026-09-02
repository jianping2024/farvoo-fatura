package main

import "farvoo-fiscal-agent/internal/fiscal/locale"

// loadAgentUILocale / setAgentUILocale are the ONLY config.json ui_locale load/save
// entrypoints for tray + embedded Fiscal Admin (defaultConfigPath).
//
// Dual HTTP surfaces (do not merge):
//   - Admin GET|PUT /local/v1/setup/ui-locale → UILocaleGet/Set → these helpers
//   - Wizard POST /api/ui-locale → applyUILocaleToConfig + saveConfig(wizard path);
//     wizard may also set text_encoding in the same save.
// fiscal-local (no callbacks) uses locale.PrefsFile — not config.json.

func loadAgentUILocale() string {
	c, err := loadConfig(defaultConfigPath())
	if err != nil || c == nil {
		return "zh"
	}
	return c.uiLocale()
}

// applyUILocaleToConfig is the ONLY mutation of config.UILocale (normalize + assign).
func applyUILocaleToConfig(c *config, code string) {
	if c == nil {
		return
	}
	c.UILocale = locale.NormalizeUILocale(code)
}

func setAgentUILocale(code string) error {
	path := defaultConfigPath()
	c, err := loadConfig(path)
	if err != nil {
		c = &config{}
	}
	applyUILocaleToConfig(c, code)
	return saveConfig(path, c)
}
