package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildStationTicketEnglishLayout(t *testing.T) {
	payload, err := json.Marshal(jobPayload{
		Locale:           "pt",
		RestaurantName:   "川味餐厅",
		TableDisplayName: "A-32",
		GuestCount:       4,
		OrderTime:        "2026-05-14 20:15",
		StationSlipOptions: &stationSlipOptions{
			ShowCategoryGroup: true,
		},
		Lines: []jobLine{
			{
				ItemIndex:           1,
				ItemCode:            "001",
				ItemName:            "Água 500ml",
				DisplayName:         "001-Água 500ml",
				Qty:                 1,
				CategoryGroupSort:   0,
				CategoryGroupHeader: "(Bebidas/ Drinks2)",
			},
			{
				ItemIndex:           7,
				ItemCode:            "007",
				ItemName:            "Coca Cola Zero",
				DisplayName:         "007-Coca Cola Zero",
				Qty:                 1,
				CategoryGroupSort:   0,
				CategoryGroupHeader: "(Bebidas/ Drinks2)",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	s := string(raw)
	for _, bad := range []string{"Mesa", "Pedido", "Estacao", "Estação", "Artigos", "Hora impressao", "Print Time"} {
		if strings.Contains(s, bad) {
			t.Fatalf("station ticket must not contain %q", bad)
		}
	}
	for _, want := range []string{
		"restaurant",
		"Guest Order",
		"Table No.:A-32",
		"Guest:4",
		"Items",
		"Qty",
		"(Bebidas/ Drinks2)",
		"001-",
		"500ml",
		"007-Coca Cola Zero",
		"Order Time:",
		"Printed By:restaurant",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in ticket output", want)
		}
	}

	if bytes.Contains(raw, []byte{0xC1}) || bytes.Contains(raw, []byte{0xE1}) {
		// Windows-1252 Á/á when present in payload.
	} else {
		t.Fatalf("expected Windows-1252 accented bytes in ticket output")
	}

	idx := bytes.Index(raw, []byte("Items"))
	if idx < 0 {
		t.Fatal(`missing "Items" column header`)
	}
	if !bytes.Contains(raw[idx:], []byte{0x1D, 0x21, 0x01}) {
		t.Fatal("menu body must use GS ! 1×2 after column headers")
	}
	if bytes.Contains(raw[idx:], []byte{0x1D, 0x21, 0x11}) {
		t.Fatal("menu body must not use GS ! 2×2 after column headers")
	}
}

func TestStationSlipSkipsCategoryHeaderWhenDisabled(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		TableDisplayName: "1",
		StationSlipOptions: &stationSlipOptions{
			ShowCategoryGroup: false,
		},
		Lines: []jobLine{{
			ItemCode:            "001",
			ItemName:            "Soup",
			DisplayName:         "001-Soup",
			Qty:                 1,
			CategoryGroupHeader: "(Bebidas/ Drinks2)",
		}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	if strings.Contains(string(raw), "(Bebidas/ Drinks2)") {
		t.Fatal("category header must not print when show_category_group is false")
	}
}

func TestStationSlipItemLineLayout(t *testing.T) {
	line := stationSlipItemLine("001-Agua 500ml", "3", escposWidth)
	if displayWidth(line) != escposWidth {
		t.Fatalf("line display width %d want %d", displayWidth(line), escposWidth)
	}
	runes := []rune(line)
	if runes[stationSlipSideMargin] != '0' {
		t.Fatalf("expected item label at col %d, got %q", stationSlipSideMargin, runes[stationSlipSideMargin])
	}
	qtyCol := []rune(padFieldRight("3", stationSlipQtyColWidth))
	qtyStart := stationSlipQtyColStart(escposWidth)
	for i, c := range qtyCol {
		if runes[qtyStart+i] != c {
			t.Fatalf("expected centered qty at col %d: got %q want %q", qtyStart, string(runes[qtyStart:qtyStart+len(qtyCol)]), string(qtyCol))
		}
	}
	for i := len(runes) - stationSlipSideMargin; i < len(runes); i++ {
		if runes[i] != ' ' {
			t.Fatalf("expected right margin %d cols", stationSlipSideMargin)
		}
	}
}

// Han column row: Qty ink right-aligned in the Qty field on a full 576px canvas.
func TestHanColumnRowQtyInkInBand(t *testing.T) {
	fontPx := bitmapTextDefaultFontPx
	labels := []string{"001-中水", "002-冰水 500毫升", "003-冰Vitalis 750ml"}
	fieldEnd := hanQtyFieldEndPx()
	tol := fontPx

	for _, label := range labels {
		maxPx := hanColumnLabelMaxPx(hanColItem)
		chunks := wrapHanTextByPx(label, maxPx, fontPx, false)
		if len(chunks) == 0 {
			t.Fatalf("empty wrap for %q", label)
		}
		img := renderBitmapColumnRow(chunks[0], "1", hanColItem, fontPx, bitmapTextStyle{})
		if img.Width != bitmapTextMaxWidthPx {
			t.Fatalf("%q canvas width %d want %d", label, img.Width, bitmapTextMaxWidthPx)
		}
		maxX := bitmapInkMaxX(img)
		if maxX < fieldEnd-tol {
			t.Fatalf("%q qty right edge %d want near field end %d", label, maxX, fieldEnd)
		}
		header := renderBitmapColumnRow("Items", "Qty", hanColHeader, fontPx, bitmapTextStyle{})
		headerMax := bitmapInkMaxX(header)
		if absInt(headerMax-maxX) > tol {
			t.Fatalf("%q qty x mismatch header=%d item=%d", label, headerMax, maxX)
		}
	}
}

func TestHanNoteWrapUsesPixelWidth(t *testing.T) {
	fontPx := bitmapTextDefaultFontPx
	prefix := labelsFor("zh").itemNote
	note := strings.Repeat("测", 40)
	lines := wrapHanNoteLines(note, prefix, fontPx, false)
	if len(lines) < 2 {
		t.Fatal("expected multi-line note wrap")
	}
	w0 := measureHanTextWidth(lines[0].text, fontPx, false)
	lineEnd := hanNoteLeftPx() + w0
	if lineEnd > hanQtyColStartPx()-hanColumnGapPx() {
		t.Fatalf("first line extends past qty band: end %d max %d", lineEnd, hanQtyColStartPx()-hanColumnGapPx())
	}
	prefixW := measureHanTextWidth(prefix, fontPx, false)
	firstBody := strings.TrimPrefix(lines[0].text, prefix)
	if len([]rune(firstBody)) < 4 {
		t.Fatalf("first line body too short (%d Han runes), regresses early wrap", len([]rune(firstBody)))
	}
	if lines[1].x <= lines[0].x {
		t.Fatalf("continuation must hang indent past prefix: x0=%d x1=%d", lines[0].x, lines[1].x)
	}
	if lines[1].x != hanNoteLeftPx()+prefixW {
		t.Fatalf("hang x got %d want %d", lines[1].x, hanNoteLeftPx()+prefixW)
	}
}

func TestHanNoteEscposSingleWrapPath(t *testing.T) {
	longNote := strings.Repeat("德萨发生发顺丰", 8)
	prefix := labelsFor("zh").itemNote
	payload, _ := json.Marshal(jobPayload{
		Locale:           "zh",
		TableDisplayName: "A-01",
		Lines: []jobLine{{
			ItemCode:    "001",
			ItemName:    "中水",
			DisplayName: "001-中水",
			Qty:         1,
			Note:        longNote,
		}},
	})
	fontPx := bitmapTextDefaultFontPx
	wantLines := len(wrapHanNoteLines(longNote, prefix, fontPx, false))
	itemLines := 1
	wantRaster := wantLines + itemLines + 1 // header row
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	gotRaster := bytes.Count(raw, []byte{0x1D, 0x76, 0x30, 0x00})
	if gotRaster < wantRaster {
		t.Fatalf("GS v 0 count %d want at least %d (note must not double-wrap via escposBitmapText)", gotRaster, wantRaster)
	}
}

func TestHanColumnLeftMatchesItemsMargin(t *testing.T) {
	want := escposDisplayColToPx(stationSlipSideMargin)
	if got := hanColumnLeftPx(hanColHeader); got != want {
		t.Fatalf("header left px %d want %d", got, want)
	}
	if got := hanColumnLeftPx(hanColItem); got != want {
		t.Fatalf("item left px %d want %d", got, want)
	}
	if got := hanNoteLeftPx(); got != want {
		t.Fatalf("note left px %d want %d", got, want)
	}
}

func TestHanItemLabelFont34SingleLine(t *testing.T) {
	fontPx := 34
	label := "002-冰水 500毫升"
	maxPx := hanColumnLabelMaxPx(hanColItem)
	maxCols := stationSlipItemMaxWidth(escposWidth)
	var chunks []string
	if displayWidth(label) <= maxCols {
		chunks = []string{label}
	} else {
		chunks = wrapHanTextByPx(label, maxPx, fontPx, false)
	}
	if len(chunks) != 1 {
		t.Fatalf("want single line for %q at font %d, got %d chunks %v", label, fontPx, len(chunks), chunks)
	}
	if chunks[0] != label {
		t.Fatalf("chunk %q want %q", chunks[0], label)
	}
}

func TestHanNoteFont34FirstLineFitsBody(t *testing.T) {
	fontPx := 34
	prefix := labelsFor("zh").itemNote
	note := strings.Repeat("不要香菜", 6)
	lines := wrapHanNoteLines(note, prefix, fontPx, false)
	if len(lines) < 2 {
		t.Fatal("expected multi-line note")
	}
	if !strings.HasPrefix(lines[0].text, prefix) {
		t.Fatalf("first line %q missing prefix %q", lines[0].text, prefix)
	}
	firstBody := strings.TrimPrefix(lines[0].text, prefix)
	if len([]rune(firstBody)) < 4 {
		t.Fatalf("first line body too short (%q), prefix must not eat entire line width", firstBody)
	}
	if lines[0].x != hanNoteLeftPx() {
		t.Fatalf("first line x got %d want %d", lines[0].x, hanNoteLeftPx())
	}
}

func TestItemNotePrefixFollowsLocale(t *testing.T) {
	if got := labelsFor("zh").itemNote; got != "备注: " {
		t.Fatalf("zh itemNote %q", got)
	}
	if got := labelsFor("en").itemNote; got != "Note: " {
		t.Fatalf("en itemNote %q", got)
	}
	if got := labelsFor("pt").itemNote; got != "Observação: " {
		t.Fatalf("pt itemNote %q", got)
	}
}

func TestHanCanvasWidthIsPOS80(t *testing.T) {
	if bitmapTextMaxWidthPx != 576 {
		t.Fatalf("canvas width %d want 576 (POS-80)", bitmapTextMaxWidthPx)
	}
	if escposDisplayColToPx(1) != 12 {
		t.Fatalf("col pitch %d want 12", escposDisplayColToPx(1))
	}
	if stationSlipQtyColWidth != 4 {
		t.Fatalf("qty col width %d want 4", stationSlipQtyColWidth)
	}
	if stationSlipQtyColStart(escposWidth) != 40 {
		t.Fatalf("qty start col %d want 40", stationSlipQtyColStart(escposWidth))
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestHanFontPxForRole(t *testing.T) {
	base := 24
	if got := hanFontPxForRole(base, hanFontSmall); got != 18 {
		t.Fatalf("small want 18 got %d", got)
	}
	if got := hanFontPxForRole(base, hanFontBody); got != 24 {
		t.Fatalf("body want 24 got %d", got)
	}
	if got := hanFontPxForRole(base, hanFontLarge); got != 36 {
		t.Fatalf("large want 36 got %d", got)
	}
}

func TestStationSlipColumnHeaderLayout(t *testing.T) {
	line := stationSlipColumnHeaderLine("Items", "Qty", escposWidth)
	runes := []rune(line)
	if runes[stationSlipSideMargin] != 'I' {
		t.Fatalf("expected Items at col %d (Guest 't'), got %q", stationSlipSideMargin, runes[stationSlipSideMargin])
	}
	qtyCol := []rune(padFieldRight("Qty", stationSlipQtyColWidth))
	qtyStart := stationSlipQtyColStart(escposWidth)
	for i, c := range qtyCol {
		if runes[qtyStart+i] != c {
			t.Fatalf("expected centered Qty header at col %d: got %q want %q", qtyStart, string(runes[qtyStart:qtyStart+len(qtyCol)]), string(qtyCol))
		}
	}
}

func TestStationTicketItemNoteUsesUnderline(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Locale:           "pt",
		TableDisplayName: "A-1",
		Lines: []jobLine{{
			ItemCode:    "001",
			ItemName:    "Soup",
			DisplayName: "001-Soup",
			Qty:         1,
			Note:        "no onion",
		}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	labelIdx := bytes.Index(raw, []byte("Observ"))
	if labelIdx < 0 {
		t.Fatal("missing Observação: prefix")
	}
	prefix := raw[max(0, labelIdx-16):labelIdx]
	if !bytes.Contains(prefix, []byte{0x1B, 0x2D, 0x01}) {
		t.Fatal("expected ESC - 1 underline before item note")
	}
	if !bytes.Contains(raw, []byte("Observ")) || !bytes.Contains(raw, []byte(": no onion")) {
		t.Fatal("expected Observação: prefix before item note")
	}
}

func TestWrapDisplay(t *testing.T) {
	if got := wrapDisplay("", 10); got != nil {
		t.Fatalf("empty want nil, got %v", got)
	}
	if got := wrapDisplay("ab", 0); got != nil {
		t.Fatalf("max<=0 want nil, got %v", got)
	}
	got := wrapDisplay("abcdefghij", 4)
	want := []string{"abcd", "efgh", "ij"}
	if len(got) != len(want) {
		t.Fatalf("len got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// Han = 2 cols
	han := wrapDisplay("测试测试测试", 4)
	if len(han) < 2 {
		t.Fatalf("Han wrap want >=2 chunks, got %v", han)
	}
	for _, c := range han {
		if displayWidth(c) > 4 {
			t.Fatalf("chunk %q wider than 4 cols", c)
		}
	}
}

func TestStationSlipNoteMaxWidthLeftOfQty(t *testing.T) {
	maxW := stationSlipNoteMaxWidth(escposWidth)
	if stationSlipSideMargin+maxW > stationSlipQtyColStart(escposWidth) {
		t.Fatalf("note text band overlaps qty column: indent=%d max=%d qtyStart=%d",
			stationSlipSideMargin, maxW, stationSlipQtyColStart(escposWidth))
	}
}

func TestStationTicketItemNoteWrapsFullText(t *testing.T) {
	note := "2 pacotes de acucar 1 pacote de leite em po sem canela e bem quente por favor"
	full := labelsFor("pt").itemNote + note
	maxW := stationSlipNoteMaxWidth(escposWidth)
	chunks := wrapDisplay(full, maxW)
	if len(chunks) < 2 {
		t.Fatal("fixture note too short to exercise wrap")
	}
	payload, _ := json.Marshal(jobPayload{
		TableDisplayName: "068",
		Lines: []jobLine{{
			ItemCode:    "903",
			ItemName:    "Cafe",
			DisplayName: "903-Cafe",
			Qty:         2,
			Note:        note,
		}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	for _, chunk := range chunks {
		enc := encodeWindows1252(chunk)
		if !bytes.Contains(raw, enc) {
			t.Fatalf("missing wrapped note chunk %q", chunk)
		}
	}
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("station note must not use ellipsis truncation")
	}
	itemLine := stationSlipItemLine("903-Cafe", "2", escposWidth)
	qtyStart := stationSlipQtyColStart(escposWidth)
	qtyCol := []rune(padFieldRight("2", stationSlipQtyColWidth))
	runes := []rune(itemLine)
	for i, c := range qtyCol {
		if runes[qtyStart+i] != c {
			t.Fatalf("qty column disturbed: got %q", itemLine)
		}
	}
}

func TestStationTicketLongZhNameWrapsNoEllipsis(t *testing.T) {
	longName := "超长中文菜名测试一二三四五六七八九十甲乙丙丁戊己庚辛"
	payload, _ := json.Marshal(jobPayload{
		Locale:           "zh",
		TableDisplayName: "A-01",
		Lines: []jobLine{{
			ItemCode:    "001",
			ItemName:    longName,
			DisplayName: "001-" + longName,
			Qty:         1,
			Note:        "我要冰的备注要打全",
		}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("zh station slip must not ellipsis-truncate menu or note")
	}
	if !bytes.Contains(raw, []byte{0x1D, 0x76, 0x30, 0x00}) {
		t.Fatal("expected GS v 0 for zh menu")
	}
	if bytes.Count(raw, []byte{0x1D, 0x76, 0x30, 0x00}) < 2 {
		t.Fatal("long zh name+note should emit multiple bitmaps")
	}
}

func TestStationTicketUsesLatinEncodingOnly(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Lines: []jobLine{{
			ItemCode:    "001",
			ItemName:    "宫保鸡丁",
			DisplayName: "001-宫保鸡丁",
			Qty:         1,
		}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	if bytes.Contains(raw, []byte{0x1C, 0x26}) {
		t.Fatal("station slip must not enter GBK mode")
	}
}
