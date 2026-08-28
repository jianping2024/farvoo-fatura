package main

import (
	"farvoo-fiscal-agent/internal/fiscal/api"
)

func printStationRowsToMeta(rows []printStationRow) []api.StationMeta {
	if len(rows) == 0 {
		return nil
	}
	out := make([]api.StationMeta, 0, len(rows))
	for _, r := range rows {
		id := r.ID
		if id == "" {
			continue
		}
		out = append(out, api.StationMeta{
			ID:        id,
			NamePt:    r.NamePt,
			NameEn:    r.NameEn,
			NameZh:    r.NameZh,
			SortOrder: r.SortOrder,
		})
	}
	return out
}

func stationMetaFromConfig(cfg *config) []api.StationMeta {
	if cfg == nil {
		return nil
	}
	if cfg.APIBase == "" || cfg.AgentJWT == "" {
		return nil
	}
	rows, err := fetchPrintStations(cfg.APIBase, cfg.AgentJWT)
	if err != nil {
		return nil
	}
	return printStationRowsToMeta(rows)
}

func stationMetaFnForConfigPath(configPath string) func() []api.StationMeta {
	return func() []api.StationMeta {
		c, err := loadConfig(configPath)
		if err != nil {
			return nil
		}
		return stationMetaFromConfig(c)
	}
}
