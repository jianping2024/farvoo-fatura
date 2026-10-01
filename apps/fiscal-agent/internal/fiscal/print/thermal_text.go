package print

import (
	"bytes"

	"farvoo-fiscal-agent/internal/escposbitmap"
	"farvoo-fiscal-agent/internal/escposenc"
)

// thermalEncoding follows Agent config text_encoding (auto|utf8|latin).
// ONLY SetThermalEncoding mutates this.
var thermalEncoding = "auto"

// SetThermalEncoding is the ONLY writer for FT thermal text_encoding policy.
func SetThermalEncoding(raw string) {
	thermalEncoding = escposenc.NormalizeThermalEncoding(raw)
}

type thermalLineStyle struct {
	Bold bool
}

// emitThermalLine is the ONLY FT receipt text emitter (docs/fiscal-thermal-text-encoding.zh.md).
// Bitmap lines must not append LF (GS v 0 already advances paper).
func emitThermalLine(b *bytes.Buffer, s string, sty thermalLineStyle) {
	switch thermalEncoding {
	case "utf8":
		b.WriteString(s)
		b.WriteByte('\n')
	case "latin":
		b.Write(escposenc.Windows1252(s))
		b.WriteByte('\n')
	default: // auto
		if escposenc.HasNonASCII(s) {
			b.Write(escposbitmap.Line(s, escposbitmap.Style{Bold: sty.Bold}, escposbitmap.DefaultFontPx))
			return
		}
		b.Write(escposenc.Windows1252(s))
		b.WriteByte('\n')
	}
}
