package main

import "strings"

// Vertical padding for Han GS v 0 rows — keep tight vs Font A 1×2 (avoid paper waste).
const (
	hanBitmapPadY      = 1
	hanBitmapHeightPad = 2
)

// Han typography roles — map Latin ESC/POS size tiers to bitmap px (base B = han_bitmap_font_px).
type hanFontRole int

const (
	hanFontSmall hanFontRole = iota // branding / footer ≈ Font A 1×1
	hanFontBody                     // column block + meta ≈ Font A 1×2
	hanFontLarge                    // title / table no. ≈ Font A 2×2
)

func hanFontPxForRole(basePx int, role hanFontRole) int {
	basePx = resolveHanBitmapFontPx(basePx)
	var scaled float64
	switch role {
	case hanFontSmall:
		scaled = float64(basePx) * 0.75
	case hanFontLarge:
		scaled = float64(basePx) * 1.5
	default:
		scaled = float64(basePx)
	}
	px := int(scaled + 0.5)
	if px < bitmapTextMinFontPx {
		return bitmapTextMinFontPx
	}
	if px > bitmapTextMaxFontPx {
		return bitmapTextMaxFontPx
	}
	return px
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

func hanQtyColStartPx() int {
	return escposDisplayColToPx(stationSlipQtyColStart(escposWidth))
}

func hanQtyColWidthPx() int {
	return escposDisplayColToPx(stationSlipQtyColWidth)
}

func hanQtyFieldEndPx() int {
	return escposDisplayColToPx(stationSlipQtyColStart(escposWidth) + stationSlipQtyColWidth)
}

func hanColumnGapPx() int {
	return escposDisplayColToPx(1)
}

// hanQtyTextStartPx — right-align s in the Qty field (Font A column semantics → px).
func hanQtyTextStartPx(s string, fontPx int, bold bool) int {
	if strings.TrimSpace(s) == "" {
		return hanQtyColStartPx()
	}
	fieldEnd := hanQtyFieldEndPx()
	rw := measureHanTextWidth(s, fontPx, bold)
	rx := fieldEnd - rw
	minX := hanQtyColStartPx()
	if rx < minX {
		return minX
	}
	return rx
}

func hanNoteLeftPx() int {
	return escposDisplayColToPx(stationSlipSideMargin)
}

func hanNoteMaxPx() int {
	max := hanQtyColStartPx() - hanNoteLeftPx() - hanColumnGapPx()
	if max < 1 {
		return 1
	}
	return max
}

type hanNoteLine struct {
	x    int
	text string
}

// wrapHanNoteLines — prefix on first line only (same font as body); continuations hang after prefix.
func wrapHanNoteLines(note, prefix string, fontPx int, bold bool) []hanNoteLine {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	leftPx := hanNoteLeftPx()
	prefixW := measureHanTextWidth(prefix, fontPx, bold)
	firstBodyMax := hanNoteMaxPx() - prefixW
	if firstBodyMax < 1 {
		firstBodyMax = 1
	}
	bodyChunks := wrapHanTextByPx(note, firstBodyMax, fontPx, bold)
	if len(bodyChunks) == 0 {
		return []hanNoteLine{{x: leftPx, text: prefix + note}}
	}
	out := []hanNoteLine{{x: leftPx, text: prefix + bodyChunks[0]}}
	consumed := len([]rune(bodyChunks[0]))
	noteRunes := []rune(note)
	if consumed >= len(noteRunes) {
		return out
	}
	remaining := strings.TrimSpace(string(noteRunes[consumed:]))
	if remaining == "" {
		return out
	}
	hangPx := leftPx + prefixW
	contMax := hanQtyColStartPx() - hangPx - hanColumnGapPx()
	if contMax < 1 {
		contMax = 1
	}
	for _, c := range wrapHanTextByPx(remaining, contMax, fontPx, bold) {
		out = append(out, hanNoteLine{x: hangPx, text: c})
	}
	return out
}

type hanColumnRowKind int

const (
	hanColHeader hanColumnRowKind = iota
	hanColItem
)

func hanColumnLeftPx(_ hanColumnRowKind) int {
	return escposDisplayColToPx(stationSlipSideMargin)
}

func hanColumnLabelMaxPx(kind hanColumnRowKind) int {
	left := hanColumnLeftPx(kind)
	gap := escposDisplayColToPx(1)
	max := hanQtyColStartPx() - left - gap
	if max < 1 {
		return 1
	}
	return max
}

// stationSlipColumnBlockUsesHanCanvas — Items/Qty block uses one 576px ruler.
// Sole gate: zh chrome (菜品/数量) OR any line/note with Han. Never mix space-padded
// Han labels via escposBitmapText with Latin Font A qty (qty drifts to paper edge).
func stationSlipColumnBlockUsesHanCanvas(p jobPayload, w *escposWriter) bool {
	if w.textMode != escposTextBitmap {
		return false
	}
	if printLocaleIsZh(p.Locale) {
		return true
	}
	for _, ln := range p.Lines {
		if hasHan(stationSlipItemLabel(ln)) || hasHan(strings.TrimSpace(ln.Note)) {
			return true
		}
	}
	return false
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

func escposBitmapRaster(img bitmapTextImage, align byte) []byte {
	if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
		return nil
	}
	// Full POS-80 canvas: alignment is baked into pixel layout; ESC a on GS v 0 is unreliable.
	if img.Width >= bitmapTextMaxWidthPx {
		align = 0
	}
	widthBytes := (img.Width + 7) / 8
	data := make([]byte, widthBytes*img.Height)
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if img.Pixels[y*img.Width+x] == 0 {
				continue
			}
			data[y*widthBytes+x/8] |= 0x80 >> uint(x%8)
		}
	}
	out := []byte{
		0x1B, 0x61, align,
		0x1D, 0x76, 0x30, 0x00,
		byte(widthBytes & 0xff), byte((widthBytes >> 8) & 0xff),
		byte(img.Height & 0xff), byte((img.Height >> 8) & 0xff),
	}
	out = append(out, data...)
	// GS v 0 already advances by image height — do not append LF (that adds a blank line of paper).
	return out
}

func escposHanColumnRow(left, right string, kind hanColumnRowKind, fontPx int, style bitmapTextStyle) []byte {
	img := renderBitmapColumnRow(left, right, kind, fontPx, style)
	return escposBitmapRaster(img, style.Align)
}

func escposHanLeftRow(text string, leftPx int, fontPx int, style bitmapTextStyle) []byte {
	img := renderBitmapLeftRow(text, leftPx, fontPx, style)
	return escposBitmapRaster(img, style.Align)
}

// escposHanReceiptRow — one 576px raster: items | qty | price (Latin 25/9/14 col bands).
func escposHanReceiptRow(left, mid, right string, fontPx int, style bitmapTextStyle) []byte {
	img := renderBitmapReceiptRow(left, mid, right, fontPx, style)
	return escposBitmapRaster(img, style.Align)
}

// escposHanPadRow — one 576px raster: left label + right-aligned value (fee lines).
func escposHanPadRow(left, right string, fontPx int, style bitmapTextStyle) []byte {
	img := renderBitmapPadRow(left, right, fontPx, style)
	return escposBitmapRaster(img, style.Align)
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
