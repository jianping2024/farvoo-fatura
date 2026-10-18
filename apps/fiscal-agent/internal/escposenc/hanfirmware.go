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
	// PathHanFirmware — line with Han: Han runs FS & + GBK + FS ! size + FS . (pirata self-test B),
	// every other run (ASCII, pt accents) Font A WPC1252 on the same line.
	PathHanFirmware
	// PathRaster — anything else (Cyrillic, emoji…): GS v 0 bitmap.
	PathRaster
)

// FS ! n (Han print mode) per Mesa/FT size role. pirata SIZE self-test: 0x04 = same height as
// GS ! 0x01 on this firmware; 0x0C = double width + double height.
const (
	HanSize1x1 byte = 0x00
	HanSize1x2 byte = 0x04
	HanSize2x2 byte = 0x0C
	HanSize2x1 byte = 0x08
)

// HanFirmwareSize maps the Font A GS ! role (double width / double height) to FS ! n.
func HanFirmwareSize(doubleW, doubleH bool) byte {
	switch {
	case doubleW && doubleH:
		return HanSize2x2
	case doubleH:
		return HanSize1x2
	case doubleW:
		return HanSize2x1
	}
	return HanSize1x1
}

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

// HanFirmwareLine is the ONLY firmware Han line encoder. The line is split into runs:
//   - CJK runs (IsHanFirmwareRune): GS ! 0 + FS & + FS ! fsBang + GBK + FS ! 0 + FS . + GS ! gsBang
//     — GS ! is cleared around Han so the bytes match the pirata "ZH B FS!1x2" self-test row
//     (GS ! on Han printed short/distorted there);
//   - everything else (ASCII, pt accents): Windows-1252 under the caller's ESC t 16 + GS ! gsBang.
//
// gsBang is the Font A GS ! n the caller has active for this line. No trailing LF.
// Caller guarantees UsesHanFirmware(s).
func HanFirmwareLine(s string, gsBang, fsBang byte) []byte {
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
		if gsBang != 0 {
			out = append(out, 0x1D, 0x21, 0x00)
		}
		out = append(out, 0x1C, 0x26, 0x1C, 0x21, fsBang)
		out = append(out, gbk...)
		out = append(out, 0x1C, 0x21, 0x00, 0x1C, 0x2E)
		if gsBang != 0 {
			out = append(out, 0x1D, 0x21, gsBang)
		}
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
