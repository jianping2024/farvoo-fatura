//go:build windows

package main

import (
	"log"
	"os"
	"syscall"
	"unsafe"
)

const errorAlreadyExists = syscall.Errno(183)

// Held until process exit so the mutex stays owned by this instance.
var agentInstanceMutex syscall.Handle

// acquireAgentSingleInstance returns false if another main agent instance is already running.
func acquireAgentSingleInstance() bool {
	if agentInstanceMutex != 0 {
		return true
	}
	name, err := syscall.UTF16PtrFromString(agentMutexName)
	if err != nil {
		return true
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openMutexW := kernel32.NewProc("OpenMutexW")
	createMutexW := kernel32.NewProc("CreateMutexW")
	getLastError := kernel32.NewProc("GetLastError")

	const mutexAllAccess = 0x001F0001
	if existing, _, _ := openMutexW.Call(mutexAllAccess, 0, uintptr(unsafe.Pointer(name))); existing != 0 {
		_ = syscall.CloseHandle(syscall.Handle(existing))
		log.Println("single-instance: OpenMutex found existing agent")
		return false
	}

	// bInitialOwner=TRUE so this instance owns the mutex immediately.
	handle, _, _ := createMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return true
	}
	agentInstanceMutex = syscall.Handle(handle)

	errno, _, _ := getLastError.Call()
	if syscall.Errno(errno) == errorAlreadyExists {
		_ = syscall.CloseHandle(agentInstanceMutex)
		agentInstanceMutex = 0
		log.Println("single-instance: CreateMutex ERROR_ALREADY_EXISTS")
		return false
	}
	return true
}

func exitAlreadyRunning() {
	loc := loadAgentUILocale()
	messageBoxOK(uiT(loc, "instance_running_title"), uiT(loc, "instance_running_body"))
	os.Exit(0)
}

// releaseAgentSingleInstance closes our mutex handle so the next start is not blocked after exit.
func releaseAgentSingleInstance() {
	if agentInstanceMutex == 0 {
		return
	}
	_ = syscall.CloseHandle(agentInstanceMutex)
	agentInstanceMutex = 0
}
