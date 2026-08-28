//go:build !windows

package main

// renderBitmapText draws on the full POS-80 canvas with alignment baked in (stub for tests).
func renderBitmapText(s string, style bitmapTextStyle, fontPx int) bitmapTextImage {
	if s == "" {
		return bitmapTextImage{}
	}
	fontPx = resolveHanBitmapFontPx(fontPx)
	charW := fontPx / 2
	if charW < 1 {
		charW = 1
	}
	charH := fontPx
	inkW := displayWidth(s)*charW + 2
	canvasW := bitmapTextMaxWidthPx
	height := charH + 2
	leftPx := hanBitmapAlignStartPx(inkW, style.Align)

	pixels := make([]byte, canvasW*height)
	col := 0
	for _, r := range s {
		span := displayCols(r)
		if r == ' ' {
			col += span
			continue
		}
		x0 := leftPx + col*charW + 1
		x1 := x0 + span*charW
		for y := 1; y < height-1; y++ {
			for x := x0; x < x1-1 && x < canvasW-1; x++ {
				if x < 0 {
					continue
				}
				border := x == x0 || x == x1-2 || y == 1 || y == height-2
				stroke := (x+y+int(r))%7 == 0
				if border || stroke || style.Bold && (x+y+int(r))%5 == 0 {
					pixels[y*canvasW+x] = 1
				}
			}
		}
		col += span
	}
	return bitmapTextImage{Width: canvasW, Height: height, Pixels: pixels}
}
