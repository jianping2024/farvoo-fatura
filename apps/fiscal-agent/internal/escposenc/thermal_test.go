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
