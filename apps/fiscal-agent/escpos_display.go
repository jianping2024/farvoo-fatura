package main

import "unicode"

// Display columns match Font A pitch on 80mm (escposWidth): Han=2, Latin=1.
func displayCols(r rune) int {
	if r == 0 {
		return 0
	}
	if unicode.Is(unicode.Han, r) {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += displayCols(r)
	}
	return n
}

// truncateDisplay caps s to maxCols display columns, appending "…" when trimmed.
func truncateDisplay(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if displayWidth(s) <= maxCols {
		return s
	}
	var b []rune
	w := 0
	ellipsis := displayCols('…')
	for _, r := range s {
		cw := displayCols(r)
		if w+cw+ellipsis > maxCols {
			break
		}
		b = append(b, r)
		w += cw
	}
	return string(b) + "…"
}

// wrapDisplay splits s into chunks of at most maxCols display columns (no ellipsis).
func wrapDisplay(s string, maxCols int) []string {
	if maxCols <= 0 {
		return nil
	}
	if s == "" {
		return nil
	}
	var out []string
	var b []rune
	w := 0
	flush := func() {
		if len(b) == 0 {
			return
		}
		out = append(out, string(b))
		b = b[:0]
		w = 0
	}
	for _, r := range s {
		cw := displayCols(r)
		if cw > maxCols {
			flush()
			out = append(out, string(r))
			continue
		}
		if w+cw > maxCols {
			flush()
		}
		b = append(b, r)
		w += cw
	}
	flush()
	return out
}

// bitmapMaxDisplayCols — how many display columns fit in the 80mm bitmap canvas at fontPx.
func bitmapMaxDisplayCols(fontPx int) int {
	fontPx = resolveHanBitmapFontPx(fontPx)
	// 1 display col ≈ half of fontPx (Han glyph ≈ 2 cols ≈ font px).
	colPx := fontPx / 2
	if colPx < 1 {
		colPx = 1
	}
	max := (bitmapTextMaxWidthPx - 8) / colPx
	if max < 1 {
		return 1
	}
	return max
}
