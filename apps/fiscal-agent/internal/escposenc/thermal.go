package escposenc

import (
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

// NormalizeThermalEncoding is the ONLY normalizer for config text_encoding.
// Values: auto | han_firmware | utf8 | latin. auto = every NeedsRaster line as GS v 0 bitmap
// (printer-agnostic). Legacy bitmap / gbk → auto.
func NormalizeThermalEncoding(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "utf8", "utf-8":
		return "utf8"
	case "latin", "windows1252", "cp1252":
		return "latin"
	case "han_firmware", "firmware", "han":
		return "han_firmware"
	default:
		return "auto"
	}
}

// NeedsRaster: true when s has a rune Windows-1252 cannot carry (Han, …). pt-PT accents fit
// WPC1252 and stay printer Font A. text_encoding=auto rasters exactly these lines; han_firmware
// splits them further with LinePathFor (firmware Han vs raster).
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
