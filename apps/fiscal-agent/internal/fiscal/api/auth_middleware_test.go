package api

import (
	"net/http/httptest"
	"testing"
)

func TestRouteAuthFor_SAFTExportsRequireOwner(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   routeAuth
	}{
		{"POST", "/local/v1/saft/exports", authOwner},
		{"GET", "/local/v1/saft/exports", authOwner},
		{"GET", "/local/v1/saft/exports/exp-1", authOwner},
		{"GET", "/local/v1/saft/exports/exp-1/download", authOwner},
		{"POST", "/local/v1/fiscal-documents", authSession},
		{"POST", "/local/v1/setup/change-pin", authSession},
		{"GET", "/local/v1/setup/status", authPublic},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		got := routeAuthFor(req)
		if got != tc.want {
			t.Fatalf("%s %s: got %v want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
