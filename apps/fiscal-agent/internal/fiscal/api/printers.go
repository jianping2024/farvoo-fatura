package api

import (
	"sort"
	"strings"
)

// StationMeta is cloud print_stations metadata merged into GET /local/v1/printers.
type StationMeta struct {
	ID        string
	NameZh    string
	NameEn    string
	NamePt    string
	SortOrder int
}

// PrinterStation is one mapped station for GET /local/v1/printers.
type PrinterStation struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Printer   string `json:"printer"`
	SortOrder int    `json:"sort_order"`
}

// StationDisplayLabel picks the human label for a print station (zh admin UI).
func StationDisplayLabel(meta StationMeta, id string) string {
	if s := strings.TrimSpace(meta.NameZh); s != "" {
		return s
	}
	if s := strings.TrimSpace(meta.NameEn); s != "" {
		return s
	}
	if s := strings.TrimSpace(meta.NamePt); s != "" {
		return s
	}
	return stationIDFallback(id)
}

func stationIDFallback(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if len(id) <= 8 {
		return id
	}
	return id[:8] + "…"
}

// BuildPrinterStationList merges live station_printers with optional cloud metadata.
// Sort: cloud sort_order, then id. Orphan mappings (no meta) sort after known stations.
func BuildPrinterStationList(mapped map[string]string, meta []StationMeta) []PrinterStation {
	if len(mapped) == 0 {
		return nil
	}
	metaByID := make(map[string]StationMeta, len(meta))
	for _, m := range meta {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		metaByID[id] = m
	}

	out := make([]PrinterStation, 0, len(mapped))
	listed := make(map[string]bool, len(mapped))

	sortedMeta := append([]StationMeta(nil), meta...)
	sort.Slice(sortedMeta, func(i, j int) bool {
		if sortedMeta[i].SortOrder != sortedMeta[j].SortOrder {
			return sortedMeta[i].SortOrder < sortedMeta[j].SortOrder
		}
		return sortedMeta[i].ID < sortedMeta[j].ID
	})

	for _, m := range sortedMeta {
		id := strings.TrimSpace(m.ID)
		addr := strings.TrimSpace(mapped[id])
		if id == "" || addr == "" {
			continue
		}
		listed[id] = true
		out = append(out, PrinterStation{
			ID:        id,
			Label:     StationDisplayLabel(m, id),
			Printer:   addr,
			SortOrder: m.SortOrder,
		})
	}

	orphanIDs := make([]string, 0)
	for id, addr := range mapped {
		id = strings.TrimSpace(id)
		addr = strings.TrimSpace(addr)
		if id == "" || addr == "" || listed[id] {
			continue
		}
		orphanIDs = append(orphanIDs, id)
	}
	sort.Strings(orphanIDs)
	for _, id := range orphanIDs {
		out = append(out, PrinterStation{
			ID:        id,
			Label:     stationIDFallback(id),
			Printer:   strings.TrimSpace(mapped[id]),
			SortOrder: 9999,
		})
	}
	return out
}
