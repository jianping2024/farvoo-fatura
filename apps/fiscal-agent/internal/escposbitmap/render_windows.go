//go:build windows

package escposbitmap

import (
	"strings"
	"syscall"
	"unsafe"
)

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

func renderText(s string, style Style, fontPx int) Image {
	canvasW := MaxWidthPx
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return Image{}
	}

	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return Image{}
	}
	defer procDeleteDC.Call(dc)

	fontPx = ClampFontPx(fontPx)
	weight := uintptr(400)
	if style.Bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	underline := uintptr(0)
	if style.Underline {
		underline = 1
	}
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, underline, 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return Image{}
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	utf16, _ := syscall.UTF16FromString(s)
	if len(utf16) <= 1 {
		return Image{}
	}

	textW := textWidthPx(dc, s) + 2
	height := textLineHeightPx(dc, s, fontPx)
	if height <= 0 {
		height = fontPx + heightPad
	}
	leftPx := alignStartPx(textW, style.Align)

	var bits unsafe.Pointer
	stride := ((canvasW*32 + 31) / 32) * 4
	bi := gdiBitmapInfo{}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.Width = int32(canvasW)
	bi.Header.Height = -int32(height)
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bitmap, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return Image{}
	}
	defer procDeleteObject.Call(bitmap)
	oldBitmap, _, _ := procSelectObject.Call(dc, bitmap)
	defer procSelectObject.Call(dc, oldBitmap)

	raw := unsafe.Slice((*byte)(bits), stride*height)
	for i := range raw {
		raw[i] = 0xff
	}

	procSetBkColor.Call(dc, 0x00ffffff)
	procSetTextColor.Call(dc, 0x00000000)
	procSetBkMode.Call(dc, 2)
	drawTextOutW(dc, leftPx, padY, s)

	pixels := make([]byte, canvasW*height)
	for y := 0; y < height; y++ {
		for x := 0; x < canvasW; x++ {
			off := y*stride + x*4
			if raw[off] < 128 || raw[off+1] < 128 || raw[off+2] < 128 {
				pixels[y*canvasW+x] = 1
			}
		}
	}
	return Image{Width: canvasW, Height: height, Pixels: pixels}
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

func textLineHeightPx(dc uintptr, s string, fontPx int) int {
	if s == "" {
		return 0
	}
	utf16, _ := syscall.UTF16FromString(s)
	if len(utf16) <= 1 {
		return 0
	}
	var size gdiSize
	procGetTextExtentPoint.Call(dc, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1), uintptr(unsafe.Pointer(&size)))
	h := int(size.CY) + heightPad
	if h < fontPx+heightPad {
		h = fontPx + heightPad
	}
	return h
}

func drawTextOutW(dc uintptr, x, y int, s string) {
	utf16, _ := syscall.UTF16FromString(s)
	if len(utf16) <= 1 {
		return
	}
	procTextOutW.Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)-1))
}
