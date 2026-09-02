package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestSetupStatusPublic_Anonymous(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	storeID := "store-pub-001"
	svc := service.New(db, nil, nil, dir, storeID)
	deps := HandlerDeps{Fiscal: svc, StoreID: storeID, DataDir: dir, Sessions: MustNewSessionManager(dir)}

	req := httptest.NewRequest(http.MethodGet, "/local/v1/setup/status", nil)
	rec := httptest.NewRecorder()
	handleSetupStatus(rec, req, deps)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"taxpayer_ok", "at_credentials_ok", "series_ok", "series_code", "validation_code",
		"at_env", "software_certificate_number", "signing_key_version", "cloud_jwt_hint",
	} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("anonymous status must not include %q", forbidden)
		}
	}
	if _, ok := body["bootstrap_required"]; !ok {
		t.Fatal("missing bootstrap_required")
	}
	if _, ok := body["operators_count"]; !ok {
		t.Fatal("missing operators_count")
	}
}
