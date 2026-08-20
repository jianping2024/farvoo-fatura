package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildConnectionTestZHUsesBitmapByDefault(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		ConnectionTest: true,
		Locale:         "zh",
		RestaurantName: "mesa.example.com",
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})

	if !bytes.Contains(raw, []byte{0x1D, 0x76, 0x30, 0x00}) {
		t.Fatal("expected raster bitmap command for Chinese headline")
	}
	if bytes.Contains(raw, []byte{0x1C, 0x26}) || bytes.Contains(raw, []byte{0x1C, 0x2E}) {
		t.Fatal("bitmap mode must not enter/exit GBK Chinese mode")
	}
	if bytes.Contains(raw, []byte{0x1B, 0x39, 0x01}) {
		t.Fatal("auto mode should not use ESC 9 UTF-8 on connection test")
	}
	venue := []byte("mesa.example.com")
	if !bytes.Contains(raw, venue) {
		t.Fatal("expected venue line")
	}
	if rasterInkBits(raw) == 0 {
		t.Fatal("Chinese connection-test raster must have ink (blank wipe regression)")
	}
}

func rasterInkBits(escpos []byte) int {
	const marker = "\x1d\x76\x30\x00"
	ink := 0
	for {
		i := bytes.Index(escpos, []byte(marker))
		if i < 0 || i+8 > len(escpos) {
			break
		}
		wb := int(escpos[i+4]) | int(escpos[i+5])<<8
		h := int(escpos[i+6]) | int(escpos[i+7])<<8
		start := i + 8
		end := start + wb*h
		if wb <= 0 || h <= 0 || end > len(escpos) {
			break
		}
		for _, b := range escpos[start:end] {
			for bit := 0; bit < 8; bit++ {
				if b&(0x80>>uint(bit)) != 0 {
					ink++
				}
			}
		}
		escpos = escpos[end:]
	}
	return ink
}

func TestBuildConnectionTestZHUsesUTF8WhenConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"agentjwt":"x","text_encoding":"utf8"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := configPathOverride
	configPathOverride = path
	defer func() { configPathOverride = prev }()

	payload, _ := json.Marshal(jobPayload{
		ConnectionTest: true,
		Locale:         "zh",
	})
	raw := escposFromJob(printJob{Type: "order_receipt", Payload: payload})
	if !bytes.Contains(raw, []byte{0x1B, 0x39, 0x01}) {
		t.Fatal("expected ESC 9 when text_encoding=utf8")
	}
	want := []byte("打印测试")
	if !bytes.Contains(raw, want) {
		t.Fatalf("expected UTF-8 headline % x", want)
	}
}

func TestBuildStationTicketZHMenuUsesBitmapWithZhHeader(t *testing.T) {
	payload, _ := json.Marshal(jobPayload{
		Locale:           "zh",
		TableDisplayName: "8",
		Lines:            []jobLine{{ItemCode: "001", ItemName: "鱼香肉丝", DisplayName: "001-鱼香肉丝", Qty: 1}},
	})
	raw := escposFromJob(printJob{Type: "station_ticket", Payload: payload})
	if bytes.Contains(raw, []byte{0x1C, 0x26}) {
		t.Fatal("station ticket must not enter GBK mode")
	}
	for _, en := range []string{"Guest Order", "Items", "Qty", "Table No.", "Printed By", "Order Time"} {
		if bytes.Contains(raw, []byte(en)) {
			t.Fatalf("zh station ticket must not contain English chrome %q", en)
		}
	}
	// zh chrome + dish names are GS v 0 bitmaps (not raw UTF-8 in ESC/POS stream).
	gs := bytes.Count(raw, []byte{0x1D, 0x76, 0x30, 0x00})
	if gs < 2 {
		t.Fatalf("expected Han bitmaps for zh chrome+item, got %d GS v 0", gs)
	}
}

func TestNormalizePrintLocale(t *testing.T) {
	if normalizePrintLocale("zh-CN") != "zh" {
		t.Fatal("zh-CN")
	}
	if normalizePrintLocale("") != "pt" {
		t.Fatal("empty defaults to pt")
	}
}
