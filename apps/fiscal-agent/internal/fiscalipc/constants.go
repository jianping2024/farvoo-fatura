package fiscalipc

import "errors"

// ErrUnsupportedPlatform is returned on non-Windows builds.
var ErrUnsupportedPlatform = errors.New("fiscal ipc: windows only")

// CommandOpenFiscal is the sole IPC command to open the fiscal shell on the running Agent.
const CommandOpenFiscal = "open-fiscal"

// AgentMutexName must match main.agentMutexName (tray single-instance).
const AgentMutexName = `Global\FarvooFiscalAgent-SingleInstance-v1`
