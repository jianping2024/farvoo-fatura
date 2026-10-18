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
	// Kind: "" = connection test slip; testPrintKindCodePage = code-page self-test slip;
	// testPrintKindStationSample = multilang station slip sample.
	Kind string `json:"kind,omitempty"`
}

const (
	testPrintKindCodePage      = "code_page"
	testPrintKindStationSample = "station_sample"
)

// buildStationSampleSlip — a real station_ticket (escposFromJob) with pt + zh print_names, long
// names that wrap and a pt-prefix + Han note: the paper sample of
// restaurant-ordering docs/product/station-slip-multilang-names.zh.md §6.
func buildStationSampleSlip(locale string) []byte {
	payload, _ := json.Marshal(jobPayload{
		Locale:             locale,
		TableDisplayName:   "001",
		GuestCount:         2,
		StationSlipOptions: &stationSlipOptions{ShowCategoryGroup: true},
		Lines: []jobLine{
			{ItemIndex: 0, ItemCode: "012", Qty: 2, Note: "sem cebola, 少放盐", CategoryGroupSort: 0, CategoryGroupHeader: "(PRATOS)",
				PrintNames: []string{"Bacalhau à Brás com Batata Palha e Azeitonas", "葡式炒鳕鱼丝配金黄薯丝和黑橄榄及香草橄榄油"}},
			{ItemIndex: 1, ItemCode: "020", Qty: 1, CategoryGroupSort: 0, CategoryGroupHeader: "(PRATOS)",
				PrintNames: []string{"Frango Piri-Piri", "霹雳烤鸡"}},
			{ItemIndex: 2, ItemCode: "001", Qty: 1, CategoryGroupSort: 1, CategoryGroupHeader: "(BEBIDAS)",
				PrintNames: []string{"Água 500ml", "矿泉水 500ml"}},
			{ItemIndex: 3, ItemCode: "006", Qty: 1, CategoryGroupSort: 1, CategoryGroupHeader: "(BEBIDAS)",
				PrintNames: []string{"Cola"}},
		},
	})
	return escposFromJob(printJob{Type: "station_ticket", Payload: payload})
}

func runTestPrintForStation(cfg *config, stationID, printerOverride, printLocale, kind string) error {
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

	var data []byte
	switch strings.TrimSpace(kind) {
	case testPrintKindCodePage:
		data = buildCodePageProbe(venue)
	case testPrintKindStationSample:
		data = buildStationSampleSlip(slipLoc)
	default:
		payload := jobPayload{
			ConnectionTest: true,
			Locale:         slipLoc,
			RestaurantName: venue,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		data = escposFromJob(printJob{Type: "order_receipt", Payload: raw})
	}
	if err := printToTarget(target, data); err != nil {
		return uiError(uiLoc, "err_print_failed", target.Display, err)
	}
	return nil
}
