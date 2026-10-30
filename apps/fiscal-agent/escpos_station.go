package main

import (
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/escposenc"
)

// Station slip layout (2026-10-18 redesign):
//
//	        restaurant · BAR           1×1 centered
//	            Pedido                 2×2 bold centered (Chinese: not bold)
//	================================================
//	Mesa 001                    18:01  2×2 bold (Chinese: not bold)
//	Conv.:1                 3 artigos  1×1
//	================================================
//	       (BEBIDAS)                   optional 1×2 centered; show_category_group
//	1x 001 Água 500ml                   qty+code 2×2; first name 1×2 (default) or 2×2
//	       中水                         other print_names 1×2, hang under name
//	       » 正常冰                      note 1×2
//	- - - - - - - - - - - - - - - - - - - - - - - -
//
// First-name size is agent config station_pt_name_size (1x2|2x2). Lead / secondary / note unchanged.
// Dish rows are never bold or reversed: ESC E / GS B smeared on store printers (0.5.39/0.5.40).
// Category headers print as-is from category_group_header (once per group), 1×2 centered; no separators.
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

// stationSlipQtyCodeLead — ONLY writer for dish qty×code lead (ASCII "x"; Unicode ×
// is blank on some store Font A glyphs at 2×2). e.g. "1x 001".
func stationSlipQtyCodeLead(qty int, code string) string {
	if code == "" {
		return fmt.Sprintf("%dx", qty)
	}
	return fmt.Sprintf("%dx %s", qty, code)
}

// stationSlipNameIndent — Font A column where names/notes start: 2×2 lead (2 cols per char) + 1×2 space.
func stationSlipNameIndent(lead string) int {
	return displayWidth(lead)*2 + 1
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
	w.writeStationHeading2x2(title)
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

// writeStationHeading2x2 — 2×2 for the title / table row; bold only for Latin. Chinese headings
// stay regular so they match the unbolded dish rows (bold YaHei raster looked heavy, 0.5.45).
func (w *escposWriter) writeStationHeading2x2(s string) {
	w.size(true, true)
	w.bold(!hasHan(s))
}

// writeStationTableLine — table no. left, order clock right, both 2×2 (24 cols); bold unless Chinese.
func (w *escposWriter) writeStationTableLine(table, clock string) {
	if table == "" && clock == "" {
		return
	}
	w.align(0)
	w.writeStationHeading2x2(table)
	if w.textMode == escposTextBitmap && hasHan(table) {
		w.writeRasterOr(escposHanPadRow(table, clock, w.hanFontPx, hanFontLarge, bitmapTextStyle{}),
			escposPadLine(table, clock, escposWidth/2))
		w.lf()
		return
	}
	w.text(escposPadLine(table, clock, escposWidth/2))
	w.lf()
}

// writeStationCategoryHeader — 唯一写法：出品联大类行版式（1×2 居中、原文、无上下分隔线）。
func (w *escposWriter) writeStationCategoryHeader(header string) {
	w.align(1)
	w.writeBody1x2()
	w.text(header)
	w.lf()
	w.align(0)
}

// writeStationMenuLines — optional category headers (once per group), then one dish block per line.
// Whether to show: only p.stationSlipShowCategoryGroup(). Text: only ln.CategoryGroupHeader as-is.
// Chrome: only writeStationCategoryHeader.
func (w *escposWriter) writeStationMenuLines(p jobPayload, lines []jobLine) {
	showGroup := p.stationSlipShowCategoryGroup()
	lastGroupHeader := ""
	for i, ln := range sortStationMenuLines(lines) {
		groupHeader := strings.TrimSpace(ln.CategoryGroupHeader)
		if showGroup && groupHeader != "" && groupHeader != lastGroupHeader {
			w.writeStationCategoryHeader(groupHeader)
			lastGroupHeader = groupHeader
		} else if i > 0 {
			w.writeStationItemDivider()
		}

		qty := ln.Qty
		if qty <= 0 {
			qty = 1
		}
		code, names := stationSlipNames(ln)
		lead := stationSlipQtyCodeLead(qty, code)
		note := strings.TrimSpace(ln.Note)
		if stationItemUsesHanCanvas(w, names) {
			w.writeHanStationItem(lead, names, note)
		} else {
			w.writeStationItem(lead, names, note)
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

// writeStationItem — Font A / firmware Han path: 2×2 lead; first name 1×2 (default) or 2×2
// on the same row (station_pt_name_size); other names and the note (both 1×2) hang under the name.
func (w *escposWriter) writeStationItem(lead string, names []string, note string) {
	first2x2 := w.stationFirstName2x2
	maxFirst := stationSlipFirstNameMaxCols(lead, first2x2)
	firstPad := stationSlipFirstNameHangPad(lead, first2x2)
	hangIndent := stationSlipNameHangIndent(lead, first2x2)
	hangPad := strings.Repeat(" ", hangIndent)
	// Secondary / note wrap at 1×2 over the hang indent width.
	maxHang := escposWidth - hangIndent

	first := ""
	if len(names) > 0 {
		first = names[0]
	}
	chunks := stationSlipWrap(first, maxFirst)

	w.align(0)
	w.bold(false)
	w.underline(false)
	w.size(true, true)
	w.text(lead)
	if !first2x2 {
		w.size(false, true)
	}
	if len(chunks) > 0 {
		// A raster segment is a full-width GS v 0 image — it cannot share the lead's row.
		if w.textMode == escposTextHanFirmware && escposenc.LinePathFor(chunks[0]) == escposenc.PathRaster {
			w.lf()
			w.text(firstPad + chunks[0])
		} else {
			w.text(" " + chunks[0])
		}
		chunks = chunks[1:]
	}
	w.lf()
	for _, c := range chunks {
		if first2x2 {
			w.size(true, true)
		} else {
			w.writeBody1x2()
		}
		w.writeStationHangLine(firstPad, c)
	}

	if len(names) > 1 {
		w.writeBody1x2()
		for _, name := range names[1:] {
			for _, c := range stationSlipWrap(name, maxHang) {
				w.writeStationHangLine(hangPad, c)
			}
		}
	}
	if note != "" {
		w.writeBody1x2()
		for _, c := range stationSlipWrap(stationSlipNotePrefix+note, maxHang) {
			w.writeStationHangLine(hangPad, c)
		}
	}
}

// writeStationHangLine — one row under the Font A name column. On the bitmap path a row that
// needs a raster (Han…) is a GS v 0 image anchored at that column's dot X, at the current GS !
// size, so it lines up with the Font A rows around it (space-padded raster text would drift).
func (w *escposWriter) writeStationHangLine(pad, s string) {
	if w.textMode == escposTextBitmap && escposenc.NeedsRaster(s) {
		x := escposDisplayColToPx(len(pad))
		w.writeHanRaster(renderHanRoleRow(s, x, w.hanFontPx, w.hanSizeRole(), bitmapTextStyle{}), pad+s)
		return
	}
	w.text(pad + s)
	w.lf()
}

// writeHanStationItem — bitmap path: same layout on the 576px canvas as writeStationItem
// (lead 2×2; first name 1×2 or 2×2; other names / note 1×2); lead row bottom-aligned.
func (w *escposWriter) writeHanStationItem(lead string, names []string, note string) {
	firstRole := hanFontBody
	if w.stationFirstName2x2 {
		firstRole = hanFontLarge
	}
	firstPx := hanFontPxForRole(w.hanFontPx, firstRole)
	bodyPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
	style := bitmapTextStyle{}
	hangIndent := stationSlipNameHangIndent(lead, w.stationFirstName2x2)
	hangPad := strings.Repeat(" ", hangIndent)
	firstPad := stationSlipFirstNameHangPad(lead, w.stationFirstName2x2)
	nameX := hanStationNameX(lead, w.hanFontPx)
	maxPx := bitmapTextMaxWidthPx - nameX

	first := ""
	if len(names) > 0 {
		first = names[0]
	}
	chunks := wrapHanTextByPx(first, maxPx, firstPx, false)
	row := renderHanRoleRow(lead, 0, w.hanFontPx, hanFontLarge, style)
	rowText := lead
	if len(chunks) > 0 {
		row = mergeBitmapRows(row, renderHanRoleRow(chunks[0], nameX, w.hanFontPx, firstRole, style))
		rowText += " " + chunks[0]
		chunks = chunks[1:]
	}

	w.align(0)
	w.writeBody1x2()
	w.writeHanRaster(row, rowText)
	for _, c := range chunks {
		w.writeHanRaster(renderHanRoleRow(c, nameX, w.hanFontPx, firstRole, style), firstPad+c)
	}
	if len(names) > 1 {
		for _, name := range names[1:] {
			for _, c := range wrapHanTextByPx(name, maxPx, bodyPx, false) {
				w.writeHanRaster(renderHanRoleRow(c, nameX, w.hanFontPx, hanFontBody, style), hangPad+c)
			}
		}
	}
	if note != "" {
		for _, c := range wrapHanTextByPx(stationSlipNotePrefix+note, maxPx, bodyPx, false) {
			w.writeHanRaster(renderHanRoleRow(c, nameX, w.hanFontPx, hanFontBody, style), hangPad+c)
		}
	}
}

// hanStationNameGapPx — blank space between the bitmap lead and the name column
// (one 2×2 space: 24 dots).
const hanStationNameGapPx = 24

// hanStationNameX — bitmap name column: the Font A indent, pushed right when the TrueType
// lead (wider than Font A 2×2 cells) would touch the name.
func hanStationNameX(lead string, basePx int) int {
	x := escposDisplayColToPx(stationSlipNameIndent(lead))
	if min := measureHanTextWidth(lead, hanFontPxForRole(basePx, hanFontLarge), false) + hanStationNameGapPx; min > x {
		x = min
	}
	return x
}

func (w *escposWriter) writeHanRaster(img bitmapTextImage, fallback string) {
	w.writeRasterOr(escposBitmapRaster(img, 0), fallback)
}

// writeStationSlipFooter — order date-time left, printed-by right (1×1).
func (w *escposWriter) writeStationSlipFooter(p jobPayload, lab ticketLabels) {
	w.separator('-')
	w.writeBody1x1()
	w.writeReceiptPadLine(stationSlipOrderAt(p), fmt.Sprintf("%s:%s", lab.printedBy, ticketBrandingWord(w.printLocale)))
}

// mergeBitmapRows ORs two full-width rows into one canvas, bottom-aligned (like mixed GS ! sizes).
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
		dy := h - img.Height
		for y := 0; y < img.Height; y++ {
			for x := 0; x < img.Width && x < out.Width; x++ {
				if img.Pixels[y*img.Width+x] != 0 {
					out.Pixels[(y+dy)*out.Width+x] = 1
				}
			}
		}
	}
	return out
}
