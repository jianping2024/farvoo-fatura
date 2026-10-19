package print

import (
	"bytes"
	"log"

	"farvoo-fiscal-agent/internal/escposbitmap"
	"farvoo-fiscal-agent/internal/escposenc"
)

// thermalEncoding follows Agent config text_encoding (auto|han_firmware|utf8|latin).
// ONLY SetThermalEncoding mutates this.
var thermalEncoding = "auto"

// SetThermalEncoding is the ONLY writer for FT thermal text_encoding policy.
func SetThermalEncoding(raw string) {
	thermalEncoding = escposenc.NormalizeThermalEncoding(raw)
}

type thermalLineStyle struct {
	Bold bool
	// DoubleH mirrors the caller's GS ! 0x01 so Han lines (bitmap or firmware) get the same 1×2.
	DoubleH bool
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
	case "han_firmware": // opt-in: printer Han font when the code page probe passes B
		switch escposenc.LinePathFor(s) {
		case escposenc.PathHanFirmware:
			b.Write(escposenc.HanFirmwareLine(s, escposenc.FontASize(false, sty.DoubleH)))
			b.WriteByte('\n')
		case escposenc.PathRaster:
			emitThermalRaster(b, s, sty)
		default:
			b.Write(escposenc.Windows1252(s))
			b.WriteByte('\n')
		}
	default: // auto — every line beyond WPC1252 → GS v 0 (printer-agnostic)
		if escposenc.NeedsRaster(s) {
			emitThermalRaster(b, s, sty)
			return
		}
		b.Write(escposenc.Windows1252(s))
		b.WriteByte('\n')
	}
}

// emitThermalRaster — escposbitmap.Line at the caller's size; on render failure prints the
// line in Font A with '?' placeholders (never dropped).
func emitThermalRaster(b *bytes.Buffer, s string, sty thermalLineStyle) {
	raster := escposbitmap.Line(s, escposbitmap.Style{Bold: sty.Bold, DoubleH: sty.DoubleH}, escposbitmap.DefaultFontPx)
	if len(raster) > 0 {
		b.Write(raster)
		return
	}
	log.Printf("thermal bitmap render failed; printing Font A fallback: %q", s)
	b.Write(escposenc.Windows1252Placeholder(s))
	b.WriteByte('\n')
}
