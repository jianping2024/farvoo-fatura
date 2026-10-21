package main

import (
	"strings"

	"farvoo-fiscal-agent/internal/escposbitmap"
	"farvoo-fiscal-agent/internal/escposenc"
)

// Vertical padding for Han GS v 0 rows — keep tight vs Font A 1×2 (avoid paper waste).
const (
	hanBitmapPadY      = 1
	hanBitmapHeightPad = 2
)

// Han typography roles — the Font A GS ! size each canvas row mirrors (base B = han_bitmap_font_px).
type hanFontRole int

const (
	hanFontSmall hanFontRole = iota // branding / footer = Font A 1×1
	hanFontBody                     // column block + meta = Font A 1×2
	hanFontLarge                    // title / table no. = Font A 2×2
)

func hanRoleSize(role hanFontRole) (doubleW, doubleH bool) {
	switch role {
	case hanFontSmall:
		return false, false
	case hanFontLarge:
		return true, true
	default:
		return false, true
	}
}

// hanFontPxForRole — TrueType render size for role (sole mapping: escposbitmap.Scale).
func hanFontPxForRole(basePx int, role hanFontRole) int {
	dw, dh := hanRoleSize(role)
	px, _ := escposbitmap.Scale(basePx, dw, dh)
	return px
}

// hanStretchRow applies the role's vertical dot doubling (1×2) to a rendered canvas row.
func hanStretchRow(img bitmapTextImage, role hanFontRole) bitmapTextImage {
	dw, dh := hanRoleSize(role)
	_, stretchY := escposbitmap.Scale(bitmapTextDefaultFontPx, dw, dh)
	return bitmapTextImage(escposbitmap.StretchY(escposbitmap.Image(img), stretchY))
}

// renderHanRoleRow — renderBitmapLeftRow at the role's size, stretched (not yet trimmed).
func renderHanRoleRow(text string, leftPx, basePx int, role hanFontRole, style bitmapTextStyle) bitmapTextImage {
	return hanStretchRow(renderBitmapLeftRow(text, leftPx, hanFontPxForRole(basePx, role), style), role)
}

// hanBitmapAlignStartPx — sole X anchor for Han lines on the 576px canvas (ESC a must not center GS v 0).
func hanBitmapAlignStartPx(textWidthPx int, align byte) int {
	if textWidthPx < 0 {
		textWidthPx = 0
	}
	switch align {
	case 1:
		x := (bitmapTextMaxWidthPx - textWidthPx) / 2
		if x < 0 {
			return 0
		}
		return x
	case 2:
		x := bitmapTextMaxWidthPx - textWidthPx - hanBitmapPadY
		if x < 0 {
			return 0
		}
		return x
	default:
		return hanBitmapPadY
	}
}

// escposDisplayColToPx maps Font A display columns to POS-80 576-dot canvas (12 dots/col).
func escposDisplayColToPx(col int) int {
	if col <= 0 {
		return 0
	}
	return col * bitmapTextMaxWidthPx / escposWidth
}

// stationItemUsesHanCanvas — sole per-dish gate (bitmap escape only): the lead + first-name row
// is one 576px canvas only when the first name needs a raster (it cannot share a Font A row).
// Every other dish stays on Font A; its Han hang lines go through writeStationHangLine.
func stationItemUsesHanCanvas(w *escposWriter, names []string) bool {
	return w.textMode == escposTextBitmap && len(names) > 0 && escposenc.NeedsRaster(names[0])
}

func lastSpaceRunes(b []rune) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == ' ' {
			return i
		}
	}
	return -1
}

func wrapHanTextByPx(s string, maxPx, fontPx int, bold bool) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if maxPx <= 0 {
		return []string{s}
	}
	if measureHanTextWidth(s, fontPx, bold) <= maxPx {
		return []string{s}
	}
	var out []string
	var b []rune
	for _, r := range s {
		try := string(append(b, r))
		if len(b) > 0 && measureHanTextWidth(try, fontPx, bold) > maxPx {
			if idx := lastSpaceRunes(b); idx > 0 {
				out = append(out, string(b[:idx]))
				rest := append([]rune(nil), b[idx+1:]...)
				b = append(rest, r)
				continue
			}
			out = append(out, string(b))
			b = []rune{r}
			continue
		}
		b = append(b, r)
	}
	if len(b) > 0 {
		out = append(out, string(b))
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

func fitHanTextToPx(s string, maxPx, fontPx int, bold bool) string {
	s = strings.TrimSpace(s)
	if s == "" || maxPx <= 0 {
		return s
	}
	if measureHanTextWidth(s, fontPx, bold) <= maxPx {
		return s
	}
	chunks := wrapHanTextByPx(s, maxPx, fontPx, bold)
	if len(chunks) == 0 {
		return s
	}
	return chunks[0]
}

// escposBitmapRaster trims a canvas row to its ink and encodes it via escposbitmap.Raster
// (sole GS v 0 encoder; full-width canvas forces ESC a 0, no trailing LF). nil = render failed.
func escposBitmapRaster(img bitmapTextImage, align byte) []byte {
	return escposbitmap.Raster(escposbitmap.Trim(escposbitmap.Image(img)), align)
}

// escposHanReceiptRow — one 576px raster: items | qty | price (Latin 25/9/14 col bands).
func escposHanReceiptRow(left, mid, right string, basePx int, role hanFontRole, style bitmapTextStyle) []byte {
	img := renderBitmapReceiptRow(left, mid, right, hanFontPxForRole(basePx, role), style)
	return escposBitmapRaster(hanStretchRow(img, role), style.Align)
}

// escposHanPadRow — one 576px raster: left label + right-aligned value (fee lines).
func escposHanPadRow(left, right string, basePx int, role hanFontRole, style bitmapTextStyle) []byte {
	img := renderBitmapPadRow(left, right, hanFontPxForRole(basePx, role), style)
	return escposBitmapRaster(hanStretchRow(img, role), style.Align)
}

// bitmapInkInXBand reports whether any set pixel lies in [startPx, endPx).
func bitmapInkInXBand(img bitmapTextImage, startPx, endPx int) bool {
	if endPx <= startPx {
		return false
	}
	for y := 0; y < img.Height; y++ {
		for x := startPx; x < endPx && x < img.Width; x++ {
			if img.Pixels[y*img.Width+x] != 0 {
				return true
			}
		}
	}
	return false
}

func bitmapInkMaxX(img bitmapTextImage) int {
	maxX := -1
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if img.Pixels[y*img.Width+x] != 0 && x > maxX {
				maxX = x
			}
		}
	}
	return maxX
}

func bitmapInkMinX(img bitmapTextImage) int {
	minX := img.Width
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if img.Pixels[y*img.Width+x] != 0 && x < minX {
				minX = x
			}
		}
	}
	if minX >= img.Width {
		return -1
	}
	return minX
}
