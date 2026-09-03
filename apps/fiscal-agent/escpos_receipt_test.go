package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildOrderReceiptPortuguesePrintLocale(t *testing.T) {
	payloadMap := jobPayload{
		Locale:           "pt",
		RestaurantName:   "川味餐厅",
		TableDisplayName: "1",
		GuestCount:       4,
		OrderTime:        "2026-05-14 20:05",
		PrintTime:        "2026-05-14 21:01",
		Subtotal:         13.75,
		AmountDue:        13.75,
		AmountPaid:       13.75,
		PaymentMethod:    "Cash",
		ReceiptVariant:   "final",
		Lines: []jobLine{
			{ItemIndex: 1, DisplayName: "Agua 500ml", Qty: 1, UnitPrice: 1.85},
			{ItemIndex: 9, DisplayName: "Ice Tea Limão", Qty: 1, UnitPrice: 2.2},
		},
	}
	rawBytes, _ := json.Marshal(payloadMap)
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: rawBytes})
	s := string(raw)
	for _, want := range []string{
		"restaurant",
		"Recibo",
		"Mesa n.\xba:01",
		"Conv.:4",
		"Pre\xe7o",
		"Agua 500ml",
		"Detalhe taxas",
		"Pre\xe7o original",
		"A pagar:13.75",
		"Valor pago:13.75",
		"-Cash Payment:13.75",
		"Pedido por:Cliente/Estabelecimento",
		"Hora pedido:2026-05-14 20:05",
		"Impresso por:restaurant",
		"Hora impress\xe3o:2026-05-14 21:01",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in receipt output", want)
		}
	}
	if strings.Contains(s, "Receipt") || strings.Contains(s, "Table No.") {
		t.Fatalf("pt locale must not use English chrome, got: %q", s)
	}
}

func TestBuildOrderReceiptEnglishLayout(t *testing.T) {
	payloadMap := jobPayload{
		Locale:           "en",
		RestaurantName:   "Demo",
		TableDisplayName: "1",
		GuestCount:       4,
		OrderTime:        "2026-05-14 20:05",
		PrintTime:        "2026-05-14 21:01",
		Subtotal:         13.75,
		AmountDue:        13.75,
		AmountPaid:       13.75,
		PaymentMethod:    "Cash",
		ReceiptVariant:   "final",
		Lines: []jobLine{
			{ItemIndex: 1, DisplayName: "Water", Qty: 1, UnitPrice: 1.85},
		},
	}
	rawBytes, _ := json.Marshal(payloadMap)
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: rawBytes})
	s := string(raw)
	for _, want := range []string{
		"Receipt",
		"Table No.:01",
		"Guest:4",
		"Fee Details",
		"Amount Due:13.75",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in receipt output", want)
		}
	}
}

func TestReceiptItemsHeaderFollowsMenuSeparator(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":          "en",
		"display_name":    "01",
		"guest_count":     2,
		"receipt_variant": "pre_bill",
		"subtotal":        10.0,
		"amount_due":      10.0,
		"lines": []map[string]any{
			{"item_index": 1, "display_name": "Water", "qty": 1, "unit_price": 10.0},
		},
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	s := string(raw)
	lab := printTicketLabels("en")
	itemsLine := escposThreeColLine(lab.items, lab.qty, lab.originalPrice)
	idx := strings.Index(s, itemsLine)
	if idx < 0 {
		t.Fatalf("missing Items header line %q", itemsLine)
	}
	dash := strings.Repeat("-", escposWidth)
	lastDash := strings.LastIndex(s[:idx], dash)
	if lastDash < 0 {
		t.Fatal("missing menu separator before Items")
	}
	gap := s[lastDash+len(dash) : idx]
	if strings.Count(gap, "\n") != 1 {
		t.Fatalf("menu separator → Items must be exactly one newline (no blank line); got %d newlines in gap %q", strings.Count(gap, "\n"), gap)
	}
}

func TestBuildOrderReceiptBuffetShareQtyLabel(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"display_name":    "008",
		"receipt_variant": "pre_bill",
		"subtotal":        127.7,
		"amount_due":      127.7,
		"lines": []map[string]any{
			{
				"item_index":       1,
				"display_name":     "Buffet livre",
				"qty":              1,
				"unit_price":       127.7,
				"share_qty_label":  "A4-C2",
			},
		},
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	s := string(raw)
	if !strings.Contains(s, "A4-C2") {
		t.Fatalf("buffet receipt must show headcount A4-C2, got: %q", s)
	}
	if !strings.Contains(s, "127.70") {
		t.Fatalf("buffet receipt must show line total 127.70, got: %q", s)
	}
}

func TestBuildOrderReceiptSplitPaymentShareQtyLabel(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"display_name":    "A-05",
		"receipt_variant": "split_payment",
		"payer_name":      "1",
		"amount_due":      1.0,
		"amount_paid":     1.0,
		"payment_method":  "Cash",
		"lines": []map[string]any{
			{
				"item_index":       1,
				"display_name":     "Coca-Cola",
				"qty":              1,
				"unit_price":       1.0,
				"share_qty_label":  "1/3",
			},
		},
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	s := string(raw)
	if !strings.Contains(s, "1/3") {
		t.Fatalf("split receipt must show share qty 1/3, got: %q", s)
	}
	if !strings.Contains(s, "1.00") {
		t.Fatalf("split receipt must show share price 1.00, got: %q", s)
	}
}

func TestBuildOrderReceiptSplitPaymentGuestNumber(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":          "en",
		"display_name":    "A-05",
		"table_id":        "550e8400-e29b-41d4-a716-446655440000",
		"receipt_variant": "split_payment",
		"payer_name":      "客人 2",
		"amount_due":      38.62,
		"amount_paid":     38.62,
		"payment_method":  "Cash",
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	s := string(raw)
	if strings.Contains(s, "??") {
		t.Fatalf("must not contain ?? placeholders: %q", s)
	}
	if strings.Contains(s, "550e8400") {
		t.Fatalf("must not print table_id UUID on receipt: %q", s)
	}
	if !strings.Contains(s, "Guest:2") {
		t.Fatalf("expected Guest:2, got excerpt around guest: %q", s)
	}
	if !strings.Contains(s, "Table No.:A-05") {
		t.Fatalf("split receipt must show display_name, got: %q", s)
	}
}

func TestParseJobPayloadDisplayNameFromJSON(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"display_name": "A-01", "table_id": "550e8400-e29b-41d4-a716-446655440000"})
	p := parseJobPayload(printJob{Payload: raw})
	if p.TableDisplayName != "A-01" {
		t.Fatalf("expected A-01, got %q", p.TableDisplayName)
	}
	if p.TableID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected table_id, got %q", p.TableID)
	}
}

func TestCheckoutBillOmitsPaymentLines(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":          "en",
		"display_name":    "A-02",
		"receipt_variant": "checkout_bill",
		"subtotal":        100,
		"amount_due":      90,
		"lines":           []jobLine{{ItemIndex: 1, DisplayName: "Soup", Qty: 1, UnitPrice: 100}},
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	s := string(raw)
	if !strings.Contains(s, "Receipt") {
		t.Fatalf("checkout_bill title must be Receipt, got: %q", s)
	}
	if strings.Contains(s, "Pre-Bill") || strings.Contains(s, "Table Consultation") || strings.Contains(s, "Consulta Mesa") {
		t.Fatal("checkout_bill must not use pre_bill title")
	}
	if strings.Contains(s, "NOT AN INVOICE") || strings.Contains(s, "SERVE DE FATURA") {
		t.Fatal("checkout_bill must not print Consulta de Mesa legal block")
	}
	if strings.Contains(s, "Amount Paid:") || strings.Contains(s, "Payment:") {
		t.Fatal("checkout_bill must not include payment confirmation lines")
	}
	if !strings.Contains(s, "Amount Due:90.00") {
		t.Fatalf("checkout_bill must show discounted amount due, got: %q", s)
	}
}

func TestPreBillOmitsPaymentLines(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":       "pt",
		"display_name": "A-02",
		"subtotal":     10,
		"amount_due":   10,
		"lines":        []jobLine{{ItemIndex: 1, DisplayName: "Soup", Qty: 1, UnitPrice: 10}},
	})
	raw := escposFromJob(printJob{Type: "pre_bill", Payload: payload})
	s := string(raw)
	if !strings.Contains(s, "Consulta Mesa") {
		t.Fatalf("pre_bill pt locale must show Consulta Mesa, got: %q", s)
	}
	if !strings.Contains(s, "SERVE DE FATURA") {
		t.Fatal("pre_bill must print not-an-invoice disclaimer")
	}
	if !strings.Contains(s, "NOME:") || !strings.Contains(s, "NIF:") {
		t.Fatal("pre_bill must print NOME and NIF fill-in lines")
	}
	if !strings.Contains(s, strings.Repeat("_", 4)) {
		t.Fatal("pre_bill fill-in lines must use underscore blanks")
	}
	if strings.Contains(s, "Amount Paid:") || strings.Contains(s, "Payment:") {
		t.Fatal("pre_bill must not include payment confirmation lines")
	}
}

func TestReceiptPortugueseMenuUsesLatinDespiteChineseRestaurant(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Locale:           "pt",
		RestaurantName:   "川味餐厅",
		TableDisplayName: "A-01",
		Lines: []jobLine{
			{ItemIndex: 7, DisplayName: "Chá camomila", Qty: 1, UnitPrice: 2.5},
			{ItemIndex: 13, DisplayName: "Chaminé", Qty: 1, UnitPrice: 3},
		},
		Subtotal:   5.5,
		AmountDue:  5.5,
	})
	for _, jobType := range []string{"pre_bill", "order_receipt"} {
		t.Run(jobType, func(t *testing.T) {
			raw := escposFromJob(printJob{Type: jobType, Payload: payload})
			if !bytes.Contains(raw, []byte{0xe1}) {
				t.Fatalf("%s: expected Windows-1252 á (0xE1) in output", jobType)
			}
			if bytes.Contains(raw, []byte{0xc3, 0xa1}) {
				t.Fatalf("%s: must not emit raw UTF-8 for á", jobType)
			}
		})
	}
}

func TestPreBillTitleEnglishLocale(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":       "en",
		"display_name": "A-03",
		"subtotal":     5,
		"amount_due":   5,
		"lines":        []jobLine{{ItemIndex: 1, DisplayName: "Tea", Qty: 1, UnitPrice: 5}},
	})
	raw := escposFromJob(printJob{Type: "pre_bill", Payload: payload})
	s := string(raw)
	if !strings.Contains(s, "Table Consultation") {
		t.Fatalf("pre_bill en locale must show Table Consultation, got: %q", s)
	}
	if !strings.Contains(s, "THIS DOCUMENT IS NOT AN INVOICE") {
		t.Fatal("pre_bill en locale must print not-an-invoice disclaimer")
	}
	if !strings.Contains(s, "Name:") || !strings.Contains(s, "NIF:") {
		t.Fatal("pre_bill en locale must print Name and NIF fill-in lines")
	}
}

func TestPreBillFillLinePadsToWidth(t *testing.T) {
	line := preBillFillLine("NOME", escposWidth)
	if displayWidth(line) != escposWidth {
		t.Fatalf("fill line width %d want %d (%q)", displayWidth(line), escposWidth, line)
	}
	if !strings.HasPrefix(line, "NOME: ") || !strings.Contains(line, "_") {
		t.Fatalf("fill line must be LABEL: ____, got %q", line)
	}
}

func TestPreBillEmptyLocaleUsesPortugueseLegalBlock(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"display_name": "A-01",
		"subtotal":     1,
		"amount_due":   1,
		"lines":        []jobLine{{ItemIndex: 1, DisplayName: "Agua", Qty: 1, UnitPrice: 1}},
	})
	s := string(escposFromJob(printJob{Type: "pre_bill", Payload: payload}))
	if !strings.Contains(s, "Consulta Mesa") || !strings.Contains(s, "SERVE DE FATURA") {
		t.Fatal("empty payload.locale must default pre-bill chrome to pt")
	}
}

func TestStationTicketOmitsPreBillLegalBlock(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Locale:           "pt",
		TableDisplayName: "A-01",
		Lines:            []jobLine{{ItemIndex: 1, DisplayName: "Agua", Qty: 1}},
	})
	s := string(escposFromJob(printJob{Type: "station_ticket", Payload: payload}))
	if strings.Contains(s, "SERVE DE FATURA") || strings.Contains(s, "Consulta Mesa") {
		t.Fatal("station_ticket must not print Consulta de Mesa legal chrome")
	}
}

func TestFinalReceiptOmitsPreBillLegalBlock(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":          "pt",
		"display_name":    "A-01",
		"receipt_variant": "final",
		"subtotal":        1,
		"amount_due":      1,
		"lines":           []jobLine{{ItemIndex: 1, DisplayName: "Agua", Qty: 1, UnitPrice: 1}},
	})
	s := string(escposFromJob(printJob{Type: "order_receipt", Payload: payload}))
	if strings.Contains(s, "SERVE DE FATURA") {
		t.Fatal("final receipt must not print not-an-invoice disclaimer")
	}
}

func TestReceiptDoesNotPrintItemNote(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Locale:           "en",
		TableDisplayName: "A-03",
		Subtotal:         5,
		AmountDue:        5,
		Lines: []jobLine{{
			ItemIndex:   1,
			DisplayName: "Tea",
			Qty:         1,
			UnitPrice:   5,
			Note:        "less sugar",
		}},
	})
	raw := escposFromJob(printJob{Type: "pre_bill", Payload: payload})
	s := string(raw)
	if strings.Contains(s, "Observ") || strings.Contains(s, "less sugar") {
		t.Fatalf("receipt must not print merged-item notes, got: %q", s)
	}
}

func TestReceiptMenuHeaderAndAmountDueUse1x2Bold(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"locale":          "en",
		"display_name":    "A-01",
		"receipt_variant": "pre_bill",
		"subtotal":        5.0,
		"amount_due":      5.0,
		"lines": []map[string]any{
			{"item_index": 1, "display_name": "001-Tea", "qty": 1, "unit_price": 5.0},
		},
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	lab := printTicketLabels("en")
	headerLine := escposThreeColLine(lab.items, lab.qty, lab.originalPrice)
	itemLine := escposThreeColLine("001-Tea", "1", "5.00")
	idx := bytes.Index(raw, []byte(headerLine))
	if idx < 0 {
		t.Fatal("missing Items header line")
	}
	itemIdx := bytes.Index(raw, []byte(itemLine))
	if itemIdx < 0 {
		t.Fatal("missing item line")
	}
	headerSegStart := idx - 16
	if headerSegStart < 0 {
		headerSegStart = 0
	}
	headerSeg := raw[headerSegStart:itemIdx]
	if !bytes.Contains(headerSeg, []byte{0x1D, 0x21, 0x01}) {
		t.Fatal("receipt column header must use GS ! 1×2")
	}
	if !bytes.Contains(headerSeg, []byte{0x1B, 0x45, 0x01}) {
		t.Fatal("receipt column header must use ESC E bold")
	}
	itemSegStart := itemIdx - 16
	if itemSegStart < idx {
		itemSegStart = idx
	}
	itemSeg := raw[itemSegStart : itemIdx+len(itemLine)]
	if !bytes.Contains(itemSeg, []byte{0x1D, 0x21, 0x01}) {
		t.Fatal("receipt item line must use GS ! 1×2")
	}
	if !bytes.Contains(itemSeg, []byte{0x1B, 0x45, 0x00}) {
		t.Fatal("receipt item line must clear ESC E bold")
	}
	amountDue := []byte("Amount Due:5.00")
	dueIdx := bytes.Index(raw, amountDue)
	if dueIdx < 0 {
		t.Fatal("missing Amount Due line")
	}
	segStart := dueIdx - 16
	if segStart < 0 {
		segStart = 0
	}
	dueSegment := raw[segStart : dueIdx+len(amountDue)]
	if !bytes.Contains(dueSegment, []byte{0x1D, 0x21, 0x01}) || !bytes.Contains(dueSegment, []byte{0x1B, 0x45, 0x01}) {
		t.Fatal("Amount Due must use 1×2 bold")
	}
}

func TestZhPreBillMenuOneRasterPerLine(t *testing.T) {
	fontPx := bitmapTextDefaultFontPx
	style := bitmapTextStyle{Bold: true}
	// Old path: pad to 48 cols then escposBitmapText/wrapDisplay → multi GS per logical line.
	padded := escposThreeColLine("001-中水", "9", "16.65")
	oldGS := bytes.Count(escposBitmapText(padded, style, fontPx), []byte{0x1D, 0x76, 0x30, 0x00})
	if oldGS < 2 {
		t.Fatalf("sanity: padded three-col wrap should be ≥2 GS, got %d", oldGS)
	}
	newGS := bytes.Count(escposHanReceiptRow("001-中水", "9", "16.65", fontPx, style), []byte{0x1D, 0x76, 0x30, 0x00})
	if newGS != 1 {
		t.Fatalf("escposHanReceiptRow must emit exactly 1 GS v 0, got %d", newGS)
	}

	payload, _ := json.Marshal(jobPayload{
		Locale:           "zh",
		TableDisplayName: "A-01",
		GuestCount:       2,
		ReceiptVariant:   "pre_bill",
		Subtotal:         16.65,
		AmountDue:        16.65,
		Lines: []jobLine{{
			ItemCode:    "001",
			ItemName:    "中水",
			DisplayName: "001-中水",
			Qty:         9,
			UnitPrice:   1.85,
		}, {
			ItemCode:    "002",
			ItemName:    "冰水 500毫升",
			DisplayName: "002-冰水 500毫升",
			Qty:         10,
			UnitPrice:   1.85,
		}},
	})
	raw := escposFromJob(printJob{Type: "pre_bill", Payload: payload})
	gs := bytes.Count(raw, []byte{0x1D, 0x76, 0x30, 0x00})
	// zh chrome + legal block + menu(header+2) + pads + footer; one GS per Han line.
	if gs < 14 || gs > 28 {
		t.Fatalf("zh pre_bill GS v 0 count %d outside one-raster-per-line band [14,28]", gs)
	}
	img := renderBitmapReceiptRow("001-中水", "9", "16.65", fontPx, style)
	if img.Width != bitmapTextMaxWidthPx {
		t.Fatalf("receipt row width %d want %d", img.Width, bitmapTextMaxWidthPx)
	}
	if !bitmapInkInXBand(img, escposDisplayColToPx(escposColItems), escposDisplayColToPx(escposColItems+escposColQty)) {
		t.Fatal("qty ink missing in mid band")
	}
	if bitmapInkMaxX(img) < bitmapTextMaxWidthPx-escposDisplayColToPx(escposColPrice) {
		t.Fatal("price ink should sit in right price band")
	}
}
