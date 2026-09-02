//go:build windows

package main

func loadTrayUILocale() string {
	return loadAgentUILocale()
}
