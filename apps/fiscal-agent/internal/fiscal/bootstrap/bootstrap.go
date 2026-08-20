// Package bootstrap wires fiscal modules into the print-agent process.
//
// Called from main after config load:
//   - Open SQLite fiscal DB
//   - Load signer / wrapped key
//   - Start fiscal print worker goroutine
//   - Mount api on local HTTP server (tray / wizard server or dedicated listener)
package bootstrap

import (
	"net/http"

	"farvoo-fiscal-agent/internal/fiscal/api"
)

// Options holds store path, network mode, and certificate number.
type Options struct {
	DBPath                    string
	BindHost                  string
	Port                      int
	SoftwareCertificateNumber string
}

// RegisterHTTP mounts fiscal routes on an existing mux (shared local server).
func RegisterHTTP(mux *http.ServeMux) {
	api.Mount(mux, api.HandlerDeps{})
}
