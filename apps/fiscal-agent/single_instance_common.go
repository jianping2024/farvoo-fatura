package main

import (
	"log"
	"os"
	"time"

	"farvoo-fiscal-agent/internal/fiscalipc"
)

// agentMutexName is the sole Windows single-instance mutex for the tray agent process.
// Setup must not use Inno AppMutex on this name (that blocks install with OK/Cancel);
// quiet upgrade close is PrepareToInstall taskkill in installer/farvoo-fiscal-agent.iss.
const agentMutexName = fiscalipc.AgentMutexName

// Restart successor handoff (sole path: waitAcquireAgentSingleInstance).
const (
	agentRestartAcquireTimeout = 15 * time.Second
	agentRestartAcquirePoll    = 50 * time.Millisecond
)

// isMainAgentInvocation is true for the long-running tray/console agent, not helper CLIs.
// --restart-wait is false here so the one-shot guard is never used for Restart;
// the successor must call waitAcquireAgentSingleInstance before runAgent.
func isMainAgentInvocation(args []string) bool {
	if len(args) < 2 {
		return true
	}
	switch args[1] {
	case "discover", "pair", "configure", "config", "setup", "fiscal",
		"help", "-h", "--help", "version", "-v", "--version", "--restart-wait":
		return false
	case "run":
		return true
	default:
		// FarvooFiscalAgent, FarvooFiscalAgent -console, FarvooFiscalAgent -api … -code …
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

// waitAcquireAgentSingleInstance is the sole tray-Restart successor gate:
// poll until this process holds the agent mutex (or deadline). Fail-closed —
// never enter runAgent after --restart-wait without holding the mutex.
func waitAcquireAgentSingleInstance(deadline time.Duration) bool {
	if deadline <= 0 {
		deadline = agentRestartAcquireTimeout
	}
	ok := waitAcquireAgentSingleInstancePoll(deadline, acquireAgentSingleInstance, time.Sleep)
	if !ok {
		log.Println("single-instance: restart wait-acquire timed out")
	}
	return ok
}

func waitAcquireAgentSingleInstancePoll(deadline time.Duration, acquire func() bool, sleep func(time.Duration)) bool {
	end := time.Now().Add(deadline)
	for {
		if acquire() {
			return true
		}
		if !time.Now().Before(end) {
			return false
		}
		sleep(agentRestartAcquirePoll)
	}
}
