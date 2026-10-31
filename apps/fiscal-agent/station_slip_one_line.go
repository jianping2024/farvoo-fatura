package main

// Station slip one-line flags (agent-local): when on, each print_names entry / note keeps
// only the first wrap chunk; excess discarded with no ellipsis.

func stationNameOneLineFromConfig() bool {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil {
		return false
	}
	return cfg.StationNameOneLine
}

func stationNoteOneLineFromConfig() bool {
	cfg, err := loadConfig(defaultConfigPath())
	if err != nil || cfg == nil {
		return false
	}
	return cfg.StationNoteOneLine
}

// stationSlipKeepLines — ONLY gate for station name/note line count under one-line flags.
// oneLine false → unchanged; true → at most the first chunk (hard discard, no "…").
func stationSlipKeepLines(chunks []string, oneLine bool) []string {
	if !oneLine || len(chunks) <= 1 {
		return chunks
	}
	return chunks[:1]
}
