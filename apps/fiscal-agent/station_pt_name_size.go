package main

import (
	"strings"
)

// Station slip first dish-name (Portuguese / print_names[0]) Font A size.
// Agent-local only; default 1×2 matches existing paper. Optional 2×2 for kitchen glanceability.
const (
	stationPtNameSize1x2 = "1x2"
	stationPtNameSize2x2 = "2x2"
)

func normalizeStationPtNameSize(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case stationPtNameSize2x2, "2×2":
		return stationPtNameSize2x2
	default:
		return stationPtNameSize1x2
	}
}

// stationPtNameSizeFromConfig reads the agent config (same path as thermal encoding).
func stationPtNameSizeFromConfig() string {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil {
		return stationPtNameSize1x2
	}
	return normalizeStationPtNameSize(cfg.StationPtNameSize)
}

func stationPtNameSizeIs2x2(size string) bool {
	return normalizeStationPtNameSize(size) == stationPtNameSize2x2
}

// stationSlipFirstNameMaxCols — wrap width for print_names[0] in Font A column units
// at the active first-name size (1×2 → full remaining width; 2×2 → half-width cells).
func stationSlipFirstNameMaxCols(lead string, firstName2x2 bool) int {
	leadW := displayWidth(lead)
	if firstName2x2 {
		// Both lead and name at 2×2: one gap space, then name cells.
		return escposWidth/2 - leadW - 1
	}
	return escposWidth - stationSlipNameIndent(lead)
}

// stationSlipNameHangIndent — paper columns (1×2 spaces) where secondary names / notes start.
// Matches the visual start of the first name after the 2×2 lead (+ gap).
func stationSlipNameHangIndent(lead string, firstName2x2 bool) int {
	if firstName2x2 {
		// Gap space also printed at 2×2 → 2 paper columns.
		return displayWidth(lead)*2 + 2
	}
	return stationSlipNameIndent(lead)
}

// stationSlipFirstNameHangPad — pad string for wrapped first-name continuation rows
// (printed at the same GS ! size as the first name).
func stationSlipFirstNameHangPad(lead string, firstName2x2 bool) string {
	if firstName2x2 {
		return strings.Repeat(" ", displayWidth(lead)+1)
	}
	return strings.Repeat(" ", stationSlipNameIndent(lead))
}
