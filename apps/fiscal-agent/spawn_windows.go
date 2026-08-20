//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func startHiddenAgentProcess(args ...string) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func spawnAgentSubcommand(subcmd string) error {
	cmd, err := startHiddenAgentProcess(subcmd)
	if err != nil {
		return fmt.Errorf("start %s: %w", subcmd, err)
	}
	registerSpawnedProcess(cmd)
	return nil
}

// spawnAgentRestart starts a replacement tray instance after this process exits.
func spawnAgentRestart() error {
	_, err := startHiddenAgentProcess("--restart-wait")
	return err
}
