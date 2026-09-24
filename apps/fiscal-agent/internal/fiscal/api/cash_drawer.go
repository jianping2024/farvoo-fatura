package api

import (
	"encoding/json"
	"fmt"
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

// KickCashDrawerAtStation is the ONLY path that sends ESC/POS kick bytes to a mapped station.
func KickCashDrawerAtStation(deps HandlerDeps, stationID string) (pin int, err error) {
	if deps.PrintBytesFn == nil {
		return 0, fmt.Errorf("print_not_configured")
	}
	if deps.CashDrawerPinGet == nil {
		return 0, fmt.Errorf("not_configured")
	}
	sid := strings.TrimSpace(stationID)
	if sid == "" {
		return 0, fmt.Errorf("station_required")
	}
	raw := ""
	if deps.StationPrintersFn != nil {
		if m := deps.StationPrintersFn(); m != nil {
			raw = strings.TrimSpace(m[sid])
		}
	}
	if raw == "" {
		return 0, fmt.Errorf("station_unmapped")
	}
	pin = fiscalprint.NormalizeCashDrawerPin(deps.CashDrawerPinGet())
	if err := deps.PrintBytesFn(raw, fiscalprint.CashDrawerKickBytes(pin)); err != nil {
		return pin, fmt.Errorf("drawer_open_failed: %w", err)
	}
	return pin, nil
}

// KickCashDrawerOnLocalDefault is the ONLY background hang-queue kick (Farvoo CASH collect).
// Uses taxpayer local_default_station_id — same as loopback Admin open without a browser session.
func KickCashDrawerOnLocalDefault(deps HandlerDeps) (stationID string, pin int, err error) {
	if deps.Fiscal == nil {
		return "", 0, fmt.Errorf("fiscal service not configured")
	}
	sid, err := deps.Fiscal.GetLocalDefaultStation(deps.StoreID)
	if err != nil {
		return "", 0, err
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return "", 0, fmt.Errorf("station_required")
	}
	pin, err = KickCashDrawerAtStation(deps, sid)
	return sid, pin, err
}

// handleOpenCashDrawer is the ONLY POST handler for manual drawer kick (session).
func handleOpenCashDrawer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
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
	pin, err := KickCashDrawerAtStation(deps, sid)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "print_not_configured"):
			writeErr(w, http.StatusServiceUnavailable, "print_not_configured", "printer output not configured")
		case strings.Contains(msg, "not_configured"):
			writeErr(w, http.StatusServiceUnavailable, "not_configured", "cash drawer pin not configured")
		case strings.Contains(msg, "station_unmapped"):
			writeErr(w, http.StatusBadRequest, "station_unmapped", "station not mapped to a printer")
		case strings.Contains(msg, "drawer_open_failed"):
			writeErr(w, http.StatusBadGateway, "drawer_open_failed", msg)
		default:
			writeErr(w, http.StatusBadGateway, "drawer_open_failed", msg)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"pin":        pin,
		"station_id": sid,
	})
}
