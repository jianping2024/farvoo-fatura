//go:build windows

package main

import (
	"log"
	"os/exec"
	"sync"
)

var (
	spawnedProcsMu sync.Mutex
	spawnedProcs   []*exec.Cmd
)

func registerSpawnedProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	spawnedProcsMu.Lock()
	spawnedProcs = append(spawnedProcs, cmd)
	spawnedProcsMu.Unlock()
	go func() {
		_ = cmd.Wait()
		spawnedProcsMu.Lock()
		defer spawnedProcsMu.Unlock()
		for i, c := range spawnedProcs {
			if c == cmd {
				spawnedProcs = append(spawnedProcs[:i], spawnedProcs[i+1:]...)
				break
			}
		}
	}()
}

// terminateSpawnedChildren ends helper processes started via spawnAgentSubcommand (legacy / CLI).
func terminateSpawnedChildren() {
	spawnedProcsMu.Lock()
	list := append([]*exec.Cmd(nil), spawnedProcs...)
	spawnedProcsMu.Unlock()
	for _, cmd := range list {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		pid := cmd.Process.Pid
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("tray: kill spawned pid=%d: %v", pid, err)
		}
		killWindowsPID(pid)
	}
}
