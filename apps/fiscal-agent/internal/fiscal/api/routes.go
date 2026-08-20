package api

import "net/http"

// Mount registers fiscal local routes on mux. Prefix: /local/v1
func Mount(mux *http.ServeMux, _ HandlerDeps) {
	// Scaffold: handlers wired in a later step.
	mux.HandleFunc("GET /local/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","module":"fiscal"}`))
	})
}

// HandlerDeps groups dependencies for HTTP handlers.
type HandlerDeps struct {
	// Fiscal *service.FiscalService
	// PrintWorker *print.Worker
}
