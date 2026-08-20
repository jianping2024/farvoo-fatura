//go:build !windows

package main

func measureHanTextWidth(s string, fontPx int, bold bool) int {
	fontPx = resolveHanBitmapFontPx(fontPx)
	charW := fontPx / 2
	if charW < 1 {
		charW = 1
	}
	return displayWidth(s) * charW
}
