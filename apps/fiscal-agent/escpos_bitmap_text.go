package main

import (
	"strings"

	"farvoo-fiscal-agent/internal/escposbitmap"
)

type escposTextMode int

const (
	escposTextLatin escposTextMode = iota
	escposTextUTF8
	// escposTextBitmap — text_encoding=auto (default): every NeedsRaster line → GS v 0, Han column canvas.
	escposTextBitmap
	// escposTextHanFirmware — text_encoding=han_firmware opt-in: per-line escposenc.LinePathFor
	// (Font A / printer Han font / raster). Only for printers whose code page probe passes B.
	escposTextHanFirmware
)

type bitmapTextStyle struct {
	Align     byte
	Bold      bool
	Underline bool
	DoubleW   bool
	DoubleH   bool
}

type bitmapTextImage struct {
	Width  int
	Height int
	Pixels []byte
}

const (
	bitmapTextDefaultFontPx = escposbitmap.DefaultFontPx
	bitmapTextMinFontPx     = escposbitmap.MinFontPx
	bitmapTextMaxWidthPx    = escposbitmap.MaxWidthPx
)

// resolveHanBitmapFontPx clamps the configured base (han_bitmap_font_px = Han 1×1 cell).
func resolveHanBitmapFontPx(n int) int {
	return escposbitmap.ClampFontPx(n)
}

// clampHanRenderPx clamps an already-scaled render size (2×2 = 2× base).
func clampHanRenderPx(n int) int {
	return escposbitmap.ClampRenderPx(n)
}

// textModeForThermal is the ONLY Mesa ticket mode picker (docs/fiscal-thermal-text-encoding.zh.md).
func textModeForThermal(needFirmwareSafeRaster bool) escposTextMode {
	if !needFirmwareSafeRaster {
		return escposTextLatin
	}
	cfg, err := loadConfig(defaultConfigPath())
	if err == nil && cfg != nil {
		switch normalizeTextEncoding(cfg.TextEncoding) {
		case "utf8":
			return escposTextUTF8
		case "latin":
			return escposTextLatin
		case "han_firmware":
			return escposTextHanFirmware
		}
	}
	return escposTextBitmap
}

func toBitmapStyle(style bitmapTextStyle) escposbitmap.Style {
	return escposbitmap.Style{
		Align:     style.Align,
		Bold:      style.Bold,
		Underline: style.Underline,
		DoubleW:   style.DoubleW,
		DoubleH:   style.DoubleH,
	}
}

// escposBitmapText — ONLY Mesa wrapper around escposbitmap.Line (no second rasterizer).
// basePx = han_bitmap_font_px; style.DoubleW/DoubleH pick 1×1 / 1×2 / 2×2.
func escposBitmapText(s string, style bitmapTextStyle, basePx int) []byte {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	return escposbitmap.Line(s, toBitmapStyle(style), basePx)
}

// renderBitmapText adapts escposbitmap.RenderImage for Han column / ink tests (not a second encoder).
func renderBitmapText(s string, style bitmapTextStyle, fontPx int) bitmapTextImage {
	img := escposbitmap.RenderImage(s, toBitmapStyle(style), fontPx)
	return bitmapTextImage{Width: img.Width, Height: img.Height, Pixels: img.Pixels}
}
