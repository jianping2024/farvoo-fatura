package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestWaitPairingOnListenAddrSeesJWT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- waitPairingOnListenAddr(ctx, path, "", "127.0.0.1:17892")
	}()

	time.Sleep(200 * time.Millisecond)
	cfg := &config{APIBase: "http://192.168.0.141", AgentJWT: "test-jwt", DeviceID: "dev-1"}
	if err := saveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pairing")
	}
}

func TestWaitForAgentPairingUsesTrayWhenListenAddrSet(t *testing.T) {
	// Unit env has no tray listener; empty listenAddr → runPairingWizard path (CLI/console).
	if addr := trayLocal.listenAddr(); addr != "" {
		t.Fatalf("expected empty tray listenAddr in unit test env, got %q", addr)
	}
}
