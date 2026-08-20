//go:build !windows

package main

// renderBitmapText draws a stub glyph grid (tests on non-Windows). Never truncateDisplay.
// Height tracks fontPx; DoubleH/DoubleW do not enlarge it.
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
	width := displayWidth(s)*charW + 2
	if width > bitmapTextMaxWidthPx {
		width = bitmapTextMaxWidthPx
	}
	height := charH + 2
	pixels := make([]byte, width*height)
	col := 0
	for _, r := range s {
		span := displayCols(r)
		if r == ' ' {
			col += span
			continue
		}
		x0 := col*charW + 1
		x1 := x0 + span*charW
		for y := 1; y < height-1; y++ {
			for x := x0; x < x1-1 && x < width-1; x++ {
				border := x == x0 || x == x1-2 || y == 1 || y == height-2
				stroke := (x+y+int(r))%7 == 0
				if border || stroke || style.Bold && (x+y+int(r))%5 == 0 {
					pixels[y*width+x] = 1
				}
			}
		}
		col += span
	}
	return bitmapTextImage{Width: width, Height: height, Pixels: pixels}
}
