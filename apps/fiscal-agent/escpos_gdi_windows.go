//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Shared GDI procs for Han *column* canvases only.
// Line-level text raster lives ONLY in internal/escposbitmap (do not reintroduce line GDI here).

type gdiSize struct{ CX, CY int32 }
type gdiBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type gdiRGBQuad struct{ Blue, Green, Red, Reserved byte }
type gdiBitmapInfo struct {
	Header gdiBitmapInfoHeader
	Colors [2]gdiRGBQuad
}

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procGetTextExtentPoint = gdi32.NewProc("GetTextExtentPoint32W")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSetBkColor         = gdi32.NewProc("SetBkColor")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procTextOutW           = gdi32.NewProc("TextOutW")
)

func boolToUintptr(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}

func textWidthPx(dc uintptr, s string) int {
	utf16, _ := syscall.UTF16FromString(s)
	if len(utf16) <= 1 {
		return 0
	}
	chars := uintptr(len(utf16) - 1)
	var size gdiSize
	procGetTextExtentPoint.Call(dc, uintptr(unsafe.Pointer(&utf16[0])), chars, uintptr(unsafe.Pointer(&size)))
	return int(size.CX)
}
