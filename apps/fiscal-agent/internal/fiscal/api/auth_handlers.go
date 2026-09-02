package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func handleListOperators(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	db := deps.Fiscal.DB()
	if db == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "db missing")
		return
	}
	rows, err := db.ListOperatorsForLogin(deps.StoreID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if rows == nil {
		rows = []store.OperatorLoginRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"operators": rows})
}

func handleBootstrapOwner(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if !IsLoopbackClient(r) {
		writeErr(w, http.StatusForbidden, "bootstrap_loopback_only", "create first admin on the agent host only")
		return
	}
	var body struct {
		DisplayName string `json:"display_name"`
		PIN         string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	id, err := deps.Fiscal.BootstrapOwner(deps.StoreID, strings.TrimSpace(body.DisplayName), body.PIN)
	if err != nil {
		if errors.Is(err, store.ErrBootstrapNotEmpty) {
			writeErr(w, http.StatusForbidden, "bootstrap_not_empty", "operators already exist")
			return
		}
		if errors.Is(err, store.ErrInvalidPIN) {
			writeErr(w, http.StatusBadRequest, "invalid_pin", "pin must be 6 digits")
			return
		}
		writeErr(w, http.StatusInternalServerError, "bootstrap_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operator_id": id})
}

func handleLogin(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		OperatorID string `json:"operator_id"`
		PIN        string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	opID := strings.TrimSpace(body.OperatorID)
	if opID == "" {
		writeErr(w, http.StatusBadRequest, "operator_id_required", "operator_id required")
		return
	}
	sess, err := deps.Fiscal.LoginOperator(r.Context(), deps.StoreID, opID, body.PIN, store.ClientIPFromRemoteAddr(r.RemoteAddr))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrIPRateLimited):
			writeErr(w, http.StatusTooManyRequests, "ip_rate_limited", "too many failed attempts")
		case errors.Is(err, store.ErrOperatorLocked):
			writeErr(w, http.StatusTooManyRequests, "operator_locked", "too many failed attempts")
		case errors.Is(err, store.ErrPINMismatch), errors.Is(err, store.ErrInvalidPIN):
			writeErr(w, http.StatusUnauthorized, "login_failed", "invalid pin")
		case errors.Is(err, store.ErrNotFound):
			writeErr(w, http.StatusUnauthorized, "login_failed", "operator not found")
		default:
			writeErr(w, http.StatusInternalServerError, "login_failed", err.Error())
		}
		return
	}
	sm := deps.Sessions
	if sm == nil {
		sm = MustNewSessionManager(deps.DataDir)
	}
	_ = sm.SetSessionCookie(w, Session{
		OperatorID:  sess.OperatorID,
		Role:        sess.Role,
		DisplayName: sess.DisplayName,
		Epoch:       sess.SessionEpoch,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"operator_id":  sess.OperatorID,
		"display_name": sess.DisplayName,
		"role":         sess.Role,
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if s := SessionFromContext(r.Context()); s != nil {
		_ = deps.Fiscal.DB().InsertAuditLog(s.OperatorID, "LOGOUT", "operator", s.OperatorID, "{}")
	}
	sm := deps.Sessions
	if sm == nil {
		sm = MustNewSessionManager(deps.DataDir)
	}
	sm.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleChangePIN(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	s := SessionFromContext(r.Context())
	if s == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
		return
	}
	var body struct {
		OldPIN string `json:"old_pin"`
		NewPIN string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json", "invalid body")
		return
	}
	if err := deps.Fiscal.ChangeOperatorPIN(deps.StoreID, s.OperatorID, body.OldPIN, body.NewPIN); err != nil {
		if errors.Is(err, store.ErrPINMismatch) {
			writeErr(w, http.StatusUnauthorized, "pin_mismatch", "old pin incorrect")
			return
		}
		if errors.Is(err, store.ErrInvalidPIN) {
			writeErr(w, http.StatusBadRequest, "invalid_pin", "pin must be 6 digits")
			return
		}
		writeErr(w, http.StatusInternalServerError, "change_pin_failed", err.Error())
		return
	}
	_ = deps.Fiscal.DB().InsertAuditLog(s.OperatorID, "PIN_CHANGE", "operator", s.OperatorID, "{}")
	setSessionCookieFromState(w, deps, s.OperatorID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

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
	res, err := deps.Fiscal.PairFiscalTerminal(r.Context(), deps.StoreID, body.PairingCode, body.Label)
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) {
			writeErr(w, http.StatusForbidden, ce.Code, ce.Msg)
			return
		}
		writeErr(w, http.StatusBadGateway, "pair_failed", err.Error())
		return
	}
	SetTerminalCookie(w, res.TerminalID)
	writeJSON(w, http.StatusOK, map[string]any{
		"terminal_id": res.TerminalID,
		"label":         res.Label,
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

// OperatorIDFromRequest returns session operator id or empty.
func OperatorIDFromRequest(r *http.Request) string {
	if s := SessionFromContext(r.Context()); s != nil {
		return s.OperatorID
	}
	return ""
}

// RequireOperatorID returns session operator or writes 401.
func RequireOperatorID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := OperatorIDFromRequest(r)
	if id == "" {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
		return "", false
	}
	return id, true
}
