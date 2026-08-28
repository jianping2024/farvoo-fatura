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

//go:embed ui/date-range.js
var fiscalUIDateRangeJS []byte

//go:embed ui/date-range.css
var fiscalUIDateRangeCSS []byte

//go:embed ui/list-pagination.js
var fiscalUIListPaginationJS []byte

//go:embed ui/list-pagination.css
var fiscalUIListPaginationCSS []byte

//go:embed ui/printer-station.js
var fiscalUIPrinterStationJS []byte

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
	mux.HandleFunc("GET /fiscal-ui/date-range.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIDateRangeJS)
	})
	mux.HandleFunc("GET /fiscal-ui/date-range.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIDateRangeCSS)
	})
	mux.HandleFunc("GET /fiscal-ui/list-pagination.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIListPaginationJS)
	})
	mux.HandleFunc("GET /fiscal-ui/list-pagination.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIListPaginationCSS)
	})
	mux.HandleFunc("GET /fiscal-ui/printer-station.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(fiscalUIPrinterStationJS)
	})
}
