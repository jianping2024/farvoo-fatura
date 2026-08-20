//go:build !windows

package main

func acquireAgentSingleInstance() bool { return true }

func exitAlreadyRunning() {}
