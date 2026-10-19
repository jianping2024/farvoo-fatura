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
	// MaxRenderPx — 2×2 renders at twice the base (han_bitmap_font_px) size.
	MaxRenderPx = 2 * MaxFontPx
	padY        = 1
	heightPad   = 2
	// Trim keeps this many blank dot rows above / below the ink (≈ Font A line gap).
	trimPadTop    = 2
	trimPadBottom = 4
)

// Style controls TrueType attributes baked into the raster. DoubleW/DoubleH mirror the
// caller's GS ! size so the bitmap matches firmware 1×2 / 2×2 (see Scale).
type Style struct {
	Align     byte
	Bold      bool
	Underline bool
	DoubleW   bool
	DoubleH   bool
}

// Image is a 1-bit ink mask (1 = black) on a full-width canvas.
type Image struct {
	Width  int
	Height int
	Pixels []byte
}

// Scale is the ONLY GS ! size → bitmap mapping. basePx (han_bitmap_font_px, default 24) is a
// 1×1 Han cell (24×24 dots, same as printer Han font). 1×2 renders at basePx and doubles every
// dot row — exactly what the printer does for GS ! 0x01; 2×2 renders at 2×basePx (smoother than
// doubling). 2×1 is not used on tickets and renders as 2×2.
func Scale(basePx int, doubleW, doubleH bool) (renderPx, stretchY int) {
	renderPx = ClampFontPx(basePx)
	if doubleW {
		return renderPx * 2, 1
	}
	if doubleH {
		return renderPx, 2
	}
	return renderPx, 1
}

// StretchY repeats every dot row n times (n ≤ 1 → unchanged).
func StretchY(img Image, n int) Image {
	if n <= 1 || img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
		return img
	}
	out := Image{Width: img.Width, Height: img.Height * n, Pixels: make([]byte, img.Width*img.Height*n)}
	for y := 0; y < img.Height; y++ {
		row := img.Pixels[y*img.Width : (y+1)*img.Width]
		for k := 0; k < n; k++ {
			copy(out.Pixels[(y*n+k)*img.Width:], row)
		}
	}
	return out
}

// Trim drops blank dot rows above / below the ink, keeping trimPadTop / trimPadBottom
// (TrueType line height ≈ 1.3em would otherwise waste paper). Inkless images are unchanged.
func Trim(img Image) Image {
	if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
		return img
	}
	inkRow := func(y int) bool {
		for _, p := range img.Pixels[y*img.Width : (y+1)*img.Width] {
			if p != 0 {
				return true
			}
		}
		return false
	}
	top := 0
	for top < img.Height && !inkRow(top) {
		top++
	}
	if top == img.Height {
		return img
	}
	bottom := img.Height - 1
	for bottom > top && !inkRow(bottom) {
		bottom--
	}
	h := bottom - top + 1 + trimPadTop + trimPadBottom
	out := Image{Width: img.Width, Height: h, Pixels: make([]byte, img.Width*h)}
	copy(out.Pixels[trimPadTop*img.Width:], img.Pixels[top*img.Width:(bottom+1)*img.Width])
	return out
}

// ClampRenderPx bounds the actual TrueType size (base × 2 for 2×2).
func ClampRenderPx(n int) int {
	if n <= 0 {
		return DefaultFontPx
	}
	if n < MinFontPx {
		return MinFontPx
	}
	if n > MaxRenderPx {
		return MaxRenderPx
	}
	return n
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
	fontPx = ClampRenderPx(fontPx)
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
// basePx is han_bitmap_font_px; style.DoubleW/DoubleH pick the size via Scale.
// Returns nil when any chunk fails to render — callers must fall back, never drop the line.
func Line(s string, style Style, basePx int) []byte {
	s = strings.TrimRight(s, "\r\n")
	if s == "" {
		return nil
	}
	renderPx, stretchY := Scale(basePx, style.DoubleW, style.DoubleH)
	chunks := wrapDisplay(s, maxDisplayCols(renderPx))
	if len(chunks) == 0 {
		return nil
	}
	var out []byte
	for _, chunk := range chunks {
		img := renderText(chunk, style, renderPx)
		if img.Width <= 0 || img.Height <= 0 || len(img.Pixels) != img.Width*img.Height {
			return nil
		}
		out = append(out, Raster(Trim(StretchY(img, stretchY)), style.Align)...)
	}
	return out
}

// RenderImage exposes platform text raster for Han column adapters / ink tests.
// fontPx is the actual render size (already scaled). Product line emission must use Line only.
func RenderImage(s string, style Style, fontPx int) Image {
	return renderText(s, style, ClampRenderPx(fontPx))
}
