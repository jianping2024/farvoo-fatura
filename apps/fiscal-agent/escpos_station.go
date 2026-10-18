package main

import (
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/escposenc"
)

// Station slip layout (2026-10-18 redesign):
//
//	        restaurant · BAR           1×1 centered
//	            Pedido                 2×2 bold centered
//	================================================
//	Mesa n.º:001                18:01  2×2 bold
//	Conv.:1                 3 artigos  1×1
//	================================================
//	[ 1× 001 ] Água 500ml              1×2 bold, qty+code reverse (GS B)
//	           中水                     1×2, hangs under name
//	           » 正常冰                  1×2 bold note
//	- - - - - - - - - - - - - - - - - - - - - - - -
const stationSlipNotePrefix = "» "

// stationSlipNames splits a dish into item_code and its print names (print_names first, else
// item_name; legacy jobs without either fall back to the item index label).
func stationSlipNames(ln jobLine) (code string, names []string) {
	code = strings.TrimSpace(ln.ItemCode)
	for _, n := range ln.PrintNames {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if len(names) > 0 {
		return code, names
	}
	name := strings.TrimSpace(ln.ItemName)
	if name == "" {
		name = strings.TrimSpace(ln.DisplayName)
		if code != "" {
			name = strings.TrimSpace(strings.TrimPrefix(name, code+"-"))
		}
	}
	if name == "" && code == "" {
		name = formatItemLabel(ln.ItemIndex, "")
	}
	if name != "" {
		names = []string{name}
	}
	return code, names
}

// stationSlipQtyCodeBlock is the reverse-video lead of each dish row, e.g. " 1× 001 ".
func stationSlipQtyCodeBlock(qty int, code string) string {
	if code == "" {
		return fmt.Sprintf(" %d× ", qty)
	}
	return fmt.Sprintf(" %d× %s ", qty, code)
}

// stationSlipWrap wraps at the last space that fits (Latin words stay whole); Han runs
// and over-long words break per rune like wrapDisplay.
func stationSlipWrap(s string, maxCols int) []string {
	s = strings.TrimSpace(s)
	if s == "" || maxCols <= 0 {
		return nil
	}
	var out []string
	var b []rune
	w := 0
	for _, r := range s {
		cw := displayCols(r)
		if w+cw > maxCols && len(b) > 0 {
			if i := lastSpaceRunes(b); i > 0 && r != ' ' {
				out = append(out, strings.TrimSpace(string(b[:i])))
				b = append([]rune(nil), b[i+1:]...)
			} else {
				out = append(out, strings.TrimSpace(string(b)))
				b = b[:0]
			}
			w = displayWidth(string(b))
			if len(b) == 0 && r == ' ' {
				continue
			}
		}
		b = append(b, r)
		w += cw
	}
	if t := strings.TrimSpace(string(b)); t != "" {
		out = append(out, t)
	}
	return out
}

func stationSlipTotalQty(lines []jobLine) int {
	n := 0
	for _, ln := range lines {
		if ln.Qty > 0 {
			n += ln.Qty
		} else {
			n++
		}
	}
	return n
}

func stationSlipItemCount(lab ticketLabels, n int) string {
	if n == 1 {
		return lab.itemCountOne
	}
	return fmt.Sprintf(lab.itemCountMany, n)
}

// stationSlipClock extracts HH:MM from order_time ("2026-10-18 18:01" or RFC3339).
func stationSlipClock(orderAt string) string {
	s := strings.TrimSpace(orderAt)
	if i := strings.LastIndexAny(s, " T"); i >= 0 {
		s = s[i+1:]
	}
	if len(s) >= 5 && s[2] == ':' {
		return s[:5]
	}
	return ""
}

func stationSlipOrderAt(p jobPayload) string {
	if s := strings.TrimSpace(p.OrderTime); s != "" {
		return s
	}
	return nowLocal()
}

func (w *escposWriter) reverse(on bool) {
	n := byte(0)
	if on {
		n = 1
	}
	w.content.Write([]byte{0x1D, 0x42, n})
}

// writeStationSlipHeader — branding · station, title, table + order clock, guest + item count.
func (w *escposWriter) writeStationSlipHeader(p jobPayload, lab ticketLabels, title string) {
	w.align(1)
	w.writeBody1x1()
	brand := ticketBrandingWord(w.printLocale)
	if st := p.stationName(); st != "" {
		brand += " · " + st
	}
	w.text(brand)
	w.lf()
	w.writeMasthead2x2Bold()
	w.text(title)
	w.lf()
	w.separator('=')

	w.writeStationTableLine(p.tableNoLabel(lab), stationSlipClock(stationSlipOrderAt(p)))

	w.writeBody1x1()
	guest := ""
	if p.GuestCount > 0 {
		guest = fmt.Sprintf("%s:%d", lab.guest, p.GuestCount)
	}
	w.writeReceiptPadLine(guest, stationSlipItemCount(lab, stationSlipTotalQty(p.Lines)))
	w.separator('=')
}

// writeStationTableLine — table no. left, order clock right, both 2×2 bold (24 cols).
func (w *escposWriter) writeStationTableLine(table, clock string) {
	if table == "" && clock == "" {
		return
	}
	w.align(0)
	w.writeMasthead2x2Bold()
	if w.textMode == escposTextBitmap && hasHan(table) {
		fontPx := hanFontPxForRole(w.hanFontPx, hanFontLarge)
		w.content.Write(escposHanPadRow(table, clock, fontPx, bitmapTextStyle{Bold: true}))
		w.lastTextBitmap = true
		w.lf()
		return
	}
	w.text(escposPadLine(table, clock, escposWidth/2))
	w.lf()
}

// writeStationMenuLines — optional category headers, then one dish block per line.
func (w *escposWriter) writeStationMenuLines(p jobPayload, lines []jobLine) {
	showGroup := p.stationSlipShowCategoryGroup()
	canvas := stationSlipColumnBlockUsesHanCanvas(p, w)
	lastGroupHeader := ""
	for i, ln := range sortStationMenuLines(lines) {
		groupHeader := strings.TrimSpace(ln.CategoryGroupHeader)
		if showGroup && groupHeader != "" && groupHeader != lastGroupHeader {
			if i > 0 {
				w.separator('-')
			}
			w.align(0)
			w.writeBody1x1()
			w.bold(true)
			w.text(groupHeader)
			w.lf()
			w.separator('-')
			lastGroupHeader = groupHeader
		} else if i > 0 {
			w.writeStationItemDivider()
		}

		qty := ln.Qty
		if qty <= 0 {
			qty = 1
		}
		code, names := stationSlipNames(ln)
		block := stationSlipQtyCodeBlock(qty, code)
		note := strings.TrimSpace(ln.Note)
		if canvas {
			w.writeHanStationItem(block, names, note)
		} else {
			w.writeStationItem(block, names, note)
		}
	}
}

func (w *escposWriter) writeStationItemDivider() {
	w.align(0)
	w.size(false, false)
	w.bold(false)
	w.text(strings.TrimRight(strings.Repeat("- ", escposWidth/2), " "))
	w.lf()
}

// writeStationItem — Font A / firmware Han path: GS B block, name on the same row,
// extra names and the note hang under the name column.
func (w *escposWriter) writeStationItem(block string, names []string, note string) {
	indent := displayWidth(block) + 1
	maxW := escposWidth - indent
	pad := strings.Repeat(" ", indent)
	first := ""
	if len(names) > 0 {
		first = names[0]
	}
	chunks := stationSlipWrap(first, maxW)

	w.align(0)
	w.writeBody1x2()
	w.bold(true)
	w.reverse(true)
	w.text(block)
	w.reverse(false)
	if len(chunks) > 0 {
		// A raster segment is a full-width GS v 0 image — it cannot share the block's row.
		if w.textMode == escposTextAuto && escposenc.LinePathFor(chunks[0]) == escposenc.PathRaster {
			w.lf()
			w.text(pad + chunks[0])
		} else {
			w.text(" " + chunks[0])
		}
		chunks = chunks[1:]
	}
	w.lf()
	for _, c := range chunks {
		w.text(pad + c)
		w.lf()
	}

	w.bold(false)
	for _, name := range names[min(1, len(names)):] {
		for _, c := range stationSlipWrap(name, maxW) {
			w.text(pad + c)
			w.lf()
		}
	}
	if note != "" {
		w.bold(true)
		for _, c := range stationSlipWrap(stationSlipNotePrefix+note, maxW) {
			w.text(pad + c)
			w.lf()
		}
		w.bold(false)
	}
}

// writeHanStationItem — bitmap escape: same layout on the 576px canvas; the block is
// reversed by inverting its pixel band (GS B does not apply to GS v 0 images).
func (w *escposWriter) writeHanStationItem(block string, names []string, note string) {
	fontPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
	bold := bitmapTextStyle{Bold: true}
	spacePx := measureHanTextWidth(" ", fontPx, true)
	blockText := strings.TrimSpace(block)
	blockEnd := spacePx*2 + measureHanTextWidth(blockText, fontPx, true)
	nameX := blockEnd + spacePx
	maxPx := bitmapTextMaxWidthPx - nameX

	first := ""
	if len(names) > 0 {
		first = names[0]
	}
	chunks := wrapHanTextByPx(first, maxPx, fontPx, true)
	row := renderBitmapLeftRow(blockText, spacePx, fontPx, bold)
	if len(chunks) > 0 {
		row = mergeBitmapRows(row, renderBitmapLeftRow(chunks[0], nameX, fontPx, bold))
		chunks = chunks[1:]
	}
	invertBitmapBand(row, 0, blockEnd)

	w.align(0)
	w.writeBody1x2()
	w.writeHanRaster(row)
	for _, c := range chunks {
		w.writeHanRaster(renderBitmapLeftRow(c, nameX, fontPx, bold))
	}
	for _, name := range names[min(1, len(names)):] {
		for _, c := range wrapHanTextByPx(name, maxPx, fontPx, false) {
			w.writeHanRaster(renderBitmapLeftRow(c, nameX, fontPx, bitmapTextStyle{}))
		}
	}
	if note != "" {
		for _, c := range wrapHanTextByPx(stationSlipNotePrefix+note, maxPx, fontPx, true) {
			w.writeHanRaster(renderBitmapLeftRow(c, nameX, fontPx, bold))
		}
	}
}

func (w *escposWriter) writeHanRaster(img bitmapTextImage) {
	w.content.Write(escposBitmapRaster(img, 0))
	w.lastTextBitmap = true
}

// writeStationSlipFooter — order date-time left, printed-by right (1×1).
func (w *escposWriter) writeStationSlipFooter(p jobPayload, lab ticketLabels) {
	w.separator('-')
	w.writeBody1x1()
	w.writeReceiptPadLine(stationSlipOrderAt(p), fmt.Sprintf("%s:%s", lab.printedBy, ticketBrandingWord(w.printLocale)))
}

// mergeBitmapRows ORs two full-width rows (top-aligned) into one canvas.
func mergeBitmapRows(a, b bitmapTextImage) bitmapTextImage {
	if a.Width == 0 {
		return b
	}
	if b.Width == 0 {
		return a
	}
	h := max(a.Height, b.Height)
	out := bitmapTextImage{Width: bitmapTextMaxWidthPx, Height: h, Pixels: make([]byte, bitmapTextMaxWidthPx*h)}
	for _, img := range []bitmapTextImage{a, b} {
		for y := 0; y < img.Height; y++ {
			for x := 0; x < img.Width && x < out.Width; x++ {
				if img.Pixels[y*img.Width+x] != 0 {
					out.Pixels[y*out.Width+x] = 1
				}
			}
		}
	}
	return out
}

// invertBitmapBand flips ink in x ∈ [startPx, endPx) over the full row height.
func invertBitmapBand(img bitmapTextImage, startPx, endPx int) {
	endPx = min(endPx, img.Width)
	for y := 0; y < img.Height; y++ {
		for x := max(startPx, 0); x < endPx; x++ {
			i := y*img.Width + x
			if img.Pixels[i] == 0 {
				img.Pixels[i] = 1
			} else {
				img.Pixels[i] = 0
			}
		}
	}
}
