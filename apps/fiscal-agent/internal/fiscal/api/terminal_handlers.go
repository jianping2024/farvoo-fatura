package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func handleTerminalPair(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if IsLoopbackClient(r) {
		writeJSON(w, http.StatusOK, map[string]any{"loopback": true})
		return
	}
	var body struct {
		PairingCode string `json:"pairing_code"`
		Label       string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	res, err := deps.Fiscal.PairFiscalTerminal(deps.StoreID, body.PairingCode, body.Label)
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) {
			writeErr(w, http.StatusForbidden, ce.Code, ce.Msg)
			return
		}
		writeErr(w, http.StatusBadRequest, "pair_failed", err.Error())
		return
	}
	SetTerminalCookie(w, res.TerminalID)
	writeJSON(w, http.StatusOK, map[string]any{
		"terminal_id": res.TerminalID,
		"label":       res.Label,
	})
}

func handleTerminalSummary(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	used, max, err := deps.Fiscal.TerminalSummary(deps.StoreID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "summary_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"terminals_used":       used,
		"max_fiscal_terminals": max,
		"loopback_exempt":      true,
	})
}

func handleListTerminals(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	rows, err := deps.Fiscal.ListFiscalTerminals(deps.StoreID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if rows == nil {
		rows = []store.FiscalTerminalRow{}
	}
	localStation, _ := deps.Fiscal.GetLocalDefaultStation(deps.StoreID)
	type item struct {
		ID                string `json:"id"`
		Label             string `json:"label"`
		Active            bool   `json:"active"`
		RegisteredAt      string `json:"registered_at"`
		LastSeenAt        string `json:"last_seen_at,omitempty"`
		LastSeenIP        string `json:"last_seen_ip,omitempty"`
		DefaultStationID  string `json:"default_station_id,omitempty"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		out = append(out, item{
			ID: row.ID, Label: row.Label, Active: row.Active,
			RegisteredAt: row.RegisteredAt, LastSeenAt: row.LastSeenAt, LastSeenIP: row.LastSeenIP,
			DefaultStationID: row.DefaultStationID,
		})
	}
	used, max, _ := deps.Fiscal.TerminalSummary(deps.StoreID)
	writeJSON(w, http.StatusOK, map[string]any{
		"terminals":                 out,
		"terminals_used":            used,
		"max_fiscal_terminals":      max,
		"local_default_station_id":  localStation,
	})
}

func handleAllowNextTerminal(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := deps.Fiscal.AllowNextTerminal(deps.StoreID, actor, body.Label)
	if err != nil {
		writeTerminalCoded(w, err, "allow_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pairing_code": res.Code,
		"expires_at":   res.ExpiresAt,
		"label":        res.Label,
	})
}

func handleRevokeTerminal(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("terminalId")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "terminal id required")
		return
	}
	if err := deps.Fiscal.RevokeFiscalTerminal(deps.StoreID, id, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "terminal not found or already revoked")
			return
		}
		writeErr(w, http.StatusInternalServerError, "revoke_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleActivateTerminal(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("terminalId")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "terminal id required")
		return
	}
	if err := deps.Fiscal.ActivateFiscalTerminal(deps.StoreID, id, actor); err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) {
			writeErr(w, http.StatusForbidden, ce.Code, ce.Msg)
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "terminal not found or already active")
			return
		}
		writeErr(w, http.StatusInternalServerError, "activate_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleDeleteTerminal(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("terminalId")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "terminal id required")
		return
	}
	if err := deps.Fiscal.DeleteInactiveFiscalTerminal(deps.StoreID, id, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "terminal not found or still active")
			return
		}
		writeErr(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleSetTerminalStation(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("terminalId")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "terminal id required")
		return
	}
	var body struct {
		StationID string `json:"station_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	if err := deps.Fiscal.SetFiscalTerminalDefaultStation(deps.StoreID, id, body.StationID, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "terminal not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "station_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"terminal_id": id,
		"station_id":  strings.TrimSpace(body.StationID),
	})
}

func handleSetTerminalLabel(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("terminalId")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id_required", "terminal id required")
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	if err := deps.Fiscal.SetFiscalTerminalLabel(deps.StoreID, id, body.Label, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not_found", "terminal not found")
			return
		}
		writeTerminalCoded(w, err, "label_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"terminal_id": id,
		"label":       strings.TrimSpace(body.Label),
	})
}

// writeTerminalCoded maps service.CodedError to HTTP; validation → 400, else 403.
func writeTerminalCoded(w http.ResponseWriter, err error, fallbackCode string) {
	var ce *service.CodedError
	if errors.As(err, &ce) {
		status := http.StatusForbidden
		if ce.Code == service.ErrCodeValidationFailed {
			status = http.StatusBadRequest
		}
		writeErr(w, status, ce.Code, ce.Msg)
		return
	}
	writeErr(w, http.StatusInternalServerError, fallbackCode, err.Error())
}

func handleSetLocalPrintStation(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	var body struct {
		StationID string `json:"station_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	if err := deps.Fiscal.SetLocalDefaultStation(deps.StoreID, body.StationID, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusBadRequest, "taxpayer_missing", "configure taxpayer first")
			return
		}
		writeErr(w, http.StatusInternalServerError, "station_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"station_id": strings.TrimSpace(body.StationID),
	})
}

// ResolveEffectivePrintStation is the ONLY effective print station resolver for invoicing.
// Loopback → local_default_station_id; LAN → active terminal cookie default_station_id.
func ResolveEffectivePrintStation(r *http.Request, deps HandlerDeps) (string, error) {
	if deps.Fiscal == nil {
		return "", errors.New("fiscal service not configured")
	}
	if IsLoopbackClient(r) {
		return deps.Fiscal.GetLocalDefaultStation(deps.StoreID)
	}
	tid := TerminalIDFromRequest(r)
	if tid == "" {
		return "", nil
	}
	row, err := deps.Fiscal.GetFiscalTerminal(deps.StoreID, tid)
	if err != nil {
		return "", err
	}
	return row.DefaultStationID, nil
}

func handleGetEffectivePrintStation(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	sid, err := ResolveEffectivePrintStation(r, deps)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "terminal_revoked", "terminal unknown or revoked")
			return
		}
		writeErr(w, http.StatusInternalServerError, "station_failed", err.Error())
		return
	}
	label := stationLabelForID(deps, sid)
	writeJSON(w, http.StatusOK, map[string]any{
		"station_id":    sid,
		"station_label": label,
		"loopback":      IsLoopbackClient(r),
	})
}

func stationLabelForID(deps HandlerDeps, sid string) string {
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return ""
	}
	mapped := map[string]string{}
	if deps.StationPrintersFn != nil {
		mapped = deps.StationPrintersFn()
	}
	var meta []StationMeta
	if deps.StationMetaFn != nil {
		meta = deps.StationMetaFn()
	}
	for _, st := range BuildPrinterStationList(mapped, meta) {
		if st.ID == sid {
			return st.Label
		}
	}
	return stationIDFallback(sid)
}

func handleSetTerminalMax(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	actor, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	var body struct {
		Max int `json:"max_fiscal_terminals"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	if err := deps.Fiscal.SetMaxFiscalTerminals(deps.StoreID, body.Max, actor); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusBadRequest, "taxpayer_missing", "configure taxpayer first")
			return
		}
		writeErr(w, http.StatusBadRequest, "max_invalid", err.Error())
		return
	}
	used, max, _ := deps.Fiscal.TerminalSummary(deps.StoreID)
	writeJSON(w, http.StatusOK, map[string]any{
		"max_fiscal_terminals": max,
		"terminals_used":       used,
	})
}
