//go:build !windows

package fiscalipc

import "context"

// ServeAgentCommands is a no-op on non-Windows platforms.
func ServeAgentCommands(ctx context.Context, onOpenFiscal func()) {}

// RequestOpenFiscal returns ErrUnsupportedPlatform on non-Windows platforms.
func RequestOpenFiscal() error {
	return ErrUnsupportedPlatform
}

// AgentInstanceRunning reports false on non-Windows platforms.
func AgentInstanceRunning() bool { return false }
