//go:build windows

package main

import (
	"log"
	"os"
	"syscall"
)

func killWindowsPID(pid int) {
	if pid <= 0 {
		return
	}
	const processTerminate = 0x0001
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	terminateProcess := kernel32.NewProc("TerminateProcess")
	closeHandle := kernel32.NewProc("CloseHandle")

	h, _, err := openProcess.Call(processTerminate, 0, uintptr(pid))
	if h == 0 {
		log.Printf("tray: OpenProcess pid=%d: %v", pid, err)
		return
	}
	defer closeHandle.Call(h)
	if ok, _, err := terminateProcess.Call(h, 1); ok == 0 {
		log.Printf("tray: TerminateProcess pid=%d: %v", pid, err)
	}
}

// killCurrentProcess ends this process when os.Exit does not return (systray thread edge cases).
func killCurrentProcess() {
	killWindowsPID(os.Getpid())
}
