// Package escposenc encodes Latin text for ESC/POS printers (Windows-1252 + ESC t 16).
// ONLY Windows-1252 encoder for Agent kitchen tickets and fiscal FT receipts.
package escposenc

import (
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// CodeTableWPC1252 is ESC t n for Windows-1252 (Epson-compatible).
const CodeTableWPC1252 byte = 16

// SelectCodeTable returns ESC t n.
func SelectCodeTable(n byte) []byte {
	return []byte{0x1B, 0x74, n}
}

// Windows1252 is the ONLY Latin byte encoder for printable receipt text.
// Unmappable runes are omitted (never replaced with '?').
func Windows1252(s string) []byte {
	enc := charmap.Windows1252.NewEncoder()
	out, err := enc.Bytes([]byte(s))
	if err != nil {
		var b strings.Builder
		for _, r := range s {
			if r < 128 {
				b.WriteRune(r)
				continue
			}
			t := string(r)
			if _, err2 := enc.Bytes([]byte(t)); err2 == nil {
				b.WriteRune(r)
			}
		}
		out, _ = enc.Bytes([]byte(b.String()))
	}
	return out
}
