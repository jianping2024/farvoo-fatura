//go:build windows

package fiscalclient

import (
	"log"
	"syscall"
	"unsafe"
)

// ClientMutexName is the sole LAN Client process single-instance mutex.
const ClientMutexName = `Global\FarvooFiscalClient-SingleInstance-v1`

var clientInstanceMutex syscall.Handle

// AcquireClientSingleInstance is the sole Client mutex acquire (fail-closed).
// Returns false if another Client already holds the mutex.
func AcquireClientSingleInstance() bool {
	if clientInstanceMutex != 0 {
		return true
	}
	name, err := syscall.UTF16PtrFromString(ClientMutexName)
	if err != nil {
		log.Println("client single-instance: mutex name:", err)
		return false
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openMutexW := kernel32.NewProc("OpenMutexW")
	createMutexW := kernel32.NewProc("CreateMutexW")
	getLastError := kernel32.NewProc("GetLastError")

	const (
		mutexAllAccess     = 0x001F0001
		errorAlreadyExists = syscall.Errno(183)
	)
	if existing, _, _ := openMutexW.Call(mutexAllAccess, 0, uintptr(unsafe.Pointer(name))); existing != 0 {
		_ = syscall.CloseHandle(syscall.Handle(existing))
		return false
	}
	handle, _, _ := createMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		log.Println("client single-instance: CreateMutex failed")
		return false
	}
	clientInstanceMutex = syscall.Handle(handle)
	errno, _, _ := getLastError.Call()
	if syscall.Errno(errno) == errorAlreadyExists {
		_ = syscall.CloseHandle(clientInstanceMutex)
		clientInstanceMutex = 0
		return false
	}
	return true
}
