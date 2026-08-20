package main

import (
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestTryPromoteRestartAfterPollingBatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	_ = os.Remove(realtimePromoteRestartMarkerPath())

	var calls atomic.Int32
	status := &agentStatus{}
	status.setPromoteRestartHandler(func() { calls.Add(1) })

	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 0 {
		t.Fatalf("no fallback: calls=%d", calls.Load())
	}

	status.markRealtimePollingFallback()
	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 0 {
		t.Fatalf("no successful print yet: calls=%d", calls.Load())
	}

	status.notePollingFallbackPrintOK()
	tryPromoteRestartAfterPollingBatch(status, 2)
	if calls.Load() != 0 {
		t.Fatalf("queue not empty: calls=%d", calls.Load())
	}

	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 1 {
		t.Fatalf("expected one restart, calls=%d", calls.Load())
	}

	// Disarmed after fire.
	status.notePollingFallbackPrintOK()
	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 1 {
		t.Fatalf("should stay disarmed without new fallback, calls=%d", calls.Load())
	}
}

func TestTryPromoteRestartCooldown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_STATE_HOME", dir)

	markRealtimePromoteRestart()

	var calls atomic.Int32
	status := &agentStatus{}
	status.setPromoteRestartHandler(func() { calls.Add(1) })
	status.markRealtimePollingFallback()
	status.notePollingFallbackPrintOK()

	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 0 {
		t.Fatalf("cooldown should block restart, calls=%d", calls.Load())
	}

	status.mu.Lock()
	armed := status.realtimeFallback && status.fallbackPrintOK
	status.mu.Unlock()
	if !armed {
		t.Fatal("expected fallback still armed after cooldown defer")
	}

	past := time.Now().Add(-realtimePromoteRestartCooldown - time.Minute).Unix()
	marker := realtimePromoteRestartMarkerPath()
	_ = os.MkdirAll(filepath.Dir(marker), 0o700)
	_ = os.WriteFile(marker, []byte(strconv.FormatInt(past, 10)), 0o600)

	tryPromoteRestartAfterPollingBatch(status, 0)
	if calls.Load() != 1 {
		t.Fatalf("after cooldown expected restart, calls=%d", calls.Load())
	}
}
