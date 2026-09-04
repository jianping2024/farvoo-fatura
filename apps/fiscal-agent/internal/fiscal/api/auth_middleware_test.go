package api

import (
	"net/http/httptest"
	"testing"
)

func TestRouteAuthFor_M32cTiers(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   routeAuth
	}{
		{"POST", "/local/v1/saft/exports", authManager},
		{"GET", "/local/v1/saft/exports", authManager},
		{"GET", "/local/v1/saft/exports/exp-1", authManager},
		{"GET", "/local/v1/saft/exports/exp-1/download", authManager},
		{"GET", "/local/v1/audit-log", authManager},
		{"PUT", "/local/v1/setup/taxpayer", authManager},
		{"PUT", "/local/v1/setup/at-credentials", authAdmin},
		{"POST", "/local/v1/setup/series/register", authAdmin},
		{"POST", "/local/v1/setup/activate-from-cloud", authAdmin},
		{"POST", "/local/v1/setup/backup", authAdmin},
		{"PUT", "/local/v1/setup/terminals/max", authAdmin},
		{"GET", "/local/v1/setup/terminals", authManager},
		{"POST", "/local/v1/setup/terminals/allow-next", authManager},
		{"POST", "/local/v1/setup/terminals/abc/revoke", authManager},
		{"POST", "/local/v1/setup/terminals/pair", authPublic},
		{"GET", "/local/v1/setup/terminals/summary", authPublic},
		{"POST", "/local/v1/fiscal-documents", authSession},
		{"POST", "/local/v1/setup/change-pin", authSession},
		{"GET", "/local/v1/setup/status", authPublic},
		{"GET", "/local/v1/setup/ui-locale", authPublic},
		{"PUT", "/local/v1/setup/ui-locale", authSession},
		{"GET", "/local/v1/setup/lan-access", authSession},
		{"PUT", "/local/v1/setup/lan-access", authAdmin},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		got := routeAuthFor(req)
		if got != tc.want {
			t.Fatalf("%s %s: got %v want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestRoleAllowed_M32c(t *testing.T) {
	if !roleAllowed(authManager, "admin") || !roleAllowed(authManager, "owner") {
		t.Fatal("manager routes allow admin and owner")
	}
	if roleAllowed(authManager, "cashier") {
		t.Fatal("manager routes deny cashier")
	}
	if !roleAllowed(authAdmin, "admin") {
		t.Fatal("admin routes allow admin")
	}
	if roleAllowed(authAdmin, "owner") {
		t.Fatal("admin routes deny owner")
	}
}
