package escposenc

import (
	"unicode"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// LinePath is the ONLY per-line thermal path for text_encoding=auto
// (docs/fiscal-thermal-text-encoding.zh.md §3).
type LinePath int

const (
	// PathLatin1252 — whole line fits WPC1252: Font A + ESC t 16 + GS ! size.
	PathLatin1252 LinePath = iota
	// PathHanFirmware — line with Han: Han runs FS & + GBK + GS ! size + FS .
	// (store SIZE self-test: ZH B GS!1x2 = tall/thin; FS ! 0x04 = flat/wide — do not use for size).
	PathHanFirmware
	// PathRaster — anything else (Cyrillic, emoji…): GS v 0 bitmap.
	PathRaster
)

// HanUnderline is FS ! bit 7 only — Kanji underline for notes. Never encode height/width in FS !.
const HanUnderline byte = 0x80

// IsHanFirmwareRune — CJK glyphs printed from the printer's Han font (Han ideographs, CJK
// punctuation, full-width forms). Counted as 2 Font A columns.
func IsHanFirmwareRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3000 && r <= 0x303F) || // CJK symbols and punctuation (、。「」)
		(r >= 0xFF01 && r <= 0xFF5E) // full-width ASCII forms (（），：)
}

var gbkEncoder = simplifiedchinese.GBK.NewEncoder()

func gbkEncodable(r rune) bool {
	_, err := gbkEncoder.String(string(r))
	return err == nil
}

func windows1252Encodable(r rune) bool {
	_, ok := charmap.Windows1252.EncodeRune(r)
	return ok
}

// UsesHanFirmware is the ONLY "firmware Han line" test: ≥1 Han ideograph; every rune is either a
// GBK-encodable CJK rune (IsHanFirmwareRune) or fits WPC1252 (ASCII, pt accents).
func UsesHanFirmware(s string) bool {
	han := false
	for _, r := range s {
		if IsHanFirmwareRune(r) {
			if !gbkEncodable(r) {
				return false
			}
			if unicode.Is(unicode.Han, r) {
				han = true
			}
			continue
		}
		if r > unicode.MaxASCII && !windows1252Encodable(r) {
			return false
		}
	}
	return han
}

// LinePathFor picks the auto path for one printed line.
func LinePathFor(s string) LinePath {
	if !NeedsRaster(s) {
		return PathLatin1252
	}
	if UsesHanFirmware(s) {
		return PathHanFirmware
	}
	return PathRaster
}

// HanFirmwareLine is the ONLY firmware Han line encoder. Bytes match store ZH B GS!1x2:
//   - CJK runs (IsHanFirmwareRune): FS & + GS ! gsBang + [FS ! hanMode] + GBK + [FS ! 0] + FS .
//   - everything else (ASCII, pt accents): Windows-1252 under the caller's ESC t 16 + GS ! gsBang.
//
// gsBang is Font A GS ! n (body 1×2 = 0x01, masthead 2×2 = 0x11). hanMode is ONLY non-size
// FS ! bits (HanUnderline); size must not use FS ! — that flattens Chinese on these printers.
// No trailing LF. Caller guarantees UsesHanFirmware(s).
func HanFirmwareLine(s string, gsBang, hanMode byte) []byte {
	var out []byte
	var run []rune
	runHan := false
	flush := func() {
		if len(run) == 0 {
			return
		}
		if !runHan {
			out = append(out, Windows1252(string(run))...)
			run = run[:0]
			return
		}
		gbk, _ := gbkEncoder.Bytes([]byte(string(run)))
		out = append(out, 0x1C, 0x26) // FS &
		if gsBang != 0 {
			out = append(out, 0x1D, 0x21, gsBang) // GS ! — same as Font A / ZH B GS!1x2
		}
		if hanMode != 0 {
			out = append(out, 0x1C, 0x21, hanMode) // FS ! underline only
		}
		out = append(out, gbk...)
		if hanMode != 0 {
			out = append(out, 0x1C, 0x21, 0x00)
		}
		out = append(out, 0x1C, 0x2E) // FS .
		run = run[:0]
	}
	for _, r := range s {
		if h := IsHanFirmwareRune(r); h != runHan {
			flush()
			runHan = h
		}
		run = append(run, r)
	}
	flush()
	return out
}

// FontASize is GS ! n for Font A double width / double height (same bits as escposWriter.size).
func FontASize(doubleW, doubleH bool) byte {
	n := byte(0)
	if doubleH {
		n |= 0x01
	}
	if doubleW {
		n |= 0x10
	}
	return n
}
