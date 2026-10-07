package main

import (
	"strings"

	"farvoo-fiscal-agent/internal/escposbitmap"
)

type escposTextMode int

const (
	escposTextLatin escposTextMode = iota
	escposTextUTF8
	escposTextBitmap
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
	bitmapTextMaxFontPx     = escposbitmap.MaxFontPx
	bitmapTextMaxWidthPx    = escposbitmap.MaxWidthPx
)

func resolveHanBitmapFontPx(n int) int {
	return escposbitmap.ClampFontPx(n)
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
		}
	}
	return escposTextBitmap
}

func toBitmapStyle(style bitmapTextStyle) escposbitmap.Style {
	return escposbitmap.Style{
		Align:     style.Align,
		Bold:      style.Bold,
		Underline: style.Underline,
	}
}

// escposBitmapText — ONLY Mesa wrapper around escposbitmap.Line (no second rasterizer).
func escposBitmapText(s string, style bitmapTextStyle, fontPx int) []byte {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	return escposbitmap.Line(s, toBitmapStyle(style), fontPx)
}

// renderBitmapText adapts escposbitmap.RenderImage for Han column / ink tests (not a second encoder).
func renderBitmapText(s string, style bitmapTextStyle, fontPx int) bitmapTextImage {
	img := escposbitmap.RenderImage(s, toBitmapStyle(style), fontPx)
	return bitmapTextImage{Width: img.Width, Height: img.Height, Pixels: img.Pixels}
}
