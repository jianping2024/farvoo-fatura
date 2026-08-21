package bootstrap

import (
	_ "embed"
	"net/http"
)

// Shared Fiscal Admin UI assets (restaurant design: one Toast, pages call showToast).
//
//go:embed ui/toast.js
var fiscalUIToastJS []byte

//go:embed ui/toast.css
var fiscalUIToastCSS []byte

func registerFiscalUIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /fiscal-ui/toast.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIToastJS)
	})
	mux.HandleFunc("GET /fiscal-ui/toast.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIToastCSS)
	})
}
