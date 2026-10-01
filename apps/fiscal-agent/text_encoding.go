package main

import (
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/escposenc"
)

func normalizeTextEncoding(raw string) string {
	return escposenc.NormalizeThermalEncoding(raw)
}

// applyThermalEncodingFromConfig is the ONLY Agent→FT thermal encoding applicator
// (also called from applyFiscalRuntimeFromConfig / wizard save).
func applyThermalEncodingFromConfig(cfg *config) {
	enc := "auto"
	if cfg != nil {
		enc = normalizeTextEncoding(cfg.TextEncoding)
	}
	fiscalprint.SetThermalEncoding(enc)
}
