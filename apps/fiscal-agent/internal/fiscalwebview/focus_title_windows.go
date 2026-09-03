//go:build windows

package fiscalwebview

import (
	"strings"
	"syscall"
	"unsafe"
)

// FocusExistingByTitle is the sole cross-process focus helper (Client second launch).
// Same restore path as in-process: IsWindow → focusHWND (works when minimized).
func FocusExistingByTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		title = WindowTitle
	}
	ptr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	findWindowW := user32.NewProc("FindWindowW")
	hwnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(ptr)))
	if !isWindow(hwnd) {
		return false
	}
	focusHWND(hwnd)
	return true
}
