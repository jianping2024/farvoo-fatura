package escposenc

import (
	"unicode"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// LinePath is the ONLY per-line thermal path for text_encoding=auto
// (docs/fiscal-thermal-text-encoding.zh.md §3).
type LinePath int

const (
	// PathLatin1252 — whole line fits WPC1252: Font A + ESC t 16 + GS ! size.
	PathLatin1252 LinePath = iota
	// PathHanFirmware — Han line: FS & + GBK + FS ! size + FS . (pirata self-test B).
	PathHanFirmware
	// PathRaster — anything else (Cyrillic, pt accent + Han on one line…): GS v 0 bitmap.
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

// UsesHanFirmware is the ONLY "firmware Han line" test: ≥1 Han ideograph, every other rune
// ASCII or a GBK-encodable CJK rune (IsHanFirmwareRune). pt accents never mix in (→ raster).
func UsesHanFirmware(s string) bool {
	han := false
	for _, r := range s {
		if r <= unicode.MaxASCII {
			continue
		}
		if !IsHanFirmwareRune(r) {
			return false
		}
		if _, err := gbkEncoder.String(string(r)); err != nil {
			return false
		}
		if unicode.Is(unicode.Han, r) {
			han = true
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

// HanFirmwareLine is the ONLY firmware Han line encoder: FS & + FS ! n + GBK(s) + FS ! 0 + FS .
// (same byte order as SizeProbeRows "ZH B FS!1x2"). No trailing LF. Caller guarantees
// UsesHanFirmware(s).
func HanFirmwareLine(s string, fsBangN byte) []byte {
	gbk, _ := gbkEncoder.Bytes([]byte(s))
	out := make([]byte, 0, len(gbk)+10)
	out = append(out, 0x1C, 0x26)
	out = append(out, 0x1C, 0x21, fsBangN)
	out = append(out, gbk...)
	out = append(out, 0x1C, 0x21, 0x00)
	return append(out, 0x1C, 0x2E)
}
