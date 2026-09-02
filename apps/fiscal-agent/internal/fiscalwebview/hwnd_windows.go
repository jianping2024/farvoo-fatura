//go:build windows

package fiscalwebview

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmNull          = 0
	smtoAbortIfHung = 0x0002
)

// hwndResponsive reports whether hwnd exists and responds to a null message (not hung).
func hwndResponsive(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	user32 := windows.NewLazyDLL("user32.dll")
	isWindow := user32.NewProc("IsWindow")
	ok, _, _ := isWindow.Call(hwnd)
	if ok == 0 {
		return false
	}
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	var result uintptr
	ret, _, _ := sendMessageTimeout.Call(
		hwnd,
		wmNull,
		0,
		0,
		smtoAbortIfHung,
		250,
		uintptr(unsafe.Pointer(&result)),
	)
	return ret != 0
}
