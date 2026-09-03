//go:build windows

package fiscalwebview

import (
	"strings"
	"syscall"
	"unsafe"
)

// FocusExistingByTitle is the sole cross-process focus helper (Client second launch).
// Best-effort: returns false if no matching window; never panics.
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
	if hwnd == 0 {
		return false
	}
	if !hwndResponsive(hwnd) {
		return false
	}
	focusHWND(hwnd)
	return true
}
