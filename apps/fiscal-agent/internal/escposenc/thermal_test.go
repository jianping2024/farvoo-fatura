package escposenc

import "testing"

func TestNormalizeThermalEncoding(t *testing.T) {
	if got := NormalizeThermalEncoding("gbk"); got != "auto" {
		t.Fatalf("gbk: %q", got)
	}
	if got := NormalizeThermalEncoding("utf-8"); got != "utf8" {
		t.Fatalf("utf-8: %q", got)
	}
	if got := NormalizeThermalEncoding("CP1252"); got != "latin" {
		t.Fatalf("CP1252: %q", got)
	}
}

func TestNeedsRasterOnlyBeyondWindows1252(t *testing.T) {
	for _, s := range []string{"TOTAL", "Água 500ml", "Preço", "Observação", "1ª Via", "Ananás", "€ 3,50"} {
		if NeedsRaster(s) {
			t.Fatalf("%q fits WPC1252 and must stay Font A", s)
		}
	}
	for _, s := range []string{"宫保鸡丁", "Água 宫保", "Борщ"} {
		if !NeedsRaster(s) {
			t.Fatalf("%q is beyond WPC1252 and must raster", s)
		}
	}
}

func TestNormalizeThermalEncodingBitmap(t *testing.T) {
	if got := NormalizeThermalEncoding(" Bitmap "); got != "bitmap" {
		t.Fatalf("bitmap: %q", got)
	}
}

func TestLinePathFor(t *testing.T) {
	cases := map[string]LinePath{
		"TOTAL":           PathLatin1252,
		"Água 500ml":      PathLatin1252,
		"宫保鸡丁":            PathHanFirmware,
		"001-宫保鸡丁      1": PathHanFirmware,
		"备注: 少放盐，不要葱（谢谢）": PathHanFirmware,
		"Água 宫保":         PathHanFirmware,
		"Observação: 少放盐": PathHanFirmware,
		"Борщ":            PathRaster,
		"宫保鸡丁 🍗":          PathRaster,
	}
	for s, want := range cases {
		if got := LinePathFor(s); got != want {
			t.Fatalf("LinePathFor(%q) = %d want %d", s, got, want)
		}
	}
}

func TestHanFirmwareLineBytes(t *testing.T) {
	// 1×2: Font A run keeps GS ! 0x01; Han run clears it, prints FS ! 0x04, then restores.
	got := HanFirmwareLine("A宫", FontASize(false, true), HanFirmwareSize(false, true))
	want := []byte{'A', 0x1D, 0x21, 0x00, 0x1C, 0x26, 0x1C, 0x21, 0x04, 0xB9, 0xAC, 0x1C, 0x21, 0x00, 0x1C, 0x2E, 0x1D, 0x21, 0x01}
	if string(got) != string(want) {
		t.Fatalf("HanFirmwareLine = % X want % X", got, want)
	}
	// pt accent outside FS & in WPC1252 (á = E1); 1×1 needs no GS ! toggling.
	got = HanFirmwareLine("á宫!", 0, 0)
	want = []byte{0xE1, 0x1C, 0x26, 0x1C, 0x21, 0x00, 0xB9, 0xAC, 0x1C, 0x21, 0x00, 0x1C, 0x2E, '!'}
	if string(got) != string(want) {
		t.Fatalf("mixed HanFirmwareLine = % X want % X", got, want)
	}
	if HanFirmwareSize(true, true) != 0x0C || HanFirmwareSize(false, false) != 0 {
		t.Fatal("FS ! size map")
	}
}
