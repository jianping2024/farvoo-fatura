//go:build windows

package fiscalwebview

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	swRestore   = 9
	swShow      = 5
	gaRoot      = 2
	wmSysCommand = 0x0112
	scRestore   = 0xF120
	flashwAll   = 0x00000003
	flashwTimer = 0x00000004
)

// focusHWND is the sole restore/activate helper for an existing shell HWND.
// Safe from any thread: PostMessage(SC_RESTORE) is handled by the WebView Run() pump.
// Guarantees attempt to un-minimize; foreground is best-effort; flashes taskbar if still not front.
func focusHWND(hwnd uintptr) {
	hwnd = rootHWND(hwnd)
	if hwnd == 0 || !isWindow(hwnd) {
		return
	}
	user32 := windows.NewLazyDLL("user32.dll")
	showWindow := user32.NewProc("ShowWindow")
	postMessage := user32.NewProc("PostMessageW")
	isIconic := user32.NewProc("IsIconic")
	bringToTop := user32.NewProc("BringWindowToTop")
	setForeground := user32.NewProc("SetForegroundWindow")
	getForeground := user32.NewProc("GetForegroundWindow")
	getWindowThread := user32.NewProc("GetWindowThreadProcessId")
	attachThreadInput := user32.NewProc("AttachThreadInput")
	flashWindowEx := user32.NewProc("FlashWindowEx")
	kernel32 := windows.NewLazyDLL("kernel32.dll")
	getCurrentThreadId := kernel32.NewProc("GetCurrentThreadId")

	iconic, _, _ := isIconic.Call(hwnd)
	if iconic != 0 {
		_, _, _ = showWindow.Call(hwnd, swRestore)
		_, _, _ = postMessage.Call(hwnd, wmSysCommand, scRestore, 0)
	} else {
		_, _, _ = showWindow.Call(hwnd, swShow)
	}

	fg, _, _ := getForeground.Call()
	curThread, _, _ := getCurrentThreadId.Call()
	tgtThread, _, _ := getWindowThread.Call(hwnd, 0)
	fgThread, _, _ := getWindowThread.Call(fg, 0)

	if fg != 0 && fgThread != 0 && fgThread != curThread {
		_, _, _ = attachThreadInput.Call(curThread, fgThread, 1)
	}
	if tgtThread != 0 && tgtThread != curThread {
		_, _, _ = attachThreadInput.Call(curThread, tgtThread, 1)
	}
	_, _, _ = bringToTop.Call(hwnd)
	_, _, _ = setForeground.Call(hwnd)
	if tgtThread != 0 && tgtThread != curThread {
		_, _, _ = attachThreadInput.Call(curThread, tgtThread, 0)
	}
	if fg != 0 && fgThread != 0 && fgThread != curThread {
		_, _, _ = attachThreadInput.Call(curThread, fgThread, 0)
	}

	front, _, _ := getForeground.Call()
	if front != hwnd {
		flashTaskbar(flashWindowEx, hwnd)
	}
}

func rootHWND(hwnd uintptr) uintptr {
	if hwnd == 0 {
		return 0
	}
	getAncestor := windows.NewLazyDLL("user32.dll").NewProc("GetAncestor")
	root, _, _ := getAncestor.Call(hwnd, gaRoot)
	if root != 0 {
		return root
	}
	return hwnd
}

func isWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ok, _, _ := windows.NewLazyDLL("user32.dll").NewProc("IsWindow").Call(hwnd)
	return ok != 0
}

type flashwInfo struct {
	cbSize    uint32
	hwnd      uintptr
	flags     uint32
	count     uint32
	timeout   uint32
}

func flashTaskbar(flashWindowEx *windows.LazyProc, hwnd uintptr) {
	info := flashwInfo{
		cbSize:  uint32(unsafe.Sizeof(flashwInfo{})),
		hwnd:    hwnd,
		flags:   flashwAll | flashwTimer,
		count:   3,
		timeout: 0,
	}
	_, _, _ = flashWindowEx.Call(uintptr(unsafe.Pointer(&info)))
}
