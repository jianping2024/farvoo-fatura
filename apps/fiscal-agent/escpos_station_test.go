package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/escposenc"
)

var (
	gsSize1x1 = []byte{0x1D, 0x21, 0x00}
	gsSize1x2 = []byte{0x1D, 0x21, 0x01}
	gsSize2x2 = []byte{0x1D, 0x21, 0x11}
	gsReverse = []byte{0x1D, 0x42}
	gsV0      = []byte{0x1D, 0x76, 0x30, 0x00}
)

func stationTicketRaw(t *testing.T, p jobPayload) []byte {
	t.Helper()
	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return escposFromJob(printJob{Type: "station_ticket", Payload: payload})
}

// leadRow is the exact Font A bytes of one dish lead: GS ! 2×2 + "1× 001" + GS ! 1×2.
func leadRow(qty int, code string) []byte {
	b := append([]byte{}, gsSize2x2...)
	b = append(b, escposenc.Windows1252(stationSlipQtyCodeLead(qty, code))...)
	return append(b, gsSize1x2...)
}

func TestBuildStationTicketPortuguesePrintLocale(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		RestaurantName:   "川味餐厅",
		TableDisplayName: "A-32",
		GuestCount:       4,
		OrderTime:        "2026-05-14 20:15",
		Lines: []jobLine{
			{
				ItemIndex:           1,
				ItemCode:            "001",
				ItemName:            "Água 500ml",
				DisplayName:         "001-Água 500ml",
				Qty:                 1,
			},
			{
				ItemIndex:           7,
				ItemCode:            "007",
				ItemName:            "Coca Cola Zero",
				DisplayName:         "007-Coca Cola Zero",
				Qty:                 2,
			},
		},
	})
	s := string(raw)
	for _, bad := range []string{"Guest Order", "Table No.", "Items", "Order Time:", "Printed By:", "Artigos", "Qtd"} {
		if strings.Contains(s, bad) {
			t.Fatalf("pt station ticket must not contain %q", bad)
		}
	}
	for _, want := range []string{
		"restaurant",
		"Pedido",
		"Conv.:4",
		"3 artigos",
		"Coca Cola Zero",
		"2026-05-14 20:15",
		"Impresso por:restaurant",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in ticket output", want)
		}
	}
	// table no. and order clock share one 2×2 row
	if !bytes.Contains(raw, escposenc.Windows1252(escposPadLine("Mesa n.º:A-32", "20:15", escposWidth/2))) {
		t.Fatal("table no. and HH:MM must share the 2×2 table row")
	}
	assertAccentsInFirmwareFont(t, raw, "Água", "n.º")

	if !bytes.Contains(raw, append(leadRow(1, "001"), escposenc.Windows1252(" Água 500ml")...)) {
		t.Fatal("dish row must be 2×2 qty+code lead followed by the 1×2 name")
	}
	if !bytes.Contains(raw, leadRow(2, "007")) {
		t.Fatal("second dish must carry its own qty+code lead")
	}
	if bytes.Contains(raw, gsReverse) {
		t.Fatal("dish rows must not use GS B reverse")
	}
}

func TestBuildStationTicketEnglishPrintLocale(t *testing.T) {
	s := string(stationTicketRaw(t, jobPayload{
		Locale:           "en",
		TableDisplayName: "A-32",
		GuestCount:       4,
		OrderTime:        "2026-05-14 20:15",
		Lines: []jobLine{{
			ItemCode: "001", ItemName: "Água 500ml", Qty: 1,
		}},
	}))
	for _, want := range []string{"Guest Order", "Table No.:A-32", "Guest:4", "1 item", "Printed By:restaurant"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in en station ticket", want)
		}
	}
}

func TestStationSlipHeaderShowsStationName(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		Locale:               "pt",
		TableDisplayName:     "1",
		StationDisplayNamePt: "BAR",
		Lines:                []jobLine{{ItemCode: "001", ItemName: "Cola", Qty: 1}},
	})
	if !bytes.Contains(raw, escposenc.Windows1252("restaurant · BAR")) {
		t.Fatal("branding line must carry the station name")
	}
}

func TestStationSlipPrintsCategoryHeaderWhenEnabled(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "1",
		StationSlipOptions: &stationSlipOptions{
			ShowCategoryGroup: true,
		},
		Lines: []jobLine{
			{
				ItemIndex: 1, ItemCode: "001", ItemName: "Água 500ml", Qty: 1,
				CategoryGroupSort: 0, CategoryGroupHeader: "(BEBIDAS)",
			},
			{
				ItemIndex: 2, ItemCode: "007", ItemName: "Cola", Qty: 2,
				CategoryGroupSort: 0, CategoryGroupHeader: "(BEBIDAS)",
			},
			{
				ItemIndex: 3, ItemCode: "020", ItemName: "Frango", Qty: 1,
				CategoryGroupSort: 1, CategoryGroupHeader: "(PRATOS)",
			},
		},
	})
	if !bytes.Contains(raw, []byte("(BEBIDAS)")) || !bytes.Contains(raw, []byte("(PRATOS)")) {
		t.Fatal("category headers must print as-is when show_category_group is true")
	}
	if bytes.Count(raw, []byte("(BEBIDAS)")) != 1 {
		t.Fatalf("same category header must print once, got %d", bytes.Count(raw, []byte("(BEBIDAS)")))
	}
	bebidas := bytes.Index(raw, []byte("(BEBIDAS)"))
	agua := bytes.Index(raw, escposenc.Windows1252("Água 500ml"))
	cola := bytes.Index(raw, []byte("Cola"))
	pratos := bytes.Index(raw, []byte("(PRATOS)"))
	frango := bytes.Index(raw, []byte("Frango"))
	if !(bebidas < agua && agua < cola && cola < pratos && pratos < frango) {
		t.Fatalf("want (BEBIDAS) < dishes < (PRATOS) < Frango; idx %d %d %d %d %d",
			bebidas, agua, cola, pratos, frango)
	}
	// No solid separators around category headers (dish-to-dish dashed divider still OK).
	solid := []byte(strings.Repeat("-", 8))
	before := raw[max(0, bebidas-40):bebidas]
	after := raw[bebidas+len("(BEBIDAS)") : min(len(raw), bebidas+len("(BEBIDAS)")+40)]
	if bytes.Contains(before, solid) || bytes.Contains(after, solid) {
		t.Fatal("category header must not be wrapped in solid separators")
	}
}

func TestStationSlipSkipsCategoryHeaderWhenDisabled(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		TableDisplayName: "1",
		StationSlipOptions: &stationSlipOptions{
			ShowCategoryGroup: false,
		},
		Lines: []jobLine{{
			ItemCode: "001", ItemName: "Soup", Qty: 1,
			CategoryGroupHeader: "(Bebidas/ Drinks2)",
		}},
	})
	if strings.Contains(string(raw), "Bebidas") {
		t.Fatal("category header must not print when show_category_group is false")
	}
}

func TestStationSlipSkipsCategoryHeaderWhenOptionsNil(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		TableDisplayName: "1",
		Lines: []jobLine{{
			ItemCode: "001", ItemName: "Soup", Qty: 1,
			CategoryGroupHeader: "(BEBIDAS)",
		}},
	})
	if strings.Contains(string(raw), "BEBIDAS") {
		t.Fatal("category header must not print when station_slip_options is absent")
	}
}

func TestStationSlipNames(t *testing.T) {
	code, names := stationSlipNames(jobLine{ItemCode: "001", ItemName: "Água 500ml"})
	if code != "001" || len(names) != 1 || names[0] != "Água 500ml" {
		t.Fatalf("legacy = %q %q", code, names)
	}
	code, names = stationSlipNames(jobLine{ItemCode: "001", ItemName: "Água 500ml", PrintNames: []string{" Água 500ml ", "", "矿泉水"}})
	if code != "001" || len(names) != 2 || names[0] != "Água 500ml" || names[1] != "矿泉水" {
		t.Fatalf("print_names = %q %q", code, names)
	}
	// display_name fallback must not repeat the code
	if _, names = stationSlipNames(jobLine{ItemCode: "003", DisplayName: "003-Cola"}); names[0] != "Cola" {
		t.Fatalf("display_name fallback = %q", names)
	}
	if code, names = stationSlipNames(jobLine{PrintNames: []string{"Cola", "Coke"}}); code != "" || names[1] != "Coke" {
		t.Fatalf("no-code = %q %q", code, names)
	}
}

func TestStationSlipQtyCodeLead(t *testing.T) {
	if got := stationSlipQtyCodeLead(1, "001"); got != "1× 001" {
		t.Fatalf("lead %q", got)
	}
	if got := stationSlipQtyCodeLead(12, ""); got != "12×" {
		t.Fatalf("no-code lead %q", got)
	}
	// "1× 001" at 2×2 = 12 cols + one 1×2 space
	if got := stationSlipNameIndent("1× 001"); got != 13 {
		t.Fatalf("indent %d want 13", got)
	}
}

func TestStationSlipClock(t *testing.T) {
	for in, want := range map[string]string{
		"2026-10-18 18:01":          "18:01",
		"2026-10-18T09:05:00+01:00": "09:05",
		"":                          "",
		"garbage":                   "",
	} {
		if got := stationSlipClock(in); got != want {
			t.Fatalf("clock(%q)=%q want %q", in, got, want)
		}
	}
}

func TestStationItemExtraNamesAndNoteHangUnderName(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines: []jobLine{{
			ItemCode:   "020",
			Qty:        7,
			Note:       "sem sal",
			PrintNames: []string{"Frango Piri-Piri", "Grilled chicken"},
		}},
	})
	indent := strings.Repeat(" ", stationSlipNameIndent(stationSlipQtyCodeLead(7, "020")))
	first := bytes.Index(raw, append(leadRow(7, "020"), []byte(" Frango Piri-Piri")...))
	second := bytes.Index(raw, []byte(indent+"Grilled chicken"))
	note := bytes.Index(raw, escposenc.Windows1252(indent+"» sem sal"))
	if first < 0 || second < 0 || note < 0 || !(first < second && second < note) {
		t.Fatalf("want lead+name < extra name < note, all hung under name; got %d %d %d", first, second, note)
	}
	// other-language names 1×1, note back to 1×2
	if !bytes.Contains(raw[first:second], gsSize1x1) {
		t.Fatal("extra print_names must be 1×1")
	}
	if !bytes.Contains(raw[second:note], gsSize1x2) {
		t.Fatal("note must be 1×2")
	}
	if bytes.Count(raw, gsSize2x2) != 3 {
		t.Fatal("2×2 only for title, table row and the one dish lead")
	}
	if bytes.Contains(raw, []byte{0x1B, 0x2D, 0x01}) {
		t.Fatal("notes must not be underlined")
	}
	// ESC E smears the tall 1×2 Font A on store printers: nothing after the header is bold.
	if bytes.Contains(raw[first:], []byte{0x1B, 0x45, 0x01}) {
		t.Fatal("dish rows and notes must not be bold")
	}
}

func TestStationTicketLongNameWrapsUnderName(t *testing.T) {
	long := "Bacalhau à Brás com Batata Palha e Azeitonas Pretas e Ovo Escalfado"
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines:            []jobLine{{ItemCode: "012", Qty: 2, PrintNames: []string{long}}},
	})
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("dish names must wrap, never ellipsis")
	}
	indent := stationSlipNameIndent(stationSlipQtyCodeLead(2, "012"))
	chunks := stationSlipWrap(long, escposWidth-indent)
	if len(chunks) < 2 {
		t.Fatal("fixture too short to wrap")
	}
	if !bytes.Contains(raw, escposenc.Windows1252(strings.Repeat(" ", indent)+chunks[1])) {
		t.Fatal("wrapped tail must hang under the name column")
	}
}

func TestStationTicketItemNoteWrapsFullText(t *testing.T) {
	note := "2 pacotes de acucar 1 pacote de leite em po sem canela e bem quente por favor"
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "068",
		Lines:            []jobLine{{ItemCode: "903", ItemName: "Cafe", Qty: 2, Note: note}},
	})
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("station note must not ellipsis-truncate")
	}
	if !bytes.Contains(raw, []byte("por favor")) {
		t.Fatal("note tail missing")
	}
}

func TestStationTicketHanMenuUsesFirmware1x2WhenOptedIn(t *testing.T) {
	withThermalConfig(t, "han_firmware")
	raw := stationTicketRaw(t, jobPayload{
		Lines: []jobLine{{
			ItemCode:   "001",
			ItemName:   "Água 500ml",
			Qty:        1,
			PrintNames: []string{"Água 500ml", "宫保鸡丁"},
		}},
	})
	gbk := []byte{0xB9, 0xAC, 0xB1, 0xA3} // 宫保

	if bytes.Contains(raw, gsV0) {
		t.Fatal("han_firmware must not raster Han dish names")
	}
	if !bytes.Contains(raw, append(leadRow(1, "001"), escposenc.Windows1252(" Água 500ml")...)) {
		t.Fatal("pt name must stay Font A WPC1252 after the 2×2 lead")
	}
	// second name 宫保鸡丁 is 1×1: GS ! 0 then FS & + GBK (HanFirmwareLine sends no GS ! at 1×1)
	at := bytes.Index(raw, append([]byte{0x1C, 0x26}, gbk...))
	if at < 0 || !bytes.Contains(raw[:at], gsSize1x1) || bytes.LastIndex(raw[:at], gsSize1x1) < bytes.LastIndex(raw[:at], gsSize1x2) {
		t.Fatal("other-language Han name must print at 1×1 (GS ! 0x00 + FS & + GBK)")
	}
}

func TestStationTicketHanNameOnBlockRowStaysFirmware(t *testing.T) {
	withThermalConfig(t, "han_firmware")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines:            []jobLine{{ItemCode: "001", ItemName: "中水", Qty: 1, Note: "正常冰"}},
	})
	// reverse block (Font A) then firmware Han name on the same row
	want := append(leadRow(1, "001"), ' ', 0x1C, 0x26)
	if !bytes.Contains(raw, want) {
		t.Fatal("Han name must follow the 2×2 lead on the same row via FS &")
	}
	if bytes.Contains(raw, gsV0) {
		t.Fatal("auto must not raster Han name or note")
	}
}

func TestStationTicketPtNoteWithHanStaysFirmware(t *testing.T) {
	withThermalConfig(t, "han_firmware")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines: []jobLine{{
			ItemCode:   "012",
			Qty:        2,
			Note:       "sem cebola, 少放盐",
			PrintNames: []string{"Bacalhau à Brás", "葡式炒鳕鱼"},
		}},
	})
	if bytes.Contains(raw, gsV0) {
		t.Fatal("Han note must not raster")
	}
	if !bytes.Contains(raw, escposenc.Windows1252("» sem cebola, ")) {
		t.Fatal("note prefix must stay Font A WPC1252")
	}
	if !bytes.Contains(raw, []byte{0x1C, 0x26, 0x1D, 0x21, 0x01}) {
		t.Fatal("Han note run must use GS ! 0x01 like dish names")
	}
	if bytes.Contains(raw, []byte{0x1C, 0x21}) {
		t.Fatal("Han lines must not emit FS !")
	}
}

func TestStationItemUsesHanCanvasOnlyForRasterFirstName(t *testing.T) {
	withThermalConfig(t, "bitmap") // Han canvas exists only on the bitmap escape path
	w := newEscposForStationTicket(jobPayload{Locale: "zh", Lines: []jobLine{{ItemCode: "031", ItemName: "可乐", Qty: 1}}})
	if !stationItemUsesHanCanvas(w, []string{"可乐"}) {
		t.Fatal("Han first name must use the canvas row")
	}
	if stationItemUsesHanCanvas(w, []string{"Café Cheio", "咖啡"}) {
		t.Fatal("Latin first name stays Font A even with a Han sub-name")
	}
}

// Store 0.5.47: only Han rows raster — a Latin dish with a Han note keeps the Font A lead/name
// bytes (same as a pt-only slip) and the note is one GS v 0 row.
func TestStationSlipLatinDishHanNoteOnlyNoteRasters(t *testing.T) {
	withThermalConfig(t, "bitmap")
	for _, loc := range []string{"pt", "zh"} {
		raw := stationTicketRaw(t, jobPayload{
			Locale:           loc,
			TableDisplayName: "A-01",
			Lines: []jobLine{
				{ItemCode: "902", ItemName: "Café Cheio", Qty: 3, Note: "三分糖"},
				{ItemCode: "901", ItemName: "Café", Qty: 1},
			},
		})
		a := bytes.Index(raw, append(leadRow(3, "902"), escposenc.Windows1252(" Café Cheio")...))
		b := bytes.Index(raw, append(leadRow(1, "901"), escposenc.Windows1252(" Café")...))
		if a < 0 || b < 0 {
			t.Fatalf("%s: Latin dish rows must stay Font A (a=%d b=%d)", loc, a, b)
		}
		lo, hi := min(a, b), max(a, b)
		// zh chrome (title / footer labels) rasters on its own; the menu block holds only the note.
		if n := bytes.Count(raw[lo:hi], gsV0); n != 1 {
			t.Fatalf("%s: menu GS v 0 count %d want 1 (note only)", loc, n)
		}
	}
}

func TestZhStationSlipBitmapItemRowsAreRasters(t *testing.T) {
	withThermalConfig(t, "bitmap")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "zh",
		TableDisplayName: "A-03",
		GuestCount:       2,
		Lines: []jobLine{
			{ItemCode: "031", ItemName: "Guarana", Qty: 1},
			{ItemCode: "007", ItemName: "可乐", Qty: 2, Note: "少冰"},
		},
	})
	// 可乐 dish row + 少冰 note on the canvas; Guarana stays Font A
	if n := bytes.Count(raw, gsV0); n < 2 {
		t.Fatalf("GS v 0 count %d want >= 2", n)
	}
	if !bytes.Contains(raw, append(leadRow(1, "031"), escposenc.Windows1252(" Guarana")...)) {
		t.Fatal("Latin dish on a zh slip must stay Font A")
	}
	if bytes.Contains(raw, gsReverse) {
		t.Fatal("GS B must not be used")
	}
}

// Chinese title / table row stay regular (bold YaHei clashed with the dish rows); Latin keeps bold.
func TestStationSlipHeaderBoldOnlyForLatin(t *testing.T) {
	withThermalConfig(t, "bitmap")
	escBoldOn := []byte{0x1B, 0x45, 0x01}
	lines := []jobLine{{ItemCode: "001", ItemName: "Água 500ml", Qty: 1}}
	zh := stationTicketRaw(t, jobPayload{Locale: "zh", TableDisplayName: "001", GuestCount: 2, Lines: lines})
	if bytes.Contains(zh, escBoldOn) {
		t.Fatal("zh station slip header must not be bold")
	}
	pt := stationTicketRaw(t, jobPayload{Locale: "pt", TableDisplayName: "001", GuestCount: 2, Lines: lines})
	if !bytes.Contains(pt, escBoldOn) {
		t.Fatal("pt station slip header keeps bold")
	}
}

func TestMergeBitmapRowsBottomAligns(t *testing.T) {
	large := hanFontPxForRole(bitmapTextDefaultFontPx, hanFontLarge)
	body := hanFontPxForRole(bitmapTextDefaultFontPx, hanFontBody)
	nameX := escposDisplayColToPx(stationSlipNameIndent("1× 001"))
	lead := renderBitmapLeftRow("1× 001", 0, large, bitmapTextStyle{})
	name := renderBitmapLeftRow("中水", nameX, body, bitmapTextStyle{})
	row := mergeBitmapRows(lead, name)
	if row.Width != bitmapTextMaxWidthPx || row.Height != max(lead.Height, name.Height) {
		t.Fatalf("merged %dx%d", row.Width, row.Height)
	}
	if !bitmapInkInXBand(row, nameX, bitmapTextMaxWidthPx) {
		t.Fatal("name ink missing after the lead")
	}
	// name sits on the bottom: its top rows (above the shorter glyph) stay blank
	top := bitmapTextImage{Width: row.Width, Height: row.Height - name.Height, Pixels: row.Pixels[:row.Width*(row.Height-name.Height)]}
	if top.Height > 0 && bitmapInkInXBand(top, nameX, bitmapTextMaxWidthPx) {
		t.Fatal("shorter name must be bottom-aligned with the 2×2 lead")
	}
}

func TestStationTicketLongZhNameWrapsNoEllipsis(t *testing.T) {
	withThermalConfig(t, "bitmap")
	longName := "超长中文菜名测试一二三四五六七八九十甲乙丙丁戊己庚辛"
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "zh",
		TableDisplayName: "A-01",
		Lines:            []jobLine{{ItemCode: "001", ItemName: longName, Qty: 1, Note: "我要冰的备注要打全"}},
	})
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("zh station slip must not ellipsis-truncate menu or note")
	}
	if bytes.Count(raw, gsV0) < 2 {
		t.Fatal("long zh name+note should emit multiple bitmaps")
	}
}

func TestHanCanvasWidthIsPOS80(t *testing.T) {
	if bitmapTextMaxWidthPx != 576 {
		t.Fatalf("canvas width %d want 576 (POS-80)", bitmapTextMaxWidthPx)
	}
	if escposDisplayColToPx(1) != 12 {
		t.Fatalf("col pitch %d want 12", escposDisplayColToPx(1))
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
	if got := hanFontPxForRole(base, hanFontSmall); got != 24 {
		t.Fatalf("small (1×1) want 24 got %d", got)
	}
	if got := hanFontPxForRole(base, hanFontBody); got != 24 {
		t.Fatalf("body (1×2) want 24 got %d", got)
	}
	if got := hanFontPxForRole(base, hanFontLarge); got != 48 {
		t.Fatalf("large (2×2) want 48 got %d", got)
	}
}

// Body (1×2) rows double every dot row, like the printer's GS ! 0x01.
func TestRenderHanRoleRowBodyIsDoubleHeight(t *testing.T) {
	small := renderHanRoleRow("宫保鸡丁", 0, bitmapTextDefaultFontPx, hanFontSmall, bitmapTextStyle{})
	body := renderHanRoleRow("宫保鸡丁", 0, bitmapTextDefaultFontPx, hanFontBody, bitmapTextStyle{})
	if small.Height == 0 || body.Height != 2*small.Height {
		t.Fatalf("body height %d want 2×small %d", body.Height, small.Height)
	}
}

func TestStationTicketHanRastersByDefault(t *testing.T) {
	withThermalConfig(t, "")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines:            []jobLine{{ItemCode: "001", DisplayName: "宫保鸡丁", Qty: 2, Note: "少放盐"}},
	})
	if !bytes.Contains(raw, []byte{0x1D, 0x76, 0x30}) {
		t.Fatal("default must raster Han dish names (GS v 0)")
	}
	if bytes.Contains(raw, []byte{0x1C, 0x26}) {
		t.Fatal("default must not use printer Han font (FS &)")
	}
}

// A failed render must still print the row (Font A, '?' for Han) — never drop a dish.
func TestWriteHanRasterFallsBackWhenRenderFails(t *testing.T) {
	w := newEscpos()
	w.writeHanRaster(bitmapTextImage{}, "2\u00d7 15 宫保")
	w.lf()
	got := w.content.String()
	if got != "2\xd7 15 ??\n" {
		t.Fatalf("fallback = %q", got)
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

// Bitmap lead (TrueType, wider than Font A 2×2 cells) must never touch the name column.
func TestHanStationNameXClearsMeasuredLead(t *testing.T) {
	for _, lead := range []string{"1×", "2× 012", "12× 1024"} {
		leadW := measureHanTextWidth(lead, hanFontPxForRole(bitmapTextDefaultFontPx, hanFontLarge), false)
		x := hanStationNameX(lead, bitmapTextDefaultFontPx)
		if x < leadW+hanStationNameGapPx {
			t.Fatalf("%q: nameX %d < lead %d + gap %d", lead, x, leadW, hanStationNameGapPx)
		}
		if x < escposDisplayColToPx(stationSlipNameIndent(lead)) {
			t.Fatalf("%q: nameX %d left of Font A indent", lead, x)
		}
	}
}
