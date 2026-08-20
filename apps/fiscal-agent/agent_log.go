package main

import (
	"fmt"
	"log"
)

func localeForLog(cfg *config) string {
	if cfg != nil {
		return cfg.uiLocale()
	}
	if c, err := loadConfig(defaultConfigPath()); err == nil {
		return c.uiLocale()
	}
	return "zh"
}

func localeFromConfigPath(path string) string {
	if c, err := loadConfig(path); err == nil {
		return c.uiLocale()
	}
	return "zh"
}

// agentLog writes a user-facing line to agent.log (follows ui_locale).
func agentLog(cfg *config, key string, args ...any) {
	loc := localeForLog(cfg)
	msg := uiT(loc, key)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	log.Println(msg)
}

// agentLogLocale is used when config is not loaded yet (e.g. startup).
func agentLogLocale(loc, key string, args ...any) {
	loc = normalizeUILocale(loc)
	msg := uiT(loc, key)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	log.Println(msg)
}

// agentLogTech appends a short technical detail in parentheses for support staff.
func agentLogTech(cfg *config, key, tech string, args ...any) {
	loc := localeForLog(cfg)
	msg := uiT(loc, key)
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	if tech != "" {
		msg += " (" + tech + ")"
	}
	log.Println(msg)
}
