//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// shellExecute is the sole Windows ShellExecuteW wrapper (browser open + uninstaller).
func shellExecute(verb, file, params string) error {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")
	v, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}
	f, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	var p *uint16
	if strings.TrimSpace(params) != "" {
		p, err = syscall.UTF16PtrFromString(params)
		if err != nil {
			return err
		}
	}
	// SW_SHOWNORMAL — required so UAC can appear for admin uninstallers.
	ret, _, callErr := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(p)),
		0,
		1,
	)
	if ret <= 32 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecute: %w", callErr)
		}
		return fmt.Errorf("ShellExecute failed (code %d)", ret)
	}
	return nil
}
