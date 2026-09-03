//go:build !windows

package fiscalclient

// AcquireClientSingleInstance always succeeds off Windows (Client is Windows-only).
func AcquireClientSingleInstance() bool { return true }

// ClientMutexName documents the Windows mutex; unused off Windows.
const ClientMutexName = `Global\FarvooFiscalClient-SingleInstance-v1`
