package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestResolveRequestStoreID(t *testing.T) {
	deps := HandlerDeps{StoreID: "store-a"}
	id, err := resolveRequestStoreID(deps, "")
	if err != nil || id != "store-a" {
		t.Fatalf("empty: id=%q err=%v", id, err)
	}
	id, err = resolveRequestStoreID(deps, "store-a")
	if err != nil || id != "store-a" {
		t.Fatalf("match: id=%q err=%v", id, err)
	}
	_, err = resolveRequestStoreID(deps, "store-other")
	if err != ErrStoreIDMismatch {
		t.Fatalf("mismatch want ErrStoreIDMismatch got %v", err)
	}
}

func TestRequireActiveLANTerminal(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-lan-1"
	svc := service.New(db, nil, nil, dir, storeID)
	deps := HandlerDeps{Fiscal: svc, StoreID: storeID, DataDir: dir}

	// Loopback exempt without cookie.
	req := httptest.NewRequest(http.MethodPost, "/local/v1/fiscal-documents", nil)
	req.RemoteAddr = "127.0.0.1:9"
	rec := httptest.NewRecorder()
	if !deps.requireActiveLANTerminal(rec, req) {
		t.Fatal("loopback must pass without terminal cookie")
	}

	// LAN without cookie → 403 terminal_required.
	req = httptest.NewRequest(http.MethodPost, "/local/v1/fiscal-documents", nil)
	req.RemoteAddr = "192.168.1.20:9"
	rec = httptest.NewRecorder()
	if deps.requireActiveLANTerminal(rec, req) {
		t.Fatal("LAN without cookie must fail")
	}
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "terminal_required") {
		t.Fatalf("want terminal_required got %d %s", rec.Code, rec.Body.String())
	}

	tid, err := db.UpsertFiscalTerminal(storeID, "ops-ref-1", "Front", true)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/local/v1/fiscal-documents", nil)
	req.RemoteAddr = "192.168.1.20:9"
	req.AddCookie(&http.Cookie{Name: terminalCookieName, Value: tid})
	rec = httptest.NewRecorder()
	if !deps.requireActiveLANTerminal(rec, req) {
		t.Fatalf("active terminal must pass: %s", rec.Body.String())
	}

	// Revoke → Touch fails → terminal_revoked.
	if _, err := db.UpsertFiscalTerminal(storeID, "ops-ref-1", "Front", false); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/local/v1/fiscal-documents", nil)
	req.RemoteAddr = "192.168.1.20:9"
	req.AddCookie(&http.Cookie{Name: terminalCookieName, Value: tid})
	rec = httptest.NewRecorder()
	if deps.requireActiveLANTerminal(rec, req) {
		t.Fatal("revoked terminal must fail")
	}
	if !strings.Contains(rec.Body.String(), "terminal_revoked") {
		t.Fatalf("want terminal_revoked got %s", rec.Body.String())
	}
}

func TestSetupStatus_RevokedSessionGetsPublic(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_DEV_KEY", "1")
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "fiscal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeID := "store-status-1"
	opID, err := db.BootstrapOwner(storeID, "Admin", "123456")
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, nil, nil, dir, storeID)
	sm, err := NewSessionManager(dir, false)
	if err != nil || sm == nil {
		t.Fatalf("session: %v", err)
	}
	deps := HandlerDeps{Fiscal: svc, StoreID: storeID, DataDir: dir, Sessions: sm}

	recCookie := httptest.NewRecorder()
	if err := sm.SetSessionCookie(recCookie, Session{
		OperatorID: opID, Role: "admin", DisplayName: "Admin", Epoch: 0,
	}); err != nil {
		t.Fatal(err)
	}
	var sessCookie *http.Cookie
	for _, c := range recCookie.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessCookie = c
			break
		}
	}
	if sessCookie == nil {
		t.Fatal("missing session cookie")
	}

	// Valid session → full status (has taxpayer_ok field).
	req := httptest.NewRequest(http.MethodGet, "/local/v1/setup/status", nil)
	req.AddCookie(sessCookie)
	rec := httptest.NewRecorder()
	handleSetupStatus(rec, req, deps)
	var full map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &full); err != nil {
		t.Fatal(err)
	}
	if _, ok := full["taxpayer_ok"]; !ok {
		t.Fatalf("valid session must get full status: %s", rec.Body.String())
	}

	// Bump epoch (PIN change / deactivate) → Public only.
	if err := db.BumpOperatorSessionEpoch(storeID, opID); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/local/v1/setup/status", nil)
	req.AddCookie(sessCookie)
	rec = httptest.NewRecorder()
	handleSetupStatus(rec, req, deps)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoked must still 200 public, got %d", rec.Code)
	}
	var pub map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &pub); err != nil {
		t.Fatal(err)
	}
	if _, ok := pub["validation_code"]; ok {
		t.Fatal("revoked session must not see validation_code")
	}
	if _, ok := pub["taxpayer_ok"]; ok {
		t.Fatal("revoked session must not see taxpayer_ok")
	}
	if _, ok := pub["bootstrap_required"]; !ok {
		t.Fatal("revoked must get public shape")
	}
}

// TestSoleLANAuthWritings locks unique-path contracts for LAN terminal + status + store_id.
func TestSoleLANAuthWritings(t *testing.T) {
	root := "."
	files := []string{
		"auth_terminal.go", "auth_session.go", "store_id.go", "routes.go", "saft_handlers.go", "session.go",
	}
	var all strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		all.Write(b)
		all.WriteByte('\n')
	}
	src := all.String()

	mustOnce := []string{
		"func (deps HandlerDeps) requireActiveLANTerminal(",
		"func (deps HandlerDeps) refreshSessionFromDB(",
		"func (deps HandlerDeps) sessionIfValidCookie(",
		"func resolveRequestStoreID(",
		"func writeResolvedStoreID(",
		"deps.requireActiveLANTerminal(w, r)",
		"sess := deps.sessionIfValidCookie(r)",
		"hmac.Equal([]byte(m.sign(raw)), []byte(parts[1]))",
	}
	for _, needle := range mustOnce {
		n := strings.Count(src, needle)
		if n != 1 {
			t.Fatalf("%q count=%d want 1", needle, n)
		}
	}
	// No leftover empty→deps.StoreID defaults in HTTP handlers (resolver is sole).
	forbidden := []string{
		`if body.StoreID == "" {`,
		`if storeID == "" {\n\t\tstoreID = deps.StoreID`,
		"m.sign(raw) != parts[1]",
		"ParseRequest(r); err == nil && parsed != nil",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("forbidden leftover pattern %q", needle)
		}
	}
	// SSE must go through guardAuto (g(...)), not bare HandleFunc.
	if !strings.Contains(src, `mux.HandleFunc("GET /local/v1/events", g(func`) {
		t.Fatal("GET /local/v1/events must use guardAuto g(...)")
	}
	// SAFT must not trust body operator_id.
	saft, err := os.ReadFile("saft_handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saft), `OperatorID string`) {
		t.Fatal("saft export must not accept body operator_id")
	}
	if !strings.Contains(string(saft), "RequireOperatorID") {
		t.Fatal("saft export must RequireOperatorID from session")
	}
}
