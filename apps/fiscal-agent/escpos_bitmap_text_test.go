package main

import (
	"bytes"
	"testing"
)

func TestRenderBitmapTextHasInkForHan(t *testing.T) {
	img := renderBitmapText("打印测试", bitmapTextStyle{Align: 1, Bold: true, DoubleW: true, DoubleH: true}, bitmapTextDefaultFontPx)
	if img.Width != bitmapTextMaxWidthPx {
		t.Fatalf("Han line canvas width %d want %d (full POS-80)", img.Width, bitmapTextMaxWidthPx)
	}
	if img.Height <= 0 {
		t.Fatalf("expected image height, got %dx%d", img.Width, img.Height)
	}
	// DoubleH must not enlarge Han font — size is fontPx (+padding).
	if img.Height > bitmapTextDefaultFontPx+16 {
		t.Fatalf("Han height should stay near %dpx, got %d", bitmapTextDefaultFontPx, img.Height)
	}
	ink := 0
	for _, p := range img.Pixels {
		if p != 0 {
			ink++
		}
	}
	if ink == 0 {
		t.Fatal("Han bitmap must contain ink pixels")
	}
	minX, maxX := bitmapInkMinX(img), bitmapInkMaxX(img)
	if minX < 0 || maxX < 0 {
		t.Fatal("expected ink bounds")
	}
	center := (minX + maxX) / 2
	paperCenter := bitmapTextMaxWidthPx / 2
	if center < paperCenter-48 || center > paperCenter+48 {
		t.Fatalf("centered Han ink center %d far from paper center %d (min=%d max=%d)", center, paperCenter, minX, maxX)
	}
	raw := escposBitmapText("打印测试", bitmapTextStyle{Align: 1}, bitmapTextDefaultFontPx)
	if rasterInkBits(raw) == 0 {
		t.Fatal("escposBitmapText must emit non-blank GS v 0 payload")
	}
	if bytes.Contains(raw, []byte{0x1B, 0x61, 1}) {
		t.Fatal("full-width Han raster must not rely on ESC a center")
	}
	if bytes.HasSuffix(raw, []byte{'\n'}) {
		t.Fatal("GS v 0 raster must not trailing LF (double line feed wastes paper)")
	}
}

func TestEscposBitmapTextWrapsLongHanNoEllipsis(t *testing.T) {
	long := stringsRepeatHan(40)
	raw := escposBitmapText(long, bitmapTextStyle{}, bitmapTextDefaultFontPx)
	if bytes.Contains(raw, []byte("…")) {
		t.Fatal("bitmap path must not emit ellipsis truncation")
	}
	gs := bytes.Count(raw, []byte{0x1D, 0x76, 0x30, 0x00})
	if gs < 2 {
		t.Fatalf("long Han must wrap into multiple GS v 0, got %d", gs)
	}
}

func TestResolveHanBitmapFontPx(t *testing.T) {
	if got := resolveHanBitmapFontPx(0); got != bitmapTextDefaultFontPx {
		t.Fatalf("0 -> default, got %d", got)
	}
	if got := resolveHanBitmapFontPx(28); got != 28 {
		t.Fatalf("28 -> 28, got %d", got)
	}
	if got := resolveHanBitmapFontPx(12); got != bitmapTextMinFontPx {
		t.Fatalf("below min -> min, got %d", got)
	}
	if got := resolveHanBitmapFontPx(99); got != bitmapTextMaxFontPx {
		t.Fatalf("above max -> max, got %d", got)
	}
}

func TestLargerFontWrapsSooner(t *testing.T) {
	long := stringsRepeatHan(20)
	small := escposBitmapText(long, bitmapTextStyle{}, 18)
	large := escposBitmapText(long, bitmapTextStyle{}, 32)
	smallGS := bytes.Count(small, []byte{0x1D, 0x76, 0x30, 0x00})
	largeGS := bytes.Count(large, []byte{0x1D, 0x76, 0x30, 0x00})
	if largeGS < smallGS {
		t.Fatalf("32px should wrap at least as often as 18px: small=%d large=%d", smallGS, largeGS)
	}
}

func stringsRepeatHan(n int) string {
	var b []rune
	for i := 0; i < n; i++ {
		b = append(b, '测')
	}
	return string(b)
}
