package main

import "strings"

func normalizeTextEncoding(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "utf8", "utf-8":
		return "utf8"
	case "latin", "windows1252", "cp1252":
		return "latin"
	default:
		return "auto"
	}
}
