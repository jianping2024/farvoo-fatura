package main

import (
	"bytes"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/escposenc"
)

// assertAccentsInFirmwareFont — pt accents ride WPC1252 Font A: no GS v 0, no UTF-8 on the wire.
func assertAccentsInFirmwareFont(t *testing.T, raw []byte, words ...string) {
	t.Helper()
	if bytes.Contains(raw, []byte{0x1D, 0x76, 0x30}) {
		t.Fatal("pt accents must not raster (GS v 0)")
	}
	for _, w := range words {
		if !bytes.Contains(raw, escposenc.Windows1252(w)) {
			t.Fatalf("missing Windows-1252 %q", w)
		}
		if bytes.Contains(raw, []byte(w)) {
			t.Fatalf("raw UTF-8 %q on wire", w)
		}
	}
}

func TestEncodeWindows1252Portuguese(t *testing.T) {
	raw := encodeWindows1252("ção")
	if len(raw) != 3 || raw[0] != 0xe7 || raw[1] != 0xe3 || raw[2] != 0x6f {
		t.Fatalf("Windows-1252 encoding: got % x", raw)
	}
}

func TestStationTicketNeedsBitmap(t *testing.T) {
	if !stationTicketNeedsBitmap(jobPayload{Locale: "pt", Lines: []jobLine{{DisplayName: "宫保鸡丁"}}}) {
		t.Fatal("expected bitmap for Chinese dish on station slip")
	}
	if stationTicketNeedsBitmap(jobPayload{Locale: "pt", RestaurantName: "川味", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("station slip ignores restaurant_name; pt chrome fits WPC1252")
	}
	if stationTicketNeedsBitmap(jobPayload{Locale: "en", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("en + ASCII dish must stay Latin mode")
	}
	if stationTicketNeedsBitmap(jobPayload{Locale: "pt", Lines: []jobLine{{DisplayName: "Água 500ml", Note: "Observação"}}}) {
		t.Fatal("pt accents fit WPC1252 and must stay Font A")
	}
	if !stationTicketNeedsBitmap(jobPayload{Locale: "zh", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("zh locale station slip needs bitmap")
	}
}

func TestReceiptTicketNeedsBitmapPayer(t *testing.T) {
	if !receiptTicketNeedsBitmap(jobPayload{PayerName: "王小明"}) {
		t.Fatal("expected bitmap for custom Chinese payer name")
	}
	if receiptTicketNeedsBitmap(jobPayload{Locale: "en", PayerName: "2", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("expected Latin for numeric placeholder payer + ASCII en")
	}
	if receiptTicketNeedsBitmap(jobPayload{Locale: "en", PayerName: "客人 2", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("split placeholder payer should use Latin after formatting")
	}
	if receiptTicketNeedsBitmap(jobPayload{Locale: "pt", PayerName: "João", Lines: []jobLine{{DisplayName: "Limão"}}}) {
		t.Fatal("pt receipt accents fit WPC1252 and must stay Font A")
	}
}

func TestReceiptTicketNeedsBitmapIgnoresRestaurantName(t *testing.T) {
	// restaurant_name alone must not flip mode; only content beyond WPC1252 does.
	if receiptTicketNeedsBitmap(jobPayload{
		Locale:         "en",
		RestaurantName: "川味餐厅",
		Lines:          []jobLine{{DisplayName: "Soup"}},
	}) {
		t.Fatal("en ASCII dish must ignore Chinese restaurant_name")
	}
	if receiptTicketNeedsBitmap(jobPayload{
		Locale:         "pt",
		RestaurantName: "川味餐厅",
		Lines:          []jobLine{{DisplayName: "Chá camomila"}},
	}) {
		t.Fatal("Portuguese menu with Chinese restaurant_name must stay Latin")
	}
}

func TestConnectionTestNeedsBitmap(t *testing.T) {
	if !connectionTestNeedsBitmap(jobPayload{RestaurantName: "川味"}) {
		t.Fatal("connection test should use bitmap when venue name has Han")
	}
	if !connectionTestNeedsBitmap(jobPayload{Locale: "zh", RestaurantName: "restaurant-ordering.vercel.app"}) {
		t.Fatal("zh locale test slip needs bitmap for Chinese headline")
	}
	if !connectionTestNeedsBitmap(jobPayload{Locale: "zh-CN", RestaurantName: "mesa.example.com"}) {
		t.Fatal("zh-CN locale test slip needs bitmap")
	}
	if !receiptTicketNeedsBitmap(jobPayload{Locale: "zh", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("zh locale receipt must use bitmap even for ASCII-only lines")
	}
	if connectionTestNeedsBitmap(jobPayload{Locale: "en", RestaurantName: "Mesa Lisboa"}) {
		t.Fatal("en locale + ASCII venue should use Latin on connection test")
	}
	if connectionTestNeedsBitmap(jobPayload{Locale: "pt", RestaurantName: "Mesa Lisboa"}) {
		t.Fatal("pt connection test (IMPRESSÃO) fits WPC1252 and must stay Font A")
	}
}

func TestFormatSplitPayerForReceipt(t *testing.T) {
	if got := formatSplitPayerForReceipt("客人 2"); got != "2" {
		t.Fatalf("客人 2: got %q", got)
	}
	if got := formatSplitPayerForReceipt("Guest 3"); got != "3" {
		t.Fatalf("Guest 3: got %q", got)
	}
	if got := formatSplitPayerForReceipt("Maria"); got != "Maria" {
		t.Fatalf("custom name: got %q", got)
	}
}

func TestEncodeWindows1252OmitsUnmappable(t *testing.T) {
	raw := encodeWindows1252("客人")
	if strings.Contains(string(raw), "?") {
		t.Fatalf("must not emit question marks, got %q", raw)
	}
}
