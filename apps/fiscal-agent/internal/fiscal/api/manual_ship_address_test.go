package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// Pre-structured clients sent free-text ship_from_address; silently ignoring it would fall
// back to default addresses, so the contract rejects it explicitly.
func TestIssueManual_RejectsLegacyShipAddressText(t *testing.T) {
	body := `{"request_id":"r1","document_type":"GR","ship_from_address":"Rua A 1, Lisboa"}`
	req := httptest.NewRequest(http.MethodPost, "/local/v1/fiscal-documents/manual", strings.NewReader(body))
	rr := httptest.NewRecorder()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	handleIssueManualFT(rr, req, HandlerDeps{Fiscal: service.New(db, nil, nil, dir, "store-demo-001")})
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "ship_from/ship_to") {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}
