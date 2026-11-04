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
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "0")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	storeID := "store-pub-001"
	svc := service.New(db, nil, nil, dir, storeID)
	deps := HandlerDeps{Fiscal: svc, StoreID: storeID, DataDir: dir}
	sm, err := NewSessionManager(dir, false)
	if err != nil || sm == nil {
		t.Fatalf("session: err=%v sm=%v", err, sm)
	}
	deps.Sessions = sm

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
	if _, ok := body["ops_fiscal_entitled"]; !ok {
		t.Fatal("missing ops_fiscal_entitled")
	}
	if _, ok := body["cloud_paired_ok"]; !ok {
		t.Fatal("missing cloud_paired_ok")
	}
	// Unpaired + no local provision → must not allow bootstrap.
	if body["bootstrap_required"] == true {
		t.Fatal("unpaired empty store must not set bootstrap_required without Ops entitlement")
	}
}

func TestSetupStatusPublic_LocalProvisionAllowsBootstrap(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "1")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	storeID := "store-uat-001"
	svc := service.New(db, nil, nil, dir, storeID)
	deps := HandlerDeps{Fiscal: svc, StoreID: storeID, DataDir: dir}
	sm, err := NewSessionManager(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	deps.Sessions = sm

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
	if body["bootstrap_required"] != true {
		t.Fatal("FISCAL_ALLOW_LOCAL_PROVISION=1 must allow bootstrap_required on empty DB")
	}
	if body["ops_fiscal_entitled"] != true {
		t.Fatal("local provision must set ops_fiscal_entitled")
	}
}

func TestBuildSetupStatusPublic_BlocksBootstrapWhenOtherStoreHasOperators(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.BootstrapOwner("restaurant-real", "Admin", "123456"); err != nil {
		t.Fatal(err)
	}
	full := &store.SetupStatus{FiscalProfile: "restaurant", ReadyToIssue: false}
	pub, err := BuildSetupStatusPublic("store-demo-001", full, db, true, true, "restaurant")
	if err != nil {
		t.Fatal(err)
	}
	if pub.BootstrapRequired {
		t.Fatal("bootstrap_required must be false when another store already has operators")
	}
	if pub.OperatorsCount != 0 {
		t.Fatalf("operators_count for demo store=%d", pub.OperatorsCount)
	}

	green, err := BuildSetupStatusPublic("greenfield-store", &store.SetupStatus{}, db, true, true, "")
	// greenfield-store is also empty but other store has ops → still blocked
	if err != nil {
		t.Fatal(err)
	}
	if green.BootstrapRequired {
		t.Fatal("any empty store_id must not bootstrap when DB already has operators elsewhere")
	}

	// True greenfield: wipe then empty DB
	if _, err := db.SQL.Exec(`DELETE FROM operators`); err != nil {
		t.Fatal(err)
	}
	ok, err := BuildSetupStatusPublic("store-demo-001", full, db, true, true, "restaurant")
	if err != nil {
		t.Fatal(err)
	}
	if !ok.BootstrapRequired {
		t.Fatal("true empty DB + entitled must allow bootstrap_required")
	}

	blocked, err := BuildSetupStatusPublic("store-demo-001", full, db, false, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if blocked.BootstrapRequired {
		t.Fatal("empty DB without Ops entitlement must not allow bootstrap_required")
	}
}
