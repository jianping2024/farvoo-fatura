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

func TestHasNonASCIIPortuguese(t *testing.T) {
	if HasNonASCII("Recibo") {
		t.Fatal("ASCII must be false")
	}
	if !HasNonASCII("Observação") {
		t.Fatal("ã must be non-ASCII")
	}
	if !HasNonASCII("宫保") {
		t.Fatal("Han must be non-ASCII")
	}
}

func TestLineUsesRasterAutoOnly(t *testing.T) {
	if !LineUsesRaster("auto", "Preço") {
		t.Fatal("auto+accent must raster")
	}
	if LineUsesRaster("latin", "Preço") {
		t.Fatal("latin must not raster via LineUsesRaster")
	}
	if LineUsesRaster("utf8", "Preço") {
		t.Fatal("utf8 must not raster via LineUsesRaster")
	}
	if LineUsesRaster("auto", "TOTAL") {
		t.Fatal("ASCII under auto stays Font A")
	}
}
