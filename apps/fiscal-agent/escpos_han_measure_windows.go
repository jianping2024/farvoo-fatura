//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func measureHanTextWidth(s string, fontPx int, bold bool) int {
	if s == "" {
		return 0
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0
	}
	defer procDeleteDC.Call(dc)

	fontPx = resolveHanBitmapFontPx(fontPx)
	weight := uintptr(400)
	if bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, 0, 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return displayWidth(s) * (fontPx / 2)
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	utf16, _ := syscall.UTF16FromString(s)
	if len(utf16) <= 1 {
		return 0
	}
	var size gdiSize
	procGetTextExtentPoint.Call(dc, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1), uintptr(unsafe.Pointer(&size)))
	if size.CX <= 0 {
		return displayWidth(s) * (fontPx / 2)
	}
	return int(size.CX)
}
