//go:build !windows

package main

import "fmt"

func spawnAgentSubcommand(subcmd string) error {
	return fmt.Errorf("spawn %s: not supported on %s", subcmd, "this OS")
}

func messageBoxOK(title, text string) {
	fmt.Printf("%s\n%s\n", title, text)
}
