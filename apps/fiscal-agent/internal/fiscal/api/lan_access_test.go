package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePutLanAccess_LoopbackOnly(t *testing.T) {
	deps := HandlerDeps{
		LanAccessSet: func(allow bool) (LanAccessSnapshot, error) {
			return LanAccessSnapshot{AllowLAN: allow, RestartRequired: true}, nil
		},
	}
	body, _ := json.Marshal(map[string]any{"allow_lan": true})
	req := httptest.NewRequest(http.MethodPut, "/local/v1/setup/lan-access", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.8:9"
	rec := httptest.NewRecorder()
	handlePutLanAccess(rec, req, deps)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHandlePutLanAccess_OK(t *testing.T) {
	deps := HandlerDeps{
		LanAccessSet: func(allow bool) (LanAccessSnapshot, error) {
			return LanAccessSnapshot{AllowLAN: allow, RestartRequired: true, Source: "config"}, nil
		},
	}
	body, _ := json.Marshal(map[string]any{"allow_lan": true})
	req := httptest.NewRequest(http.MethodPut, "/local/v1/setup/lan-access", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	handlePutLanAccess(rec, req, deps)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var snap LanAccessSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !snap.AllowLAN || !snap.RestartRequired {
		t.Fatalf("%+v", snap)
	}
}

func TestHandlePutLanAccess_EnvLocked(t *testing.T) {
	deps := HandlerDeps{
		LanAccessSet: func(allow bool) (LanAccessSnapshot, error) {
			return LanAccessSnapshot{}, ErrLanEnvLocked
		},
	}
	body, _ := json.Marshal(map[string]any{"allow_lan": false})
	req := httptest.NewRequest(http.MethodPut, "/local/v1/setup/lan-access", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	handlePutLanAccess(rec, req, deps)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPreferAgentLANIP(t *testing.T) {
	if got := PreferAgentLANIP([]string{"8.8.8.8", "192.168.1.10"}); got != "192.168.1.10" {
		t.Fatalf("got %q", got)
	}
}
