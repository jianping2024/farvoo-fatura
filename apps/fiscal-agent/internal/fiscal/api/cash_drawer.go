package api

import (
	"encoding/json"
	"net/http"
	"strings"

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

// CashDrawerSnapshot is the ONLY JSON shape for GET/PUT /local/v1/setup/cash-drawer.
type CashDrawerSnapshot struct {
	Pin int `json:"pin"` // 2 or 5
}

// handleGetCashDrawer is the ONLY GET handler for cash drawer pin.
func handleGetCashDrawer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.CashDrawerPinGet == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_configured", "cash drawer pin not configured")
		return
	}
	writeJSON(w, http.StatusOK, CashDrawerSnapshot{Pin: fiscalprint.NormalizeCashDrawerPin(deps.CashDrawerPinGet())})
}

// handlePutCashDrawer is the ONLY PUT handler for cash drawer pin (admin/owner).
func handlePutCashDrawer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.CashDrawerPinGet == nil || deps.CashDrawerPinSet == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_configured", "cash drawer pin not configured")
		return
	}
	var body struct {
		Pin int `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON")
		return
	}
	pin := fiscalprint.NormalizeCashDrawerPin(body.Pin)
	if body.Pin != 2 && body.Pin != 5 {
		writeErr(w, http.StatusBadRequest, "invalid_pin", "pin must be 2 or 5")
		return
	}
	if err := deps.CashDrawerPinSet(pin); err != nil {
		writeErr(w, http.StatusInternalServerError, "cash_drawer_save_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, CashDrawerSnapshot{Pin: pin})
}

// handleOpenCashDrawer is the ONLY POST handler for manual drawer kick (session).
func handleOpenCashDrawer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.PrintBytesFn == nil {
		writeErr(w, http.StatusServiceUnavailable, "print_not_configured", "printer output not configured")
		return
	}
	if deps.CashDrawerPinGet == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_configured", "cash drawer pin not configured")
		return
	}
	sid, err := ResolveEffectivePrintStation(r, deps)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "station_failed", err.Error())
		return
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		writeErr(w, http.StatusBadRequest, "station_required", "set a default print station first")
		return
	}
	raw := ""
	if deps.StationPrintersFn != nil {
		if m := deps.StationPrintersFn(); m != nil {
			raw = strings.TrimSpace(m[sid])
		}
	}
	if raw == "" {
		writeErr(w, http.StatusBadRequest, "station_unmapped", "station not mapped to a printer")
		return
	}
	pin := fiscalprint.NormalizeCashDrawerPin(deps.CashDrawerPinGet())
	data := fiscalprint.CashDrawerKickBytes(pin)
	if err := deps.PrintBytesFn(raw, data); err != nil {
		writeErr(w, http.StatusBadGateway, "drawer_open_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"pin":        pin,
		"station_id": sid,
	})
}
