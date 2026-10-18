package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/escposenc"
)

var (
	gsReverseOn  = []byte{0x1D, 0x42, 0x01}
	gsReverseOff = []byte{0x1D, 0x42, 0x00}
	gsV0         = []byte{0x1D, 0x76, 0x30, 0x00}
)

func stationTicketRaw(t *testing.T, p jobPayload) []byte {
	t.Helper()
	payload, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return escposFromJob(printJob{Type: "station_ticket", Payload: payload})
}

// reverseBlock is the exact Font A bytes of one dish lead: GS B 1 + " 1× 001 " + GS B 0.
func reverseBlock(qty int, code string) []byte {
	b := append([]byte{}, gsReverseOn...)
	b = append(b, escposenc.Windows1252(stationSlipQtyCodeBlock(qty, code))...)
	return append(b, gsReverseOff...)
}

func TestBuildStationTicketPortuguesePrintLocale(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
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
				CategoryGroupHeader: "(Bebidas/ Drinks2)",
			},
			{
				ItemIndex:           7,
				ItemCode:            "007",
				ItemName:            "Coca Cola Zero",
				DisplayName:         "007-Coca Cola Zero",
				Qty:                 2,
				CategoryGroupHeader: "(Bebidas/ Drinks2)",
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
		"(Bebidas/ Drinks2)",
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

	if !bytes.Contains(raw, append(reverseBlock(1, "001"), escposenc.Windows1252(" Água 500ml")...)) {
		t.Fatal("dish row must be reverse qty+code block followed by the name")
	}
	if !bytes.Contains(raw, reverseBlock(2, "007")) {
		t.Fatal("second dish must carry its own qty+code block")
	}
	idx := bytes.Index(raw, gsReverseOn)
	if !bytes.Contains(raw[idx:], []byte{0x1D, 0x21, 0x01}) || bytes.Contains(raw[idx:], []byte{0x1D, 0x21, 0x11}) {
		t.Fatal("menu body must stay GS ! 1×2 (never 2×2)")
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

func TestStationSlipSkipsCategoryHeaderWhenDisabled(t *testing.T) {
	raw := stationTicketRaw(t, jobPayload{
		TableDisplayName: "1",
		StationSlipOptions: &stationSlipOptions{
			ShowCategoryGroup: false,
		},
		Lines: []jobLine{{
			ItemCode:            "001",
			ItemName:            "Soup",
			Qty:                 1,
			CategoryGroupHeader: "(Bebidas/ Drinks2)",
		}},
	})
	if strings.Contains(string(raw), "(Bebidas/ Drinks2)") {
		t.Fatal("category header must not print when show_category_group is false")
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

func TestStationSlipQtyCodeBlock(t *testing.T) {
	if got := stationSlipQtyCodeBlock(1, "001"); got != " 1× 001 " {
		t.Fatalf("block %q", got)
	}
	if got := stationSlipQtyCodeBlock(12, ""); got != " 12× " {
		t.Fatalf("no-code block %q", got)
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
	indent := strings.Repeat(" ", displayWidth(stationSlipQtyCodeBlock(7, "020"))+1)
	first := bytes.Index(raw, append(reverseBlock(7, "020"), []byte(" Frango Piri-Piri")...))
	second := bytes.Index(raw, []byte(indent+"Grilled chicken"))
	note := bytes.Index(raw, escposenc.Windows1252(indent+"» sem sal"))
	if first < 0 || second < 0 || note < 0 || !(first < second && second < note) {
		t.Fatalf("want block+name < extra name < note, all hung under name; got %d %d %d", first, second, note)
	}
	if bytes.Count(raw, gsReverseOn) != 1 {
		t.Fatal("only the first row carries the qty+code block")
	}
	if bytes.Contains(raw, []byte{0x1B, 0x2D, 0x01}) {
		t.Fatal("notes are bold, not underlined")
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
	indent := displayWidth(stationSlipQtyCodeBlock(2, "012")) + 1
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

func TestStationTicketHanMenuUsesFirmware1x2ByDefault(t *testing.T) {
	withThermalConfig(t, "")
	raw := stationTicketRaw(t, jobPayload{
		Lines: []jobLine{{
			ItemCode:   "001",
			ItemName:   "Água 500ml",
			Qty:        1,
			PrintNames: []string{"Água 500ml", "宫保鸡丁"},
		}},
	})
	gbk := []byte{0xB9, 0xAC, 0xB1, 0xA3} // 宫保
	if !bytes.Contains(raw, append([]byte{0x1C, 0x26, 0x1D, 0x21, 0x01}, gbk...)) {
		t.Fatal("Han dish name must be FS & + GS ! 0x01 (1×2) + GBK")
	}
	if bytes.Contains(raw, gsV0) {
		t.Fatal("auto must not raster Han dish names")
	}
	if !bytes.Contains(raw, append(reverseBlock(1, "001"), escposenc.Windows1252(" Água 500ml")...)) {
		t.Fatal("pt name must stay Font A WPC1252 after the reverse block")
	}
}

func TestStationTicketHanNameOnBlockRowStaysFirmware(t *testing.T) {
	withThermalConfig(t, "")
	raw := stationTicketRaw(t, jobPayload{
		Locale:           "pt",
		TableDisplayName: "001",
		Lines:            []jobLine{{ItemCode: "001", ItemName: "中水", Qty: 1, Note: "正常冰"}},
	})
	// reverse block (Font A) then firmware Han name on the same row
	want := append(reverseBlock(1, "001"), ' ', 0x1C, 0x26)
	if !bytes.Contains(raw, want) {
		t.Fatal("Han name must follow the reverse block on the same row via FS &")
	}
	if bytes.Contains(raw, gsV0) {
		t.Fatal("auto must not raster Han name or note")
	}
}

func TestStationTicketPtNoteWithHanStaysFirmware(t *testing.T) {
	withThermalConfig(t, "")
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

func TestStationSlipColumnBlockUsesHanCanvas_ZhForcesLatinMenu(t *testing.T) {
	withThermalConfig(t, "bitmap") // Han canvas exists only on the bitmap escape path
	zh := jobPayload{Locale: "zh", Lines: []jobLine{{ItemCode: "031", ItemName: "Guarana", Qty: 1}}}
	if !stationSlipColumnBlockUsesHanCanvas(zh, newEscposForStationTicket(zh)) {
		t.Fatal("zh chrome must use the Han canvas even when dish names are Latin")
	}
	en := jobPayload{Locale: "en", Lines: []jobLine{{ItemCode: "031", ItemName: "Guarana", Qty: 1}}}
	if stationSlipColumnBlockUsesHanCanvas(en, newEscposForStationTicket(en)) {
		t.Fatal("en Latin menu must stay on Font A")
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
	// 2 dish rows + 1 note row on the canvas
	if n := bytes.Count(raw, gsV0); n < 3 {
		t.Fatalf("GS v 0 count %d want >= 3", n)
	}
	if bytes.Contains(raw, gsReverseOn) {
		t.Fatal("canvas rows invert pixels; GS B must not be used")
	}
}

func TestHanStationItemInvertsBlockBand(t *testing.T) {
	fontPx := bitmapTextDefaultFontPx
	spacePx := measureHanTextWidth(" ", fontPx, true)
	blockText := strings.TrimSpace(stationSlipQtyCodeBlock(1, "001"))
	blockEnd := spacePx*2 + measureHanTextWidth(blockText, fontPx, true)
	row := mergeBitmapRows(
		renderBitmapLeftRow(blockText, spacePx, fontPx, bitmapTextStyle{Bold: true}),
		renderBitmapLeftRow("中水", blockEnd+spacePx, fontPx, bitmapTextStyle{Bold: true}),
	)
	invertBitmapBand(row, 0, blockEnd)
	if row.Width != bitmapTextMaxWidthPx {
		t.Fatalf("canvas width %d want %d", row.Width, bitmapTextMaxWidthPx)
	}
	// background of the block band is ink (reverse); the gap before the name is blank
	if row.Pixels[0] == 0 {
		t.Fatal("block band must be inverted (ink at top-left)")
	}
	if bitmapInkInXBand(row, blockEnd, blockEnd+spacePx/2) {
		t.Fatal("gap between block and name must stay blank")
	}
	if !bitmapInkInXBand(row, blockEnd+spacePx, bitmapTextMaxWidthPx) {
		t.Fatal("name ink missing after the block")
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
