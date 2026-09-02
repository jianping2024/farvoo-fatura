package main

import "farvoo-fiscal-agent/internal/fiscal/locale"

// loadAgentUILocale / setAgentUILocale are the ONLY config.json ui_locale accessors
// shared by tray, wizards, and embedded Fiscal Admin.

func loadAgentUILocale() string {
	c, err := loadConfig(defaultConfigPath())
	if err != nil || c == nil {
		return "zh"
	}
	return c.uiLocale()
}

func setAgentUILocale(code string) error {
	path := defaultConfigPath()
	c, err := loadConfig(path)
	if err != nil {
		c = &config{}
	}
	c.UILocale = locale.NormalizeUILocale(code)
	return saveConfig(path, c)
}
