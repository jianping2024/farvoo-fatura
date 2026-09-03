//go:build windows

package main

import (
	"flag"
	"log"
	"os"
	"syscall"
	"unsafe"
)

func windowsPrepareConsole(args []string) {
	if len(args) < 2 {
		return
	}
	switch args[1] {
	case "discover", "pair", "configure", "config", "setup",
		"help", "-h", "--help", "version", "-v", "--version":
		attachConsoleWindow()
		return
	case "run":
		if agentArgsWantConsole(args[2:]) {
			attachConsoleWindow()
		}
		return
	}
	if agentArgsWantConsole(args[1:]) {
		attachConsoleWindow()
	}
}

func agentArgsWantConsole(args []string) bool {
	fs := flag.NewFlagSet("console-probe", flag.ContinueOnError)
	show := fs.Bool("console", false, "")
	_ = fs.Parse(args)
	return *show
}

// attachConsoleWindow creates a console for CLI subcommands (windowsgui builds).
func attachConsoleWindow() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	r, _, _ := kernel32.NewProc("AllocConsole").Call()
	if r == 0 {
		if hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call(); hwnd != 0 {
			return
		}
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
		log.SetOutput(out)
		setConsoleOutputBright(out)
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = in
	}
}

func hideConsoleWindow() {
	getConsoleWindow := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
	showWindow := syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	const swHide = 0
	_, _, _ = showWindow.Call(hwnd, swHide)
}

func showConsoleWindow() {
	hwnd, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	if hwnd == 0 {
		attachConsoleWindow()
		hwnd, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	}
	disableConsoleCloseButton(hwnd)
	showWindow := syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	const swShow = 5
	_, _, _ = showWindow.Call(hwnd, swShow)
}

// showOrFocusConsoleWindow is the sole tray console open path: show if hidden, focus if visible.
// Does not hide on second click (no toggle).
func showOrFocusConsoleWindow() {
	hwnd, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	if hwnd == 0 {
		showConsoleWindow()
		hwnd, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	} else {
		isWindowVisible := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible")
		visible, _, _ := isWindowVisible.Call(hwnd)
		if visible == 0 {
			showConsoleWindow()
			return
		}
	}
	if hwnd == 0 {
		return
	}
	const swRestore = 9
	user32 := syscall.NewLazyDLL("user32.dll")
	_, _, _ = user32.NewProc("ShowWindow").Call(hwnd, swRestore)
	_, _, _ = user32.NewProc("SetForegroundWindow").Call(hwnd)
}

func disableConsoleCloseButton(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	getSystemMenu := user32.NewProc("GetSystemMenu")
	deleteMenu := user32.NewProc("DeleteMenu")
	drawMenuBar := user32.NewProc("DrawMenuBar")
	const (
		scClose     = 0xF060
		mfByCommand = 0x00000000
	)
	menu, _, _ := getSystemMenu.Call(hwnd, 0)
	if menu == 0 {
		return
	}
	_, _, _ = deleteMenu.Call(menu, scClose, mfByCommand)
	_, _, _ = drawMenuBar.Call(hwnd)
}

// setConsoleOutputBright forces the same bright white text for all console lines
// (Windows defaults can leave the first attribute band looking washed out).
func setConsoleOutputBright(f *os.File) {
	if f == nil {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setAttr := kernel32.NewProc("SetConsoleTextAttribute")
	const brightWhite = 0x0007 // FOREGROUND_RED|GREEN|BLUE
	_, _, _ = setAttr.Call(f.Fd(), brightWhite)
}

const (
	mbOK            = 0
	mbYesNo         = 0x00000004
	mbIconInfo      = 0x00000040
	mbIconQuest     = 0x00000020
	mbSetForeground = 0x00010000
	mbTopmost       = 0x00040000
	messageBoxIDYes = 6
	messageBoxFlags = mbSetForeground | mbTopmost
)

// messageBoxOK shows a simple info dialog (tray "About").
func messageBoxOK(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(text)
	_, _, _ = messageBoxW.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), mbOK|mbIconInfo|messageBoxFlags)
}

// messageBoxYesNo returns true if the user chose Yes.
func messageBoxYesNo(title, text string) bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(text)
	ret, _, _ := messageBoxW.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), mbYesNo|mbIconQuest|messageBoxFlags)
	return ret == messageBoxIDYes
}
