package api

import (
	"errors"
	"net/http"
	"strings"
)

// ErrStoreIDMismatch is returned when a request store_id disagrees with the agent store.
var ErrStoreIDMismatch = errors.New("api: store_id mismatch")

// resolveRequestStoreID is the ONLY HTTP request store_id resolver.
// Empty → deps.StoreID; non-empty must equal deps.StoreID.
func resolveRequestStoreID(deps HandlerDeps, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return deps.StoreID, nil
	}
	if requested != deps.StoreID {
		return "", ErrStoreIDMismatch
	}
	return deps.StoreID, nil
}

// writeResolvedStoreID resolves store_id or writes 403 — ONLY handler-facing store_id gate.
func writeResolvedStoreID(w http.ResponseWriter, deps HandlerDeps, requested string) (string, bool) {
	id, err := resolveRequestStoreID(deps, requested)
	if err != nil {
		writeErr(w, http.StatusForbidden, "store_id_mismatch", "store_id must match this agent")
		return "", false
	}
	return id, true
}
