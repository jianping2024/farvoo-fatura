//go:build !windows

package escposbitmap

// renderText draws on the full POS-80 canvas (stub for tests / non-Windows).
func renderText(s string, style Style, fontPx int) Image {
	if s == "" {
		return Image{}
	}
	fontPx = ClampFontPx(fontPx)
	charW := fontPx / 2
	if charW < 1 {
		charW = 1
	}
	charH := fontPx
	inkW := displayWidth(s)*charW + 2
	height := charH + 2
	leftPx := alignStartPx(inkW, style.Align)

	pixels := make([]byte, MaxWidthPx*height)
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
			for x := x0; x < x1-1 && x < MaxWidthPx-1; x++ {
				if x < 0 {
					continue
				}
				border := x == x0 || x == x1-2 || y == 1 || y == height-2
				stroke := (x+y+int(r))%7 == 0
				if border || stroke || (style.Bold && (x+y+int(r))%5 == 0) {
					pixels[y*MaxWidthPx+x] = 1
				}
			}
		}
		col += span
	}
	return Image{Width: MaxWidthPx, Height: height, Pixels: pixels}
}
