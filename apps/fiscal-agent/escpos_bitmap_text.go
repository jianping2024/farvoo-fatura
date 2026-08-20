package main

import "strings"

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

// Han bitmap TrueType size: default when job omits han_bitmap_font_px; clamp matches web.
const (
	bitmapTextDefaultFontPx = 24
	bitmapTextMinFontPx     = 16
	bitmapTextMaxFontPx     = 40
	// POS-80 / 80mm printable width: Font A 48 cols × 12 dots = 576 (never 384 = 48×8).
	bitmapTextMaxWidthPx = 576
)

func resolveHanBitmapFontPx(n int) int {
	if n < bitmapTextMinFontPx {
		if n <= 0 {
			return bitmapTextDefaultFontPx
		}
		return bitmapTextMinFontPx
	}
	if n > bitmapTextMaxFontPx {
		return bitmapTextMaxFontPx
	}
	return n
}

func textModeForConfiguredChinese(needChinese bool) escposTextMode {
	if !needChinese {
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

func needsBitmapText(s string) bool {
	return hasHan(s)
}

// escposBitmapText renders s as one or more GS v 0 rasters. Over-wide strings are
// wrapDisplay'd (never truncateDisplay) so every rune is emitted.
func escposBitmapText(s string, style bitmapTextStyle, fontPx int) []byte {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	fontPx = resolveHanBitmapFontPx(fontPx)
	maxCols := bitmapMaxDisplayCols(fontPx)
	chunks := wrapDisplay(s, maxCols)
	if len(chunks) == 0 {
		return nil
	}
	var out []byte
	for _, chunk := range chunks {
		out = append(out, escposBitmapTextOne(chunk, style, fontPx)...)
	}
	return out
}

func escposBitmapTextOne(s string, style bitmapTextStyle, fontPx int) []byte {
	img := renderBitmapText(s, style, fontPx)
	if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
		return encodeWindows1252(s)
	}
	return escposBitmapRaster(img, style.Align)
}
