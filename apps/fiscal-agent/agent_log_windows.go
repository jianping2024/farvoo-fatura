//go:build windows

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

const (
	agentLogMaxBytes    = 5 * 1024 * 1024
	agentLogRotateCount = 3
)

func agentDataDir() string {
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		return filepath.Join(base, "Mesa Print Agent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "MesaPrintAgent")
	}
	return filepath.Join(home, "AppData", "Local", "Mesa Print Agent")
}

func initWindowsAgentLog() {
	dir := agentDataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	path := filepath.Join(dir, "agent.log")
	rotateAgentLog(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(f))
	agentLogLocale(loadTrayUILocale(), "log_agent_start", Version)
}

func rotateAgentLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= agentLogMaxBytes {
		return
	}

	for i := agentLogRotateCount; i >= 1; i-- {
		dst := rotatedAgentLogPath(path, i)
		_ = os.Remove(dst)
		if i == 1 {
			_ = os.Rename(path, dst)
			continue
		}
		_ = os.Rename(rotatedAgentLogPath(path, i-1), dst)
	}
}

func rotatedAgentLogPath(path string, n int) string {
	return path + "." + strconv.Itoa(n)
}
