package main

import (
	"strings"
	"testing"
)

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
		t.Fatal("station slip ignores restaurant_name; ASCII dish stays Latin")
	}
	if !stationTicketNeedsBitmap(jobPayload{Locale: "zh", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("zh locale station slip needs bitmap")
	}
}

func TestReceiptTicketNeedsBitmapPayer(t *testing.T) {
	if !receiptTicketNeedsBitmap(jobPayload{PayerName: "王小明"}) {
		t.Fatal("expected bitmap for custom Chinese payer name")
	}
	if receiptTicketNeedsBitmap(jobPayload{PayerName: "2", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("expected Latin for numeric placeholder payer")
	}
	if receiptTicketNeedsBitmap(jobPayload{PayerName: "客人 2", Lines: []jobLine{{DisplayName: "Soup"}}}) {
		t.Fatal("split placeholder payer should use Latin after formatting")
	}
}

func TestReceiptTicketNeedsBitmapIgnoresRestaurantName(t *testing.T) {
	if receiptTicketNeedsBitmap(jobPayload{
		RestaurantName: "川味餐厅",
		Lines:          []jobLine{{DisplayName: "Chá camomila"}},
	}) {
		t.Fatal("Portuguese menu with Chinese restaurant_name in payload must stay Latin")
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
