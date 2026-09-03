package api

import (
	"encoding/json"
	"errors"
	"net/http"

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
	type item struct {
		ID           string `json:"id"`
		Label        string `json:"label"`
		Active       bool   `json:"active"`
		RegisteredAt string `json:"registered_at"`
		LastSeenAt   string `json:"last_seen_at,omitempty"`
		LastSeenIP   string `json:"last_seen_ip,omitempty"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		out = append(out, item{
			ID: row.ID, Label: row.Label, Active: row.Active,
			RegisteredAt: row.RegisteredAt, LastSeenAt: row.LastSeenAt, LastSeenIP: row.LastSeenIP,
		})
	}
	used, max, _ := deps.Fiscal.TerminalSummary(deps.StoreID)
	writeJSON(w, http.StatusOK, map[string]any{
		"terminals":            out,
		"terminals_used":       used,
		"max_fiscal_terminals": max,
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
		var ce *service.CodedError
		if errors.As(err, &ce) {
			writeErr(w, http.StatusForbidden, ce.Code, ce.Msg)
			return
		}
		writeErr(w, http.StatusInternalServerError, "allow_failed", err.Error())
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
