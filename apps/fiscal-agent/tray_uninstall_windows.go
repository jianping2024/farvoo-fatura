//go:build windows

package main

import (
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// uninstallAgentWithUserData is the sole tray uninstall path: wipe local config+logs,
// launch Setup uninstaller when present, then exit so files can be removed.
func uninstallAgentWithUserData(rt *trayRuntime) {
	loc := loadAgentUILocale()
	removed := removeAgentUserDataDirs()
	unins := findFiscalAgentUninstallCommand()
	if unins == "" {
		log.Println("tray: uninstall — no Setup uninstaller found (portable?)")
		messageBoxOK(uiT(loc, "about_title"), uiT(loc, "uninstall_portable_hint"))
		requestTrayExit(rt)
		return
	}
	log.Printf("tray: uninstall — removed user data=%v; launching %s", removed, unins)
	if err := startWindowsUninstaller(unins); err != nil {
		log.Printf("tray: uninstall launch failed: %v", err)
		messageBoxOK(uiT(loc, "about_title"), fmtUninstallLaunchFail(loc, err))
		requestTrayExit(rt)
		return
	}
	requestTrayExit(rt)
}

func fmtUninstallLaunchFail(locale string, err error) string {
	return strings.ReplaceAll(uiT(locale, "uninstall_launch_fail"), "%s", err.Error())
}

func removeAgentUserDataDirs() bool {
	ok := true
	for _, dir := range agentUserDataDirs() {
		if dir == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("tray: uninstall remove %s: %v", dir, err)
			ok = false
		}
	}
	return ok
}

func findFiscalAgentUninstallCommand() string {
	for _, keyName := range fiscalAgentUninstallRegistryKeyNames() {
		for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
			for _, base := range []string{
				`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`,
				`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\`,
			} {
				if cmd := readUninstallString(root, base+keyName); cmd != "" {
					return cmd
				}
			}
		}
	}
	if cmd := findUninstallByDisplayName(); cmd != "" {
		return cmd
	}
	return uninstallCommandBesideExecutable()
}

func readUninstallString(root registry.Key, path string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, _, err := k.GetStringValue("QuietUninstallString")
	if err == nil && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	s, _, err = k.GetStringValue("UninstallString")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func findUninstallByDisplayName() string {
	roots := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	subPaths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	for _, root := range roots {
		for _, sub := range subPaths {
			k, err := registry.OpenKey(root, sub, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			names, err := k.ReadSubKeyNames(-1)
			_ = k.Close()
			if err != nil {
				continue
			}
			for _, name := range names {
				path := sub + `\` + name
				sk, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
				if err != nil {
					continue
				}
				display, _, _ := sk.GetStringValue("DisplayName")
				_ = sk.Close()
				if fiscalAgentUninstallDisplayNameMatch(display) {
					if cmd := readUninstallString(root, path); cmd != "" {
						return cmd
					}
				}
			}
		}
	}
	return ""
}

// startWindowsUninstaller is the sole uninstaller launch path: parse cmdline → ShellExecute.
// Never cmd /C + HideWindow (breaks quoted Program Files paths and UAC).
func startWindowsUninstaller(cmdline string) error {
	exe, args, err := parseWindowsCommandLine(cmdline)
	if err != nil {
		return err
	}
	args = ensureInnoSilentArgs(args)
	return shellExecute("open", exe, args)
}
