package api

import (
	"net/http"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/audit"
	"farvoo-fiscal-agent/internal/fiscal/service"
)

func parseAuditLogQuery(r *http.Request) service.AuditLogListInput {
	page := 1
	if q := r.URL.Query().Get("page"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			page = n
		}
	}
	// 0 → store.ListAuditLog applies AuditLogDefaultPageSize (ONLY default).
	pageSize := 0
	if q := r.URL.Query().Get("page_size"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			pageSize = n
		}
	}
	return service.AuditLogListInput{
		Page:       page,
		PageSize:   pageSize,
		Action:     strings.TrimSpace(r.URL.Query().Get("action")),
		OperatorID: strings.TrimSpace(r.URL.Query().Get("operator_id")),
		From:       strings.TrimSpace(r.URL.Query().Get("from")),
		To:         strings.TrimSpace(r.URL.Query().Get("to")),
	}
}

func handleListAuditLog(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	sess := SessionFromContext(r.Context())
	if sess == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
		return
	}
	in := parseAuditLogQuery(r)
	in.ViewerRole = sess.Role
	if sess.Role == "owner" && in.Action != "" && !audit.OwnerMayView(in.Action) {
		writeErr(w, http.StatusForbidden, "forbidden", "action not allowed for owner")
		return
	}
	result, err := deps.Fiscal.ListAuditLog(in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if result.Items == nil {
		result.Items = []service.AuditLogItem{}
	}
	if result.FilterActions == nil {
		result.FilterActions = []audit.FilterAction{}
	}
	writeJSON(w, http.StatusOK, result)
}
