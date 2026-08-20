//go:build windows

package main

func loadTrayUILocale() string {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil {
		return "zh"
	}
	return cfg.uiLocale()
}
