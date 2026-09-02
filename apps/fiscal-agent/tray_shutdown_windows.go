//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

var exitOnce sync.Once

// stopTrayAgentWork cancels polling/init and stops local HTTP wizards. Safe from any goroutine.
func stopTrayAgentWork(rt *trayRuntime) {
	log.Println("tray: stop work")
	if rt != nil {
		rt.mu.Lock()
		if rt.workCancel != nil {
			rt.workCancel()
			rt.workCancel = nil
		}
		cancel := rt.cancel
		rt.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	shutdownAllWizardServers()
	shutdownTrayLocalHTTP()
	terminateSpawnedChildren()
	releaseAgentSingleInstance()
}

// requestTrayRestart spawns a replacement instance, then exits. Spawn runs first so a
// failure leaves the current agent running. Menu "Restart" only — pair success uses rebind.
func requestTrayRestart(rt *trayRuntime) {
	log.Println("tray: restart requested")
	if err := spawnAgentRestart(); err != nil {
		log.Printf("tray: restart spawn failed: %v", err)
		loc := loadAgentUILocale()
		messageBoxOK(uiT(loc, "about_title"), fmt.Sprintf(uiT(loc, "restart_failed"), err.Error()))
		return
	}
	stopTrayAgentWork(rt)
	go systray.Quit()
	go func() {
		time.Sleep(2 * time.Second)
		exitTrayAgent()
	}()
	exitTrayAgent()
}

// requestTrayExit shuts down and terminates the process immediately.
// systray.Quit must not run on the menu goroutine — it can block and prevent exit.
func requestTrayExit(rt *trayRuntime) {
	stopTrayAgentWork(rt)
	go systray.Quit()
	// Belt-and-suspenders: if systray.Run never returns, still exit.
	go func() {
		time.Sleep(2 * time.Second)
		exitTrayAgent()
	}()
	exitTrayAgent()
}

// exitTrayAgent terminates the process (does not return).
func exitTrayAgent() {
	exitOnce.Do(func() {
		log.Println("tray: exiting process")
		os.Exit(0)
		killCurrentProcess()
	})
}
