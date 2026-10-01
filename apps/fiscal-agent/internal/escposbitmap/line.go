// Package escposbitmap is the ONLY line-level GS v 0 text renderer for thermal tickets
// (CJK + Latin accents). Authority: docs/fiscal-thermal-text-encoding.zh.md
package escposbitmap

import (
	"strings"
	"unicode"
)

// POS-80 printable width: Font A 48 cols × 12 dots = 576.
const (
	MaxWidthPx    = 576
	DefaultFontPx = 24
	MinFontPx     = 16
	MaxFontPx     = 40
	padY          = 1
	heightPad     = 2
)

// Style controls TrueType attributes baked into the raster.
type Style struct {
	Align     byte
	Bold      bool
	Underline bool
}

// Image is a 1-bit ink mask (1 = black) on a full-width canvas.
type Image struct {
	Width  int
	Height int
	Pixels []byte
}

// ClampFontPx matches web/Agent han_bitmap_font_px bounds.
func ClampFontPx(n int) int {
	if n < MinFontPx {
		if n <= 0 {
			return DefaultFontPx
		}
		return MinFontPx
	}
	if n > MaxFontPx {
		return MaxFontPx
	}
	return n
}

func displayCols(r rune) int {
	if r == 0 {
		return 0
	}
	if unicode.Is(unicode.Han, r) {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += displayCols(r)
	}
	return n
}

func wrapDisplay(s string, maxCols int) []string {
	if maxCols <= 0 || s == "" {
		return nil
	}
	var out []string
	var b []rune
	w := 0
	flush := func() {
		if len(b) == 0 {
			return
		}
		out = append(out, string(b))
		b = b[:0]
		w = 0
	}
	for _, r := range s {
		cw := displayCols(r)
		if cw > maxCols {
			flush()
			out = append(out, string(r))
			continue
		}
		if w+cw > maxCols {
			flush()
		}
		b = append(b, r)
		w += cw
	}
	flush()
	return out
}

func maxDisplayCols(fontPx int) int {
	fontPx = ClampFontPx(fontPx)
	colPx := fontPx / 2
	if colPx < 1 {
		colPx = 1
	}
	max := (MaxWidthPx - 8) / colPx
	if max < 1 {
		return 1
	}
	return max
}

func alignStartPx(textWidthPx int, align byte) int {
	if textWidthPx < 0 {
		textWidthPx = 0
	}
	switch align {
	case 1:
		x := (MaxWidthPx - textWidthPx) / 2
		if x < 0 {
			return 0
		}
		return x
	case 2:
		x := MaxWidthPx - textWidthPx - padY
		if x < 0 {
			return 0
		}
		return x
	default:
		return padY
	}
}

// Raster encodes img as GS v 0. Full-width canvases force ESC a 0 (alignment in pixels).
func Raster(img Image, align byte) []byte {
	if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
		return nil
	}
	if img.Width >= MaxWidthPx {
		align = 0
	}
	widthBytes := (img.Width + 7) / 8
	data := make([]byte, widthBytes*img.Height)
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			if img.Pixels[y*img.Width+x] == 0 {
				continue
			}
			data[y*widthBytes+x/8] |= 0x80 >> uint(x%8)
		}
	}
	out := []byte{
		0x1B, 0x61, align,
		0x1D, 0x76, 0x30, 0x00,
		byte(widthBytes & 0xff), byte((widthBytes >> 8) & 0xff),
		byte(img.Height & 0xff), byte((img.Height >> 8) & 0xff),
	}
	return append(out, data...)
}

// Line is the ONLY line-level thermal text rasterizer (wraps; never truncates with ellipsis).
func Line(s string, style Style, fontPx int) []byte {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	fontPx = ClampFontPx(fontPx)
	chunks := wrapDisplay(s, maxDisplayCols(fontPx))
	if len(chunks) == 0 {
		return nil
	}
	var out []byte
	for _, chunk := range chunks {
		img := renderText(chunk, style, fontPx)
		if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
			continue
		}
		out = append(out, Raster(img, style.Align)...)
	}
	return out
}

// RenderImage exposes platform text raster for Han column adapters / ink tests.
// Product line emission must use Line only.
func RenderImage(s string, style Style, fontPx int) Image {
	return renderText(s, style, ClampFontPx(fontPx))
}
