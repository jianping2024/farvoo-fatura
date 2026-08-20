//go:build windows

package main

func setTrayUILocale(code string) error {
	path := defaultConfigPath()
	cfg, err := loadConfig(path)
	if err != nil {
		cfg = &config{}
	}
	cfg.UILocale = normalizeUILocale(code)
	return saveConfig(path, cfg)
}

func uiLocaleOptionTitle(menuLocale, option string) string {
	label := uiT(menuLocale, "menu_ui_locale_opt_"+option)
	if normalizeUILocale(menuLocale) == normalizeUILocale(option) {
		return "✓ " + label
	}
	return label
}

func uiLocaleOptionLogLabel(code string) string {
	return uiT(normalizeUILocale(code), "menu_ui_locale_opt_"+normalizeUILocale(code))
}
