//go:build !windows

package main

func uninstallAgentWithUserData(rt *trayRuntime) {
	_ = rt
}

func confirmTrayUninstall(locale string) bool {
	_ = locale
	return false
}
