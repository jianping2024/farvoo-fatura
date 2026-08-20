package main

import (
	"encoding/json"
	"strings"
)

// jobRouteStationID is the mapped station id for logs (station_ticket / receipt station:…).
func jobRouteStationID(job printJob) string {
	if job.Type == "station_ticket" {
		var p struct {
			PrintStationID string `json:"print_station_id"`
		}
		_ = json.Unmarshal(job.Payload, &p)
		if sid := strings.TrimSpace(p.PrintStationID); sid != "" {
			return sid
		}
	}
	if isReceiptPrintJobType(job.Type) {
		var p struct {
			ReceiptPrinterID string `json:"receipt_printer_id"`
		}
		_ = json.Unmarshal(job.Payload, &p)
		rid := strings.TrimSpace(p.ReceiptPrinterID)
		const prefix = "station:"
		if strings.HasPrefix(rid, prefix) {
			if sid := strings.TrimSpace(rid[len(prefix):]); sid != "" {
				return sid
			}
		}
		return "receipt"
	}
	return strings.TrimSpace(job.Type)
}
