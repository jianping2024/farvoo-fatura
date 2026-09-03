package api

import (
	"errors"
	"net/http"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

// requireActiveLANTerminal is the ONLY LAN terminal gate for authenticated routes.
// Loopback clients are exempt (do not occupy Ops terminal slots).
// Non-loopback must present fiscal_terminal_id for an active row; Touch updates last_seen_at.
func (deps HandlerDeps) requireActiveLANTerminal(w http.ResponseWriter, r *http.Request) bool {
	if IsLoopbackClient(r) {
		return true
	}
	tid := TerminalIDFromRequest(r)
	if tid == "" {
		writeErr(w, http.StatusForbidden, "terminal_required", "pair this PC before using fiscal on LAN")
		return false
	}
	if deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return false
	}
	if err := deps.Fiscal.DB().TouchFiscalTerminal(deps.StoreID, tid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusForbidden, "terminal_revoked", "terminal unknown or revoked; re-pair with Ops code")
			return false
		}
		writeErr(w, http.StatusInternalServerError, "terminal_check_failed", err.Error())
		return false
	}
	return true
}
