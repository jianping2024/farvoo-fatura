package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/escposenc"
)

// 80mm paper ≈ 48 chars (Font A). Layout follows reference thermal receipts.
const escposWidth = 48

// Station slip menu body column model (Agent-side layout only).
// Left/right symmetric: stationSlipSideMargin cols; Items, item rows, and notes share the same left edge.
const (
	stationSlipSideMargin  = 4
	stationSlipQtyColWidth = 4 // fits "Qty" / 1–9999; frees item+note band toward right
)

// Top: no extra LF — most printers already feed after ESC @ init.
// Bottom: 2× single-height "restaurant" row before cut (visible pad only).
const (
	escposTopMarginLines    = 0
	escposBottomMarginLines = 2
)

// Minimal feed before blade after bottom pad (not part of visible margin).
const (
	escposCutFeedDotsDefault = 0x18
	escposCutFeedDotsTall    = 0x38 // tickets with double-height blocks
)

type ticketLabels struct {
	connectionTest string
	guestOrder     string
	receipt        string
	preBill        string
	tableNo        string
	guest          string
	items          string
	qty            string
	originalPrice  string
	feeDetails     string
	originalTotal  string
	subtotal       string
	amountDue      string
	orderTime      string
	printedBy      string
	printTime      string
	printedByVal   string
	orderedBy      string
	amountPaid     string
	station        string
	itemNote       string // underlined guest note label, e.g. "备注: " / "Note: " / "Observação: "
}

func labelsFor(locale string) ticketLabels {
	switch locale {
	case "zh":
		return ticketLabels{
			connectionTest: "打印测试",
			guestOrder:     "出菜单",
			receipt:        "收据",
			preBill:        "预结单",
			tableNo:        "桌号",
			guest:          "人数",
			items:          "菜品",
			qty:            "数量",
			originalPrice:  "原价",
			feeDetails:     "费用明细",
			originalTotal:  "原价合计",
			subtotal:       "小计",
			amountDue:      "应付",
			orderTime:      "下单时间",
			printedBy:      "打印",
			printTime:      "打印时间",
			printedByVal:   "系统",
			orderedBy:      "下单方",
			amountPaid:     "实付",
			station:        "档口",
			itemNote:       "备注: ",
		}
	case "en":
		return ticketLabels{
			connectionTest: "PRINT TEST",
			guestOrder:     "Guest Order",
			receipt:        "Receipt",
			preBill:        "Pre-Bill",
			tableNo:        "Table No.",
			guest:          "Guest",
			items:          "Items",
			qty:            "Qty",
			originalPrice:  "Pri",
			feeDetails:     "Fee Details",
			originalTotal:  "Original price",
			subtotal:       "Subtotal",
			amountDue:      "Amount Due",
			orderTime:      "Order Time",
			printedBy:      "Printed By",
			printTime:      "Print Time",
			printedByVal:   "Customer/Merchant",
			orderedBy:      "Ordered By",
			amountPaid:     "Amount Paid",
			station:        "Station",
			itemNote:       "Note: ",
		}
	default: // pt (pt-PT semantics)
		return ticketLabels{
			connectionTest: "TESTE IMPRESSÃO",
			guestOrder:     "Pedido",
			receipt:        "Recibo",
			preBill:        "Pré-conta",
			tableNo:        "Mesa n.º",
			guest:          "Conv.",
			items:          "Artigos",
			qty:            "Qtd",
			originalPrice:  "Preço",
			feeDetails:     "Detalhe taxas",
			originalTotal:  "Preço original",
			subtotal:       "Subtotal",
			amountDue:      "A pagar",
			orderTime:      "Hora pedido",
			printedBy:      "Impresso por",
			printTime:      "Hora impressão",
			printedByVal:   "Cliente/Estabelecimento",
			orderedBy:      "Pedido por",
			amountPaid:     "Valor pago",
			station:        "Estação",
			itemNote:       "Observação: ",
		}
	}
}

type stationSlipOptions struct {
	ShowCategoryGroup bool `json:"show_category_group"`
}

type jobLine struct {
	ItemIndex           int     `json:"item_index"`
	ItemCode            string  `json:"item_code"`
	ItemName            string  `json:"item_name"`
	DisplayName         string  `json:"display_name"`
	Qty                 int     `json:"qty"`
	ShareQtyLabel       string  `json:"share_qty_label"`
	Note                string  `json:"note"`
	UnitPrice           float64 `json:"unit_price"`
	CategoryGroupSort   int     `json:"category_group_sort"`
	CategoryGroupHeader string  `json:"category_group_header"`
}

type jobPayload struct {
	Locale               string              `json:"locale"`
	ConnectionTest       bool                `json:"connection_test"`
	RestaurantName       string              `json:"restaurant_name"`
	TableDisplayName     string              `json:"display_name"`
	TableID              string              `json:"table_id"`
	GuestCount           int                 `json:"guest_count"`
	StationDisplayNamePt string              `json:"station_display_name_pt"`
	StationDisplayNameEn string              `json:"station_display_name_en"`
	StationDisplayNameZh string              `json:"station_display_name_zh"`
	StationSlipOptions   *stationSlipOptions `json:"station_slip_options"`
	HanBitmapFontPx      int                 `json:"han_bitmap_font_px"`
	Lines                []jobLine           `json:"lines"`
	Subtotal             float64             `json:"subtotal"`
	AmountDue            float64             `json:"amount_due"`
	AmountPaid           float64             `json:"amount_paid"`
	PaymentMethod        string              `json:"payment_method"`
	OrderedBy            string              `json:"ordered_by"`
	OrderTime            string              `json:"order_time"`
	PrintTime            string              `json:"print_time"`
	// pre_bill | checkout_bill | split_payment | final (empty → final on order_receipt)
	ReceiptVariant string `json:"receipt_variant"`
	PayerName      string `json:"payer_name"`
}

func parseJobPayload(job printJob) jobPayload {
	var p jobPayload
	_ = json.Unmarshal(job.Payload, &p)
	p.Locale = normalizePrintLocale(p.Locale)
	return p
}

// formatTableNoLabel prints table labels from JSON (string or legacy numeric).
func formatTableNoLabel(lab ticketLabels, tableNo string) string {
	t := strings.TrimSpace(tableNo)
	if t == "" {
		return ""
	}
	if len(t) <= 2 {
		if n, err := strconv.Atoi(t); err == nil && strconv.Itoa(n) == t {
			return fmt.Sprintf("%s:%02d", lab.tableNo, n)
		}
	}
	return fmt.Sprintf("%s:%s", lab.tableNo, t)
}

func (p jobPayload) tableNoLabel(lab ticketLabels) string {
	return formatTableNoLabel(lab, p.TableDisplayName)
}

// receiptHeaderTitle — receipts use English labels (lab); dish lines keep menu display names.
func receiptHeaderTitle(variant string, lab ticketLabels) string {
	if variant == "pre_bill" {
		if t := strings.TrimSpace(lab.preBill); t != "" {
			return t
		}
	}
	return lab.receipt
}

func (p jobPayload) stationName() string {
	switch p.Locale {
	case "zh":
		if s := strings.TrimSpace(p.StationDisplayNameZh); s != "" {
			return s
		}
	case "en":
		if s := strings.TrimSpace(p.StationDisplayNameEn); s != "" {
			return s
		}
	}
	if s := strings.TrimSpace(p.StationDisplayNamePt); s != "" {
		return s
	}
	return strings.TrimSpace(p.StationDisplayNameEn)
}

func (p jobPayload) venueName() string {
	if s := strings.TrimSpace(p.RestaurantName); s != "" {
		return s
	}
	return "restaurant"
}

func (p jobPayload) stationSlipShowCategoryGroup() bool {
	return p.StationSlipOptions != nil && p.StationSlipOptions.ShowCategoryGroup
}

type escposWriter struct {
	prefix          []byte
	content         bytes.Buffer
	textMode        escposTextMode
	printLocale     string
	hanFontPx       int
	hanRole         hanFontRole
	hadDoubleHeight bool
	alignMode       byte
	boldOn          bool
	underlineOn     bool
	doubleW         bool
	doubleH         bool
	lastTextBitmap  bool
}

func writeMarginLines(b *bytes.Buffer, lines int) {
	for i := 0; i < lines; i++ {
		b.WriteByte('\n')
	}
}

func cutFeedDots(hadDoubleHeight bool) byte {
	if hadDoubleHeight {
		return escposCutFeedDotsTall
	}
	return escposCutFeedDotsDefault
}

func newEscpos() *escposWriter {
	w := &escposWriter{textMode: escposTextLatin, hanFontPx: bitmapTextDefaultFontPx}
	w.init()
	return w
}

func newEscposForStationTicket(p jobPayload) *escposWriter {
	w := newEscpos()
	w.printLocale = normalizePrintLocale(p.Locale)
	w.hanFontPx = resolveHanBitmapFontPx(p.HanBitmapFontPx)
	w.applyTextMode(textModeForConfiguredChinese(stationTicketNeedsBitmap(p)))
	return w
}

func newEscposForReceiptTicket(p jobPayload) *escposWriter {
	w := newEscpos()
	w.printLocale = normalizePrintLocale(p.Locale)
	w.hanFontPx = resolveHanBitmapFontPx(p.HanBitmapFontPx)
	w.applyTextMode(textModeForConfiguredChinese(receiptTicketNeedsBitmap(p)))
	return w
}

func newEscposForConnectionTest(p jobPayload) *escposWriter {
	w := newEscpos()
	w.printLocale = normalizePrintLocale(p.Locale)
	w.hanFontPx = resolveHanBitmapFontPx(p.HanBitmapFontPx)
	w.applyTextMode(textModeForConfiguredChinese(connectionTestNeedsBitmap(p)))
	return w
}

func (w *escposWriter) init() { w.prefix = append(w.prefix, 0x1B, 0x40) }

func (w *escposWriter) applyTextMode(mode escposTextMode) {
	w.textMode = mode
	switch mode {
	case escposTextUTF8:
		w.enableUTF8()
	default:
		w.enableLatin()
	}
}

// enableLatin selects WPC1252 (covers Portuguese accents on most 80mm printers).
func (w *escposWriter) enableLatin() {
	w.prefix = append(w.prefix, escposenc.SelectCodeTable(escposenc.CodeTableWPC1252)...)
}

func (w *escposWriter) enableUTF8() {
	w.prefix = append(w.prefix, 0x1B, 0x39, 0x01)
}

func (w *escposWriter) align(mode byte) {
	w.alignMode = mode
	w.content.Write([]byte{0x1B, 0x61, mode})
}

func (w *escposWriter) bold(on bool) {
	n := byte(0)
	if on {
		n = 1
	}
	w.boldOn = on
	w.content.Write([]byte{0x1B, 0x45, n})
}

func (w *escposWriter) underline(on bool) {
	n := byte(0)
	if on {
		n = 1
	}
	w.underlineOn = on
	w.content.Write([]byte{0x1B, 0x2D, n})
}

func (w *escposWriter) size(doubleW, doubleH bool) {
	if doubleH {
		w.hadDoubleHeight = true
	}
	n := byte(0)
	if doubleH {
		n |= 0x01
	}
	if doubleW {
		n |= 0x10
	}
	w.doubleW = doubleW
	w.doubleH = doubleH
	w.content.Write([]byte{0x1D, 0x21, n})
}

func (w *escposWriter) text(s string) {
	if w.textMode == escposTextBitmap && needsBitmapText(s) {
		fontPx := hanFontPxForRole(w.hanFontPx, w.hanRole)
		w.content.Write(escposBitmapText(s, bitmapTextStyle{
			Align:     w.alignMode,
			Bold:      w.boldOn,
			Underline: w.underlineOn,
			DoubleW:   w.doubleW,
			DoubleH:   w.doubleH,
		}, fontPx))
		w.lastTextBitmap = true
		return
	}
	if w.textMode == escposTextUTF8 {
		w.content.Write([]byte(s))
		w.lastTextBitmap = false
		return
	}
	w.content.Write(encodeWindows1252(s))
	w.lastTextBitmap = false
}

func (w *escposWriter) lf() {
	if w.lastTextBitmap {
		w.lastTextBitmap = false
		return
	}
	w.content.WriteByte('\n')
}

func (w *escposWriter) writeResetPrintMode(out *bytes.Buffer) {
	out.Write([]byte{0x1B, 0x61, 0})
	out.Write([]byte{0x1D, 0x21, 0})
	out.Write([]byte{0x1B, 0x45, 0})
	out.Write([]byte{0x1B, 0x2D, 0})
	out.Write([]byte{0x1B, 0x32}) // ESC 2 — default line spacing
}

// finish assembles init + top pad + body + bottom pad (+ cut feed when cut is true).
func (w *escposWriter) finish(cut bool) []byte {
	var out bytes.Buffer
	out.Write(w.prefix)
	writeMarginLines(&out, escposTopMarginLines)
	out.Write(w.content.Bytes())
	if cut {
		w.writeResetPrintMode(&out)
		writeMarginLines(&out, escposBottomMarginLines)
		out.Write([]byte{0x1D, 0x56, 0x42, cutFeedDots(w.hadDoubleHeight)})
	}
	return out.Bytes()
}
func (w *escposWriter) separator(ch rune) {
	w.align(0)
	w.size(false, false)
	w.bold(false)
	line := strings.Repeat(string(ch), escposWidth)
	w.text(line)
	w.lf()
}

const (
	escposColItems = 25 // leaves 9 for Qty (A999-C999) and 14 for price column
	escposColQty   = 9
	escposColPrice = escposWidth - escposColItems - escposColQty
)

func padField(s string, width int, alignRight bool) string {
	s = truncateDisplay(s, width)
	gap := width - displayWidth(s)
	if gap < 0 {
		gap = 0
	}
	if alignRight {
		return strings.Repeat(" ", gap) + s
	}
	return s + strings.Repeat(" ", gap)
}

func padFieldCenter(s string, width int) string {
	s = truncateDisplay(s, width)
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
}

func padFieldRight(s string, width int) string {
	s = truncateDisplay(s, width)
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}

func escposThreeColLine(left, mid, right string) string {
	return padField(left, escposColItems, false) +
		padFieldCenter(mid, escposColQty) +
		padField(right, escposColPrice, true)
}

func (w *escposWriter) rightLine(s string, bold bool) {
	w.align(2)
	w.size(false, false)
	w.bold(bold)
	w.text(s)
	w.lf()
	w.bold(false)
	w.align(0)
}

func (w *escposWriter) writeReceiptMenuBodyLine(left, mid, right string, bold bool) {
	if w.textMode == escposTextBitmap && (hasHan(left) || hasHan(mid) || hasHan(right)) {
		fontPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
		w.content.Write(escposHanReceiptRow(left, mid, right, fontPx, bitmapTextStyle{
			Align: w.alignMode,
			Bold:  bold,
		}))
		w.lastTextBitmap = true
		w.lf()
		return
	}
	if bold {
		w.writeBody1x2Bold()
	} else {
		w.writeBody1x2()
	}
	w.text(escposThreeColLine(left, mid, right))
	w.lf()
}

func (w *escposWriter) writeReceiptPadLine(left, right string) {
	if w.textMode == escposTextBitmap && (hasHan(left) || hasHan(right)) {
		fontPx := hanFontPxForRole(w.hanFontPx, w.hanRole)
		w.content.Write(escposHanPadRow(left, right, fontPx, bitmapTextStyle{
			Align: w.alignMode,
			Bold:  w.boldOn,
		}))
		w.lastTextBitmap = true
		w.lf()
		return
	}
	w.text(escposPadLine(left, right, escposWidth))
	w.lf()
}

func (w *escposWriter) writeReceiptAmountDueLine(s string) {
	w.align(2)
	w.writeBody1x2Bold()
	w.text(s)
	w.lf()
	w.writeBody1x1()
	w.align(0)
}

func stationSlipQtyColStart(width int) int {
	return width - stationSlipSideMargin - stationSlipQtyColWidth
}

func stationSlipHeaderItemsMaxWidth(width int) int {
	return stationSlipQtyColStart(width) - stationSlipSideMargin
}

func stationSlipItemMaxWidth(width int) int {
	return stationSlipQtyColStart(width) - stationSlipSideMargin
}

func placeRunesAt(buf []rune, start int, r []rune) {
	for i, c := range r {
		pos := start + i
		if pos >= 0 && pos < len(buf) {
			buf[pos] = c
		}
	}
}

func stationSlipColumnHeaderLine(itemsLabel, qtyLabel string, width int) string {
	items := []rune(truncateDisplay(itemsLabel, stationSlipHeaderItemsMaxWidth(width)))
	qty := []rune(truncateDisplay(padFieldRight(qtyLabel, stationSlipQtyColWidth), stationSlipQtyColWidth))
	buf := make([]rune, width)
	for i := range buf {
		buf[i] = ' '
	}
	placeRunesAt(buf, stationSlipSideMargin, items)
	placeRunesAt(buf, stationSlipQtyColStart(width), qty)
	return string(buf)
}

// stationSlipItemLine — Latin Font A column layout only (48 cols). Han uses escposHanColumnRow.
func stationSlipItemLine(leftLabel, qtyStr string, width int) string {
	qtyStart := stationSlipQtyColStart(width)
	qtyField := truncateDisplay(padFieldRight(strings.TrimSpace(qtyStr), stationSlipQtyColWidth), stationSlipQtyColWidth)
	left := strings.Repeat(" ", stationSlipSideMargin) + leftLabel
	pad := qtyStart - displayWidth(left)
	if pad < 0 {
		pad = 0
	}
	line := left + strings.Repeat(" ", pad) + qtyField
	if trail := width - displayWidth(line); trail > 0 {
		line += strings.Repeat(" ", trail)
	}
	return line
}

func stationSlipItemLabel(ln jobLine) string {
	code := strings.TrimSpace(ln.ItemCode)
	name := strings.TrimSpace(ln.ItemName)
	if name == "" {
		name = strings.TrimSpace(ln.DisplayName)
	}
	if code != "" && name != "" {
		return code + "-" + name
	}
	if code != "" {
		return code
	}
	if name != "" {
		return name
	}
	return formatItemLabel(ln.ItemIndex, "")
}

func escposPadLine(left, right string, width int) string {
	left = truncateDisplay(left, width-2)
	right = truncateDisplay(right, width-2)
	gap := width - displayWidth(left) - displayWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// writeBody1x1 — Font A normal (receipt fee/footer; station slip footer).
func (w *escposWriter) writeBody1x1() {
	w.size(false, false)
	w.bold(false)
	w.underline(false)
	w.hanRole = hanFontSmall
}

// writeBody1x2 — Font A double height (station slip menu body).
func (w *escposWriter) writeBody1x2() {
	w.size(false, true)
	w.bold(false)
	w.underline(false)
	w.hanRole = hanFontBody
}

// writeBody1x2Bold — receipt column header and amount due (Font A 1×2 bold).
func (w *escposWriter) writeBody1x2Bold() {
	w.size(false, true)
	w.bold(true)
	w.underline(false)
	w.hanRole = hanFontBody
}

// writeMasthead2x2Bold — ticket title and table number emphasis only.
func (w *escposWriter) writeMasthead2x2Bold() {
	w.size(true, true)
	w.bold(true)
	w.hanRole = hanFontLarge
}

func (w *escposWriter) writeHanColumnRow(left, right string, kind hanColumnRowKind) {
	fontPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
	w.content.Write(escposHanColumnRow(left, right, kind, fontPx, bitmapTextStyle{
		Align:     w.alignMode,
		Bold:      w.boldOn,
		Underline: w.underlineOn,
	}))
	w.lastTextBitmap = true
}

func ticketBrandingWord(locale string) string {
	if printLocaleIsZh(locale) {
		return "餐厅"
	}
	return "restaurant"
}

func (w *escposWriter) writeTicketBranding() {
	w.align(0)
	w.writeBody1x1()
	w.text(ticketBrandingWord(w.printLocale))
	w.lf()
}

func (w *escposWriter) writeTicketMasthead(title string) {
	w.writeTicketBranding()
	w.align(1)
	w.writeMasthead2x2Bold()
	w.text(title)
	w.lf()
	w.separator('-')
}

// writeTableContext prints table no. (2×2 bold) and optional meta lines at body size.
func (w *escposWriter) writeTableContext(p jobPayload, lab ticketLabels, tableCentered bool, metaLines ...string) {
	if tableCentered {
		w.align(1)
	} else {
		w.align(0)
	}
	w.writeMasthead2x2Bold()
	if line := p.tableNoLabel(lab); line != "" {
		w.text(line)
		w.lf()
	}
	w.writeBody1x1()
	w.align(0)
	for _, line := range metaLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		w.text(line)
		w.lf()
	}
}

func sortStationMenuLines(lines []jobLine) []jobLine {
	out := append([]jobLine(nil), lines...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.CategoryGroupSort != b.CategoryGroupSort {
			return a.CategoryGroupSort < b.CategoryGroupSort
		}
		return a.ItemIndex < b.ItemIndex
	})
	return out
}

// writeStationSlipHeader — branding, title, table context, Items/Qty column header (1×2).
func (w *escposWriter) writeStationSlipHeader(p jobPayload, lab ticketLabels) {
	w.writeTicketMasthead(lab.guestOrder)
	var meta []string
	if p.GuestCount > 0 {
		meta = append(meta, fmt.Sprintf("%s:%d", lab.guest, p.GuestCount))
	}
	w.writeTableContext(p, lab, false, meta...)
	w.separator('-')
	w.writeBody1x2()
	if stationSlipColumnBlockUsesHanCanvas(p, w) {
		w.writeHanColumnRow(lab.items, lab.qty, hanColHeader)
	} else {
		w.text(stationSlipColumnHeaderLine(lab.items, lab.qty, escposWidth))
		w.lf()
	}
}

func stationSlipNoteMaxWidth(width int) int {
	return stationSlipQtyColStart(width) - stationSlipSideMargin
}

func (w *escposWriter) writeHanStationNote(note, prefix string) {
	fontPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
	style := bitmapTextStyle{
		Align:     w.alignMode,
		Bold:      w.boldOn,
		Underline: true,
	}
	for _, nl := range wrapHanNoteLines(note, prefix, fontPx, false) {
		w.writeBody1x2()
		w.content.Write(escposHanLeftRow(nl.text, nl.x, fontPx, style))
		w.lastTextBitmap = true
	}
}

func (w *escposWriter) writeStationItemNoteLine(note string, width int, p jobPayload) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	prefix := labelsFor(p.Locale).itemNote
	w.writeBody1x2()
	if stationSlipColumnBlockUsesHanCanvas(p, w) {
		w.writeHanStationNote(note, prefix)
		return
	}
	indent := strings.Repeat(" ", stationSlipSideMargin)
	maxW := stationSlipNoteMaxWidth(width)
	w.underline(true)
	for _, line := range wrapDisplay(prefix+note, maxW) {
		w.text(indent + line)
		w.lf()
	}
	w.underline(false)
}

// writeStationMenuLines — guest-order body at Font A 1×2 with column-model layout.
func (w *escposWriter) writeStationMenuLines(p jobPayload, lines []jobLine) {
	showGroup := p.stationSlipShowCategoryGroup()
	w.writeBody1x2()
	lastGroupHeader := ""
	for _, ln := range sortStationMenuLines(lines) {
		groupHeader := strings.TrimSpace(ln.CategoryGroupHeader)
		if showGroup && groupHeader != "" && groupHeader != lastGroupHeader {
			w.align(1)
			w.text(groupHeader) // wrap inside escposBitmapText when Han
			w.lf()
			w.align(0)
			lastGroupHeader = groupHeader
		}

		qty := ln.Qty
		if qty <= 0 {
			qty = 1
		}
		w.writeStationMenuItem(ln, qty, p)
		w.writeStationItemNoteLine(ln.Note, escposWidth, p)
	}
}

func (w *escposWriter) writeStationMenuItem(ln jobLine, qty int, p jobPayload) {
	label := stationSlipItemLabel(ln)
	qtyStr := fmt.Sprintf("%d", qty)

	if stationSlipColumnBlockUsesHanCanvas(p, w) {
		fontPx := hanFontPxForRole(w.hanFontPx, hanFontBody)
		maxPx := hanColumnLabelMaxPx(hanColItem)
		maxCols := stationSlipItemMaxWidth(escposWidth)
		var chunks []string
		if displayWidth(label) <= maxCols {
			chunks = []string{label}
		} else {
			chunks = wrapHanTextByPx(label, maxPx, fontPx, false)
		}
		if len(chunks) == 0 {
			chunks = []string{label}
		}
		w.writeBody1x2()
		w.writeHanColumnRow(chunks[0], qtyStr, hanColItem)
		for _, c := range chunks[1:] {
			w.writeBody1x2()
			w.writeHanColumnRow(c, "", hanColItem)
		}
		return
	}

	maxLeft := stationSlipItemMaxWidth(escposWidth)
	chunks := wrapDisplay(label, maxLeft)
	if len(chunks) == 0 {
		chunks = []string{label}
	}
	w.writeBody1x2()
	w.text(stationSlipItemLine(chunks[0], qtyStr, escposWidth))
	w.lf()
	for _, c := range chunks[1:] {
		w.writeBody1x2()
		var b strings.Builder
		col := 0
		for col < stationSlipSideMargin {
			b.WriteByte(' ')
			col++
		}
		b.WriteString(c)
		w.text(b.String())
		w.lf()
	}
}

func (w *escposWriter) writeStationSlipFooter(p jobPayload, lab ticketLabels) {
	w.separator('-')
	w.writeBody1x1()
	orderAt := strings.TrimSpace(p.OrderTime)
	if orderAt == "" {
		orderAt = nowLocal()
	}
	w.text(fmt.Sprintf("%s:%s", lab.orderTime, orderAt))
	w.lf()
	w.text(fmt.Sprintf("%s:%s", lab.printedBy, ticketBrandingWord(w.printLocale)))
	w.lf()
}

func formatItemLabel(idx int, name string) string {
	name = strings.TrimSpace(name)
	if idx > 0 {
		return fmt.Sprintf("%03d-%s", idx, name)
	}
	return name
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

type receiptLineFields struct {
	label     string
	qtyCol    string
	priceCol  string
	lineTotal float64
	hasPrice  bool
}

func receiptLineFieldsFrom(ln jobLine) receiptLineFields {
	qty := ln.Qty
	if qty <= 0 {
		qty = 1
	}
	label := strings.TrimSpace(ln.DisplayName)
	lineTotal := ln.UnitPrice * float64(qty)
	qtyCol := fmt.Sprintf("%d", qty)
	if shareLabel := strings.TrimSpace(ln.ShareQtyLabel); shareLabel != "" {
		qtyCol = shareLabel
		lineTotal = ln.UnitPrice
	}
	priceCol := ""
	hasPrice := false
	if ln.UnitPrice > 0 {
		hasPrice = true
		priceCol = formatMoney(lineTotal)
	}
	return receiptLineFields{
		label:     label,
		qtyCol:    qtyCol,
		priceCol:  priceCol,
		lineTotal: lineTotal,
		hasPrice:  hasPrice,
	}
}

func (w *escposWriter) writeReceiptMenuLines(lines []jobLine, lab ticketLabels) (sum float64, hasPrice bool) {
	w.writeReceiptMenuBodyLine(lab.items, lab.qty, lab.originalPrice, true)
	for _, ln := range lines {
		fields := receiptLineFieldsFrom(ln)
		if fields.hasPrice {
			hasPrice = true
			sum += fields.lineTotal
		}
		w.writeReceiptMenuBodyLine(fields.label, fields.qtyCol, fields.priceCol, false)
	}
	w.writeBody1x1()
	return sum, hasPrice
}

func nowLocal() string {
	return time.Now().Format("2006-01-02 15:04")
}

func escposFromJob(job printJob) []byte {
	p := parseJobPayload(job)
	lab := labelsFor(p.Locale)

	switch job.Type {
	case "station_ticket":
		return buildStationTicket(p)
	case "order_receipt":
		if p.ConnectionTest {
			return buildConnectionTest(p, lab)
		}
		variant := strings.TrimSpace(p.ReceiptVariant)
		if variant == "" {
			variant = "final"
		}
		withPayment := variant == "final" || variant == "split_payment"
		return buildOrderReceipt(p, printTicketLabels(p.Locale), withPayment, variant)
	case "pre_bill":
		p.ReceiptVariant = "pre_bill"
		return buildOrderReceipt(p, printTicketLabels(p.Locale), false, "pre_bill")
	default:
		return buildStationTicket(p)
	}
}

// buildStationTicket — internal station slip; fixed chrome follows print_locale (zh vs en).
func buildStationTicket(p jobPayload) []byte {
	lab := printTicketLabels(p.Locale)
	w := newEscposForStationTicket(p)
	w.writeStationSlipHeader(p, lab)
	w.writeStationMenuLines(p, p.Lines)
	w.writeStationSlipFooter(p, lab)
	return w.finish(true)
}

// buildOrderReceipt — checkout / pre-bill / split-payment / final (English layout per sample).
func buildOrderReceipt(p jobPayload, lab ticketLabels, withPayment bool, variant string) []byte {
	w := newEscposForReceiptTicket(p)
	isSplit := variant == "split_payment"
	payer := formatSplitPayerForReceipt(p.PayerName)

	w.writeTicketMasthead(receiptHeaderTitle(variant, lab))
	var meta []string
	if isSplit && payer != "" {
		meta = append(meta, fmt.Sprintf("%s:%s", lab.guest, payer))
	} else if p.GuestCount > 0 {
		meta = append(meta, fmt.Sprintf("%s:%d", lab.guest, p.GuestCount))
	}
	w.writeTableContext(p, lab, true, meta...)
	w.separator('-')

	var sum float64
	hasPrice := false
	if len(p.Lines) > 0 {
		sum, hasPrice = w.writeReceiptMenuLines(p.Lines, lab)
		w.separator('-')
	}

	if p.Subtotal > 0 {
		sum = p.Subtotal
		hasPrice = true
	}
	if p.AmountDue > 0 {
		sum = p.AmountDue
		hasPrice = true
	}
	if hasPrice && len(p.Lines) == 0 {
		w.separator('-')
	}
	if hasPrice {
		if !isSplit {
			w.text(lab.feeDetails)
			w.lf()
			w.writeReceiptPadLine(lab.originalTotal, formatMoney(sum))
			w.writeReceiptPadLine(lab.subtotal, formatMoney(sum))
		}
		due := sum
		if p.AmountDue > 0 {
			due = p.AmountDue
		}
		w.writeReceiptAmountDueLine(lab.amountDue + ":" + formatMoney(due))
		if withPayment {
			paid := due
			if p.AmountPaid > 0 {
				paid = p.AmountPaid
			}
			w.rightLine(lab.amountPaid+":"+formatMoney(paid), true)
			method := strings.TrimSpace(p.PaymentMethod)
			if method == "" {
				method = "Cash"
			}
			w.rightLine("-"+method+" Payment:"+formatMoney(paid), false)
		}
		if isSplit {
			w.separator('-')
		}
	}

	w.separator('-')

	orderedByVal := strings.TrimSpace(p.OrderedBy)
	if orderedByVal == "" {
		orderedByVal = lab.printedByVal
	}
	orderAt := strings.TrimSpace(p.OrderTime)
	if orderAt == "" {
		orderAt = nowLocal()
	}
	printAt := strings.TrimSpace(p.PrintTime)
	if printAt == "" {
		printAt = nowLocal()
	}
	w.text(fmt.Sprintf("%s:%s", lab.orderedBy, orderedByVal))
	w.lf()
	w.text(fmt.Sprintf("%s:%s", lab.orderTime, orderAt))
	w.lf()
	w.text(fmt.Sprintf("%s:%s", lab.printedBy, ticketBrandingWord(w.printLocale)))
	w.lf()
	w.text(fmt.Sprintf("%s:%s", lab.printTime, printAt))
	w.lf()

	return w.finish(true)
}

func buildConnectionTest(p jobPayload, lab ticketLabels) []byte {
	w := newEscposForConnectionTest(p)
	w.align(1)
	w.bold(true)
	w.size(true, true)
	w.text(lab.connectionTest)
	w.size(false, false)
	w.bold(false)
	w.lf()
	w.separator('-')
	w.align(0)
	w.text(p.venueName())
	w.lf()
	w.text(nowLocal())
	w.lf()
	return w.finish(true)
}
