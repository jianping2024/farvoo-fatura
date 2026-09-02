//go:build !windows

package main

import "context"

func runFiscalCommand() {}

func openFiscalShellWindow(baseURL string) {}

func startFiscalIPC(ctx context.Context, open func()) {}

var openFiscalOnTrayStart bool

func (rt *trayRuntime) openFiscalShellFromTray() {}
