package escposenc

import (
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

// NormalizeThermalEncoding is the ONLY normalizer for config text_encoding.
// Values: auto | utf8 | latin. Legacy gbk → auto.
func NormalizeThermalEncoding(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "utf8", "utf-8":
		return "utf8"
	case "latin", "windows1252", "cp1252":
		return "latin"
	default:
		return "auto"
	}
}

// NeedsRaster is the ONLY per-line raster decision for text_encoding=auto:
// true when s has a rune Windows-1252 cannot carry (Han, …). pt-PT accents fit
// WPC1252 and stay printer Font A (same size / columns as ASCII).
func NeedsRaster(s string) bool {
	for _, r := range s {
		if r <= unicode.MaxASCII {
			continue
		}
		if _, ok := charmap.Windows1252.EncodeRune(r); !ok {
			return true
		}
	}
	return false
}
