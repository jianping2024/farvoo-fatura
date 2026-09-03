package api

import (
	"net/http"
	"strings"
)

type routeAuth int

const (
	authPublic routeAuth = iota
	authSession
	authManager
	authAdmin
)

type routeSpec struct {
	method string
	path   string
	auth   routeAuth
}

// publicRoutes are anonymous whitelist (H1).
var publicRoutes = []routeSpec{
	{http.MethodGet, "/local/v1/health", authPublic},
	{http.MethodGet, "/local/v1/setup/status", authPublic},
	{http.MethodGet, "/local/v1/setup/ui-locale", authPublic},
	{http.MethodGet, "/local/v1/setup/operators", authPublic},
	{http.MethodPost, "/local/v1/setup/login", authPublic},
	{http.MethodPost, "/local/v1/setup/bootstrap-owner", authPublic},
	{http.MethodPost, "/local/v1/setup/terminals/pair", authPublic},
	{http.MethodGet, "/local/v1/setup/terminals/summary", authPublic},
	{http.MethodPost, "/local/v1/dev/bill-sync/pull", authPublic},
}

func routeAuthFor(r *http.Request) routeAuth {
	p := r.URL.Path
	if p != "/" && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	for _, spec := range publicRoutes {
		if r.Method == spec.method && p == spec.path {
			return authPublic
		}
	}
	adminPaths := map[string]bool{
		"/local/v1/setup/at-credentials":      true,
		"/local/v1/setup/series/register":     true,
		"/local/v1/setup/activate":            true,
		"/local/v1/setup/activate-from-cloud": true,
		"/local/v1/setup/backup":              true,
		"/local/v1/setup/integrity/verify":    true,
		"/local/v1/setup/prepare-swap":        true,
	}
	if adminPaths[p] {
		return authAdmin
	}
	managerPaths := map[string]bool{
		"/local/v1/setup/taxpayer": true,
		"/local/v1/setup/operator": true,
	}
	if managerPaths[p] {
		return authManager
	}
	if p == "/local/v1/setup/operators/manage" {
		return authManager
	}
	if strings.HasPrefix(p, "/local/v1/saft/exports") {
		return authManager
	}
	if p == "/local/v1/audit-log" {
		return authManager
	}
	return authSession
}

func roleAllowed(mode routeAuth, role string) bool {
	switch mode {
	case authAdmin:
		return role == "admin"
	case authManager:
		return role == "admin" || role == "owner"
	default:
		return true
	}
}

func forbiddenRole(w http.ResponseWriter, need string) {
	writeErr(w, http.StatusForbidden, "forbidden", need)
}
