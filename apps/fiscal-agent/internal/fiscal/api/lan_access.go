package api

import (
	"encoding/json"
	"net/http"
)

// LanAccessSnapshot is the ONLY JSON shape for GET/PUT /local/v1/setup/lan-access.
type LanAccessSnapshot struct {
	AllowLAN        bool     `json:"allow_lan"`
	ListeningLAN    bool     `json:"listening_lan"`
	BindAddr        string   `json:"bind_addr"`
	Port            string   `json:"port"`
	Source          string   `json:"source"` // config | default
	RestartRequired bool     `json:"restart_required"`
	AgentLANIPs     []string `json:"agent_lan_ips"`
}

// handleGetLanAccess is the ONLY GET handler for LAN listen intent / status.
func handleGetLanAccess(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.LanAccessGet == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_configured", "lan access persistence not configured")
		return
	}
	snap, err := deps.LanAccessGet()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "lan_access_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handlePutLanAccess is the ONLY PUT handler for fiscal_allow_lan (admin + loopback).
func handlePutLanAccess(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if !IsLoopbackClient(r) {
		writeErr(w, http.StatusForbidden, "lan_loopback_only", "change LAN access on the Agent PC only")
		return
	}
	if deps.LanAccessSet == nil {
		writeErr(w, http.StatusServiceUnavailable, "not_configured", "lan access persistence not configured")
		return
	}
	var body struct {
		AllowLAN bool `json:"allow_lan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid JSON")
		return
	}
	snap, err := deps.LanAccessSet(body.AllowLAN)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "lan_access_save_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
