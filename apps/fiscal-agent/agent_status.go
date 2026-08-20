package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// trayLevel drives tray icon color on Windows.
type trayLevel int

const (
	trayLevelGreen trayLevel = iota
	trayLevelYellow
	trayLevelRed
)

// agentStatus is shared between the poll loop and the system tray (Windows).
type agentStatus struct {
	mu             sync.RWMutex
	summary        string
	detail         string
	mode           NotificationMode // actual running notifier mode
	scheduleClosed bool             // when true, operational statuses cannot overwrite yellow

	// realtimeFallback: preferred realtime but active notifier is polling (startup fallback).
	realtimeFallback bool
	fallbackPrintOK  bool
	// promoteRestart: sole auto restore path — tray process restart (menu Restart, no confirm).
	promoteRestart func()
}

func (s *agentStatus) set(summary, detail string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scheduleClosed {
		switch strings.TrimSpace(summary) {
		case "Error", "Schedule error":
			// keep hard failures visible while closed
		default:
			return
		}
	}
	s.summary = strings.TrimSpace(summary)
	s.detail = strings.TrimSpace(detail)
}

// setScheduleClosed is the only writer for outside-hours tray state.
func (s *agentStatus) setScheduleClosed(closed bool, detail string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleClosed = closed
	if closed {
		s.summary = "Outside business hours"
		s.detail = strings.TrimSpace(detail)
	}
}

func (s *agentStatus) setMode(mode NotificationMode) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.mode = mode
	s.mu.Unlock()
}

// setPromoteRestartHandler wires the sole auto-restart callback (Windows tray → requestTrayRestart).
func (s *agentStatus) setPromoteRestartHandler(fn func()) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.promoteRestart = fn
	s.mu.Unlock()
}

// markRealtimePollingFallback arms promote-after-batch when Realtime gave way to polling.
func (s *agentStatus) markRealtimePollingFallback() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.realtimeFallback = true
	s.fallbackPrintOK = false
	s.mu.Unlock()
}

// notePollingFallbackPrintOK records a successful print while in realtime→polling fallback.
func (s *agentStatus) notePollingFallbackPrintOK() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.realtimeFallback {
		s.fallbackPrintOK = true
	}
	s.mu.Unlock()
}

func (s *agentStatus) notifyMode() NotificationMode {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *agentStatus) level() trayLevel {
	s.mu.RLock()
	sum := s.summary
	s.mu.RUnlock()
	return trayLevelForSummary(sum)
}

func trayLevelForSummary(summary string) trayLevel {
	switch summary {
	case "Connection problem", "Print failed", "Schedule error", "Error":
		return trayLevelRed
	case "Outside business hours", "Waiting for receipt printer", "Starting", "Setting up", "Stopped":
		return trayLevelYellow
	default:
		return trayLevelGreen
	}
}

func (s *agentStatus) userSummary(locale string) string {
	s.mu.RLock()
	sum := s.summary
	s.mu.RUnlock()
	return trayUserSummary(sum, locale)
}

func (s *agentStatus) userDetail(locale string) string {
	s.mu.RLock()
	sum := s.summary
	det := s.detail
	s.mu.RUnlock()
	return trayUserDetail(sum, det, locale)
}

func trayUserSummary(summary, locale string) string {
	key := ""
	switch summary {
	case "Starting":
		key = "status_starting"
	case "Setting up":
		key = "status_setting_up"
	case "Ready":
		key = "status_ready"
	case "Outside business hours":
		key = "status_outside_hours"
	case "Waiting for receipt printer":
		key = "status_wait_receipt"
	case "Printing":
		key = "status_printing"
	case "Printing queue":
		key = "status_print_queue"
	case "Connection problem":
		key = "status_conn_problem"
	case "Print failed":
		key = "status_print_failed"
	case "Schedule error":
		key = "status_schedule_error"
	case "Stopped":
		key = "status_stopped"
	case "Error":
		key = "status_error"
	}
	if key != "" {
		return uiT(locale, key)
	}
	if summary == "" {
		return uiT(locale, "status_starting")
	}
	return summary
}

func trayUserDetail(summary, detail, locale string) string {
	switch summary {
	case "Ready":
		switch detail {
		case "Watching for new tickets":
			return uiT(locale, "detail_watching")
		case "Idle — waiting for orders":
			return uiT(locale, "detail_idle")
		case "Polling":
			return uiT(locale, "detail_polling")
		case "Last print OK":
			return uiT(locale, "detail_last_ok")
		default:
			if detail != "" {
				return detail
			}
			return uiT(locale, "detail_connected")
		}
	case "Outside business hours":
		if detail == "Not polling until next window" {
			return uiT(locale, "detail_outside_next")
		}
		if detail == "Not polling" {
			return uiT(locale, "detail_outside")
		}
	case "Waiting for receipt printer":
		return uiT(locale, "detail_map_station")
	case "Printing queue":
		if strings.Contains(detail, "job") {
			return uiT(locale, "detail_queue")
		}
	case "Connection problem", "Print failed", "Schedule error", "Error":
		return detail
	case "Setting up":
		return uiT(locale, "detail_setting_up")
	}
	return detail
}

func (s *agentStatus) tooltip(version, locale string) string {
	sum := s.userSummary(locale)
	det := s.userDetail(locale)
	tip := fmt.Sprintf("%s %s\n%s", uiT(locale, "tray_tooltip_prefix"), version, sum)
	if mode := s.notifyMode(); mode != "" {
		tip += "\n" + notifyModeLabel(locale, mode)
	}
	if det != "" {
		tip += "\n" + det
	}
	return tip
}

func (s *agentStatus) menuStatusLine(locale string) string {
	line := uiT(locale, "menu_status_prefix") + s.userSummary(locale)
	if mode := s.notifyMode(); mode != "" {
		line += " — " + notifyModeLabel(locale, mode)
	}
	if det := s.userDetail(locale); det != "" && len(det) < 80 {
		line += " — " + det
	}
	if cfg, err := loadConfig(defaultConfigPath()); err == nil && cfg != nil {
		if suffix := cfg.credentialStatusSuffix(locale, time.Now()); suffix != "" {
			line += " — " + suffix
		}
	}
	return line
}

func notifyModeLabel(locale string, mode NotificationMode) string {
	switch mode {
	case NotificationModeRealtime:
		return uiT(locale, "mode_realtime")
	case NotificationModePolling:
		return uiT(locale, "mode_polling")
	default:
		return string(mode)
	}
}
