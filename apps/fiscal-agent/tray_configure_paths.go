package main

import "strings"

func isTrayConfigurePath(path string) bool {
	if isWizardStaticPath(path) {
		return true
	}
	switch path {
	case "/configure", "/pair":
		return true
	default:
		return strings.HasPrefix(path, "/api/") && path != "/api/health"
	}
}
