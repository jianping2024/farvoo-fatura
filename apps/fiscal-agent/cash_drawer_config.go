package main

import fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"

// loadAgentCashDrawerPin / setAgentCashDrawerPin are the ONLY config.json cash_drawer_pin
// load/save entrypoints for embedded Fiscal Admin (defaultConfigPath).
// fiscal-local (no callbacks) uses print.PinPrefsFile — not config.json.

func loadAgentCashDrawerPin() int {
	c, err := loadConfig(defaultConfigPath())
	if err != nil || c == nil {
		return 2
	}
	return c.cashDrawerPin()
}

// applyCashDrawerPinToConfig is the ONLY mutation of config.CashDrawerPin (normalize + assign).
func applyCashDrawerPinToConfig(c *config, pin int) {
	if c == nil {
		return
	}
	c.CashDrawerPin = fiscalprint.NormalizeCashDrawerPin(pin)
}

func setAgentCashDrawerPin(pin int) error {
	path := defaultConfigPath()
	c, err := loadConfig(path)
	if err != nil {
		c = &config{}
	}
	applyCashDrawerPinToConfig(c, pin)
	return saveConfig(path, c)
}

func (c *config) cashDrawerPin() int {
	if c == nil {
		return 2
	}
	return fiscalprint.NormalizeCashDrawerPin(c.CashDrawerPin)
}
