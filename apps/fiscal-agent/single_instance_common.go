package main

import "os"

// agentMutexName is the sole Windows single-instance mutex for the tray agent process.
// Setup must not use Inno AppMutex on this name (that blocks install with OK/Cancel);
// quiet upgrade close is PrepareToInstall taskkill in installer/mesa-print-agent.iss.
const agentMutexName = `Global\FarvooFiscalAgent-SingleInstance-v1`

// isMainAgentInvocation is true for the long-running tray/console agent, not helper CLIs.
func isMainAgentInvocation(args []string) bool {
	if len(args) < 2 {
		return true
	}
	switch args[1] {
	case "discover", "pair", "configure", "config", "setup",
		"help", "-h", "--help", "version", "-v", "--version", "--restart-wait":
		return false
	case "run":
		return true
	default:
		// MesaPrintAgent, MesaPrintAgent -console, MesaPrintAgent -api … -code …
		return true
	}
}

func guardMainAgentSingleInstance() {
	if !isMainAgentInvocation(os.Args) {
		return
	}
	if !acquireAgentSingleInstance() {
		exitAlreadyRunning()
	}
}
