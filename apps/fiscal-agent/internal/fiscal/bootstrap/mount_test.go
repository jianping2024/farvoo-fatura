package bootstrap_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
	"farvoo-fiscal-agent/internal/fiscal/worker"
)

func TestMountRoutesHealth(t *testing.T) {
	dir := t.TempDir()
	rt, err := bootstrap.StartCore(bootstrap.Options{
		DBPath:    filepath.Join(dir, "f.db"),
		DataDir:   filepath.Join(dir, "sec"),
		StoreID:   "store-demo-001",
		PrintSink: &worker.MemorySink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	req := httptest.NewRequest(http.MethodGet, "/local/v1/health", nil)
	rr := httptest.NewRecorder()
	rt.Mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}
