//go:build !windows

package main

import "strings"

// renderBitmapColumnRow — stub places ink in left band + Qty right-aligned in field.
func renderBitmapColumnRow(left, right string, kind hanColumnRowKind, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	fontPx = resolveHanBitmapFontPx(fontPx)
	height := fontPx + hanBitmapHeightPad
	pixels := make([]byte, canvasW*height)
	stamp := stubStampInk(pixels, canvasW, height, fontPx)

	leftX := hanColumnLeftPx(kind)
	if left != "" {
		stamp(left, leftX)
	}
	if right != "" {
		rx := hanQtyTextStartPx(right, fontPx, style.Bold)
		stamp(right, rx)
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func renderBitmapLeftRow(text string, leftPx int, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	text = strings.TrimSpace(text)
	if text == "" {
		return bitmapTextImage{}
	}
	fontPx = resolveHanBitmapFontPx(fontPx)
	height := fontPx + hanBitmapHeightPad
	pixels := make([]byte, canvasW*height)
	stubStampInk(pixels, canvasW, height, fontPx)(text, leftPx)
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func renderBitmapReceiptRow(left, mid, right string, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	fontPx = resolveHanBitmapFontPx(fontPx)
	height := fontPx + hanBitmapHeightPad
	pixels := make([]byte, canvasW*height)
	stamp := stubStampInk(pixels, canvasW, height, fontPx)

	itemsEnd := escposDisplayColToPx(escposColItems)
	qtyStart := itemsEnd
	qtyW := escposDisplayColToPx(escposColQty)
	priceStart := escposDisplayColToPx(escposColItems + escposColQty)

	left = fitHanTextToPx(left, itemsEnd, fontPx, style.Bold)
	if left != "" {
		stamp(left, 0)
	}
	mid = strings.TrimSpace(mid)
	if mid != "" {
		mw := measureHanTextWidth(mid, fontPx, style.Bold)
		mx := qtyStart + (qtyW-mw)/2
		if mx < qtyStart {
			mx = qtyStart
		}
		stamp(mid, mx)
	}
	right = strings.TrimSpace(right)
	if right != "" {
		rw := measureHanTextWidth(right, fontPx, style.Bold)
		rx := canvasW - rw
		if rx < priceStart {
			rx = priceStart
		}
		stamp(right, rx)
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func renderBitmapPadRow(left, right string, fontPx int, style bitmapTextStyle) bitmapTextImage {
	canvasW := bitmapTextMaxWidthPx
	fontPx = resolveHanBitmapFontPx(fontPx)
	height := fontPx + hanBitmapHeightPad
	pixels := make([]byte, canvasW*height)
	stamp := stubStampInk(pixels, canvasW, height, fontPx)

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
	if left != "" {
		stamp(left, 0)
	}
	if right != "" {
		stamp(right, canvasW-rw)
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}

func stubStampInk(pixels []byte, canvasW, height, fontPx int) func(string, int) {
	charW := fontPx / 2
	if charW < 1 {
		charW = 1
	}
	return func(s string, x0 int) {
		col := 0
		for _, r := range s {
			if r == ' ' {
				col++
				continue
			}
			x := x0 + col*charW
			span := displayCols(r) * charW
			for y := hanBitmapPadY; y < height-hanBitmapPadY && y < height; y++ {
				for xi := x; xi < x+span-1 && xi < canvasW-1; xi++ {
					if xi >= 0 {
						pixels[y*canvasW+xi] = 1
					}
				}
			}
			col += displayCols(r)
		}
	}
}
