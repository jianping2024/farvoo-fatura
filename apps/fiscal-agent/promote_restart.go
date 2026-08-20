package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Sole auto path back to preferred realtime after realtime→polling fallback:
// finish the current local queue after at least one successful print, then invoke
// the tray restart handler (same as menu Restart — full process respawn, no confirm).
// Do not add an in-process notifier switch beside this.

const realtimePromoteRestartCooldown = 5 * time.Minute

func realtimePromoteRestartMarkerPath() string {
	return filepath.Join(agentDataDir(), ".last_realtime_promote_restart")
}

func realtimePromoteRestartCooldownOK() bool {
	raw, err := os.ReadFile(realtimePromoteRestartMarkerPath())
	if err != nil {
		return true
	}
	sec, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || sec <= 0 {
		return true
	}
	return time.Since(time.Unix(sec, 0)) >= realtimePromoteRestartCooldown
}

func markRealtimePromoteRestart() {
	path := realtimePromoteRestartMarkerPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o600)
}

// tryPromoteRestartAfterPollingBatch fires at most once per fallback session when the
// queue drained after a successful print. Cooldown prevents restart loops when Realtime
// still cannot connect after respawn.
func tryPromoteRestartAfterPollingBatch(status *agentStatus, queueLen int) {
	if status == nil || queueLen > 0 {
		return
	}

	status.mu.Lock()
	ready := status.realtimeFallback && status.fallbackPrintOK && status.promoteRestart != nil
	fn := status.promoteRestart
	if !ready {
		status.mu.Unlock()
		return
	}
	if !realtimePromoteRestartCooldownOK() {
		status.mu.Unlock()
		log.Println("Realtime promote: tray restart deferred (cooldown)")
		return
	}
	status.realtimeFallback = false
	status.fallbackPrintOK = false
	status.mu.Unlock()

	markRealtimePromoteRestart()
	log.Println("tray: auto-restart after polling batch (restore realtime preferred)")
	fn()
}
