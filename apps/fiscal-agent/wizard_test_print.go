package main

import (
	"encoding/json"
	"strings"
)

type testPrintRequest struct {
	StationID string `json:"station_id"`
	Printer   string `json:"printer,omitempty"`
	// Locale: slip language zh|en|pt — independent of tray ui_locale.
	Locale string `json:"locale,omitempty"`
}

func runTestPrintForStation(cfg *config, stationID, printerOverride, printLocale string) error {
	if cfg == nil {
		return uiError("zh", "err_not_loaded")
	}
	uiLoc := cfg.uiLocale()
	slipLoc := normalizePrintLocale(printLocale)
	stationID = strings.TrimSpace(stationID)
	printerOverride = strings.TrimSpace(printerOverride)

	var rawAddr string
	if printerOverride != "" {
		rawAddr = printerOverride
	} else if stationID != "" && cfg.StationPrinters != nil {
		rawAddr = strings.TrimSpace(cfg.StationPrinters[stationID])
	}
	if rawAddr == "" && stationID == "" {
		stationID, rawAddr = cfg.firstMappedStation()
	}
	if rawAddr == "" {
		return uiError(uiLoc, "err_save_mapping_first")
	}

	target, err := parsePrinterTarget(rawAddr)
	if err != nil {
		return err
	}

	venue := "Mesa"
	if u := strings.TrimSpace(cfg.APIBase); u != "" {
		venue = u
		if i := strings.Index(u, "://"); i >= 0 {
			venue = strings.TrimPrefix(u[i+3:], "www.")
			if j := strings.IndexAny(venue, "/:"); j > 0 {
				venue = venue[:j]
			}
		}
	}

	payload := jobPayload{
		ConnectionTest: true,
		Locale:         slipLoc,
		RestaurantName: venue,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	job := printJob{Type: "order_receipt", Payload: raw}
	data := escposFromJob(job)
	if err := printToTarget(target, data); err != nil {
		return uiError(uiLoc, "err_print_failed", target.Display, err)
	}
	return nil
}
