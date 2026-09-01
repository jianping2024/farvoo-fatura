package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBootstrapOwner_RejectsNonLoopback(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"display_name": "Owner", "pin": "123456"})
	req := httptest.NewRequest(http.MethodPost, "/local/v1/setup/bootstrap-owner", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.50:54321"
	w := httptest.NewRecorder()
	handleBootstrapOwner(w, req, HandlerDeps{StoreID: "store-1"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
