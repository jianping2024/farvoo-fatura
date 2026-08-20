//go:build !windows

package main

import "context"

type trayRuntime struct{}

type trayLocalHTTP struct{}

var trayLocal trayLocalHTTP

func startTrayLocalHTTP(rt *trayRuntime) {}
func shutdownTrayLocalHTTP()             {}

func (t *trayLocalHTTP) listenAddr() string { return "" }

func (t *trayLocalHTTP) runConfigureSession(ctx context.Context, configPath, prefillAPI, rawQuery string) error {
	return runConfigureWizard(ctx, configPath, prefillAPI, rawQuery)
}
