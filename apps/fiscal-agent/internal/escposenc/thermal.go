package escposenc

import (
	"strings"
	"unicode"
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

// HasNonASCII reports whether s contains any rune outside ASCII.
func HasNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

// LineUsesRaster is the ONLY per-line raster decision for text_encoding=auto.
func LineUsesRaster(encoding, s string) bool {
	if NormalizeThermalEncoding(encoding) != "auto" {
		return false
	}
	return HasNonASCII(s)
}
