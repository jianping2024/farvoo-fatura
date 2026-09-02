//go:build !windows

package main

// Stubs so fiscal_embed can reference the same names on non-Windows builds.
func loadTrayUILocale() string { return loadAgentUILocale() }
func setTrayUILocale(code string) error { return setAgentUILocale(code) }
