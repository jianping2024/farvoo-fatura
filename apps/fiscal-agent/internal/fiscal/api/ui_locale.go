package api

import (
	"encoding/json"
	"io"
	"net/http"

	"farvoo-fiscal-agent/internal/fiscal/locale"
)

// handleGetUILocale is the ONLY Admin read of ui_locale (+ derived invoice_locale).
func handleGetUILocale(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	_ = r
	ui := "zh"
	if deps.UILocaleGet != nil {
		ui = locale.NormalizeUILocale(deps.UILocaleGet())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ui_locale":      ui,
		"invoice_locale": locale.InvoiceLocaleFromUI(ui),
	})
}

// handlePutUILocale is the ONLY Admin write of ui_locale (scheme A).
func handlePutUILocale(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		UILocale string `json:"ui_locale"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 512)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_request", "invalid json")
		return
	}
	ui := locale.NormalizeUILocale(body.UILocale)
	if deps.UILocaleSet == nil {
		writeErr(w, http.StatusInternalServerError, "not_configured", "ui_locale persistence not configured")
		return
	}
	if err := deps.UILocaleSet(ui); err != nil {
		writeErr(w, http.StatusInternalServerError, "save_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"ui_locale":      ui,
		"invoice_locale": locale.InvoiceLocaleFromUI(ui),
	})
}
