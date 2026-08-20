//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

// renderBitmapColumnRow draws left + right fields on a full-width POS-80 canvas (576 dots).
// Right field is right-aligned in the Qty column field (pixel anchor — no space padding).
func renderBitmapColumnRow(left, right string, kind hanColumnRowKind, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	if left == "" && right == "" {
		return bitmapTextImage{}
	}

	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteDC.Call(dc)

	fontPx = resolveHanBitmapFontPx(fontPx)
	weight := uintptr(400)
	if style.Bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, uintptr(boolToUintptr(style.Underline)), 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	leftH := textLineHeightPx(dc, left, fontPx)
	rightH := textLineHeightPx(dc, right, fontPx)
	height := leftH
	if rightH > height {
		height = rightH
	}
	if height <= 0 {
		height = fontPx + 8
	}

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
		return bitmapTextImage{}
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

	const padY = 4
	leftX := hanColumnLeftPx(kind)

	if left != "" {
		drawTextOutW(dc, leftX, padY, left)
	}
	if right != "" {
		rx := hanQtyTextStartPx(right, fontPx, style.Bold)
		drawTextOutW(dc, rx, padY, right)
	}

	pixels := make([]byte, canvasW*height)
	for y := 0; y < height; y++ {
		for x := 0; x < canvasW; x++ {
			off := y*stride + x*4
			if raw[off] < 128 || raw[off+1] < 128 || raw[off+2] < 128 {
				pixels[y*canvasW+x] = 1
			}
		}
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

// renderBitmapLeftRow draws one underlined note/menu line at leftPx on a full-width canvas.
func renderBitmapLeftRow(text string, leftPx int, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	text = strings.TrimSpace(text)
	if text == "" {
		return bitmapTextImage{}
	}

	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteDC.Call(dc)

	fontPx = resolveHanBitmapFontPx(fontPx)
	weight := uintptr(400)
	if style.Bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, uintptr(boolToUintptr(style.Underline)), 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	height := textLineHeightPx(dc, text, fontPx)
	if height <= 0 {
		height = fontPx + hanBitmapHeightPad
	}

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
		return bitmapTextImage{}
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

	drawTextOutW(dc, leftPx, hanBitmapPadY, text)

	pixels := make([]byte, canvasW*height)
	for y := 0; y < height; y++ {
		for x := 0; x < canvasW; x++ {
			off := y*stride + x*4
			if raw[off] < 128 || raw[off+1] < 128 || raw[off+2] < 128 {
				pixels[y*canvasW+x] = 1
			}
		}
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func renderBitmapReceiptRow(left, mid, right string, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteDC.Call(dc)

	fontPx = resolveHanBitmapFontPx(fontPx)
	weight := uintptr(400)
	if style.Bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, 0, 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	itemsEnd := escposDisplayColToPx(escposColItems)
	qtyStart := itemsEnd
	qtyW := escposDisplayColToPx(escposColQty)
	priceStart := escposDisplayColToPx(escposColItems + escposColQty)

	left = fitHanTextToPx(left, itemsEnd, fontPx, style.Bold)
	mid = strings.TrimSpace(mid)
	right = strings.TrimSpace(right)

	height := textLineHeightPx(dc, left+mid+right, fontPx)
	if height <= 0 {
		height = fontPx + hanBitmapHeightPad
	}

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
		return bitmapTextImage{}
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

	if left != "" {
		drawTextOutW(dc, 0, hanBitmapPadY, left)
	}
	if mid != "" {
		mw := measureHanTextWidth(mid, fontPx, style.Bold)
		mx := qtyStart + (qtyW-mw)/2
		if mx < qtyStart {
			mx = qtyStart
		}
		drawTextOutW(dc, mx, hanBitmapPadY, mid)
	}
	if right != "" {
		rw := measureHanTextWidth(right, fontPx, style.Bold)
		rx := canvasW - rw
		if rx < priceStart {
			rx = priceStart
		}
		drawTextOutW(dc, rx, hanBitmapPadY, right)
	}

	pixels := make([]byte, canvasW*height)
	for y := 0; y < height; y++ {
		for x := 0; x < canvasW; x++ {
			off := y*stride + x*4
			if raw[off] < 128 || raw[off+1] < 128 || raw[off+2] < 128 {
				pixels[y*canvasW+x] = 1
			}
		}
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func renderBitmapPadRow(left, right string, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteDC.Call(dc)

	fontPx = resolveHanBitmapFontPx(fontPx)
	weight := uintptr(400)
	if style.Bold {
		weight = 700
	}
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(^uint32(fontPx-1)+1), 0, 0, 0, weight, 0, 0, 0,
		1, 4, 0, 0, 0, uintptr(unsafe.Pointer(face)),
	)
	if font == 0 {
		return bitmapTextImage{}
	}
	defer procDeleteObject.Call(font)
	oldFont, _, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	right = strings.TrimSpace(right)
	rw := 0
	if right != "" {
		rw = measureHanTextWidth(right, fontPx, style.Bold)
	}
	gap := escposDisplayColToPx(1)
	leftMax := canvasW - rw - gap
	if leftMax < 1 {
		leftMax = 1
	}
	left = fitHanTextToPx(left, leftMax, fontPx, style.Bold)

	height := textLineHeightPx(dc, left+right, fontPx)
	if height <= 0 {
		height = fontPx + hanBitmapHeightPad
	}

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
		return bitmapTextImage{}
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

	if left != "" {
		drawTextOutW(dc, 0, hanBitmapPadY, left)
	}
	if right != "" {
		drawTextOutW(dc, canvasW-rw, hanBitmapPadY, right)
	}

	pixels := make([]byte, canvasW*height)
	for y := 0; y < height; y++ {
		for x := 0; x < canvasW; x++ {
			off := y*stride + x*4
			if raw[off] < 128 || raw[off+1] < 128 || raw[off+2] < 128 {
				pixels[y*canvasW+x] = 1
			}
		}
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
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
	h := int(size.CY) + hanBitmapHeightPad
	if h < fontPx+hanBitmapHeightPad {
		h = fontPx + hanBitmapHeightPad
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
