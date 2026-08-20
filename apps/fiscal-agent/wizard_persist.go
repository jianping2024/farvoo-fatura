package main

import (
	"errors"
	"net/http"
	"strings"
)

// persistStationPrinterSetup validates mapping locally, syncs to cloud first, then saves config.
func persistStationPrinterSetup(
	configPath string,
	cfg **config,
	c *config,
	body setupRequestBody,
) (int, map[string]any) {
	cleaned, err := applyPrinterSetup(c, body)
	if err != nil {
		return http.StatusBadRequest, map[string]any{"error": err.Error()}
	}

	if syncErr := syncRoutingToCloud(c, body.ForceTakeover); syncErr != nil {
		if rse, ok := isRoutingSyncConflict(syncErr); ok {
			return http.StatusConflict, map[string]any{
				"error":      rse.Message,
				"error_code": rse.Code,
				"conflicts":  rse.Conflicts,
			}
		}
		var rse *RoutingSyncError
		if errors.As(syncErr, &rse) && rse.Code == "unauthorized" {
			return http.StatusUnauthorized, map[string]any{
				"error":      rse.Message,
				"error_code": "agent_revoked",
			}
		}
		msg := strings.TrimSpace(syncErr.Error())
		if msg == "" {
			msg = "routing sync failed"
		}
		return http.StatusBadGateway, map[string]any{
			"error":      msg,
			"error_code": "routing_sync_failed",
		}
	}

	if err := saveConfig(configPath, c); err != nil {
		return http.StatusInternalServerError, map[string]any{"error": err.Error()}
	}
	*cfg = c
	cleared := len(cleaned) == 0
	if cleared {
		agentLog(c, "log_station_maps_cleared")
	} else {
		agentLog(c, "log_station_maps_saved", len(cleaned))
	}
	return http.StatusOK, map[string]any{
		"status":           "ok",
		"station_printers": c.StationPrinters,
		"routing_sync":     "ok",
		"mapping_cleared":  cleared,
	}
}
