package main

import (
	_ "embed"
	"net/http"
)

//go:embed wizard_ui_shared.js
var wizardUISharedJS []byte

func registerWizardUISharedRoute(mux *http.ServeMux) {
	mux.HandleFunc("/wizard-ui-shared.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(wizardUISharedJS)
	})
}

func isWizardStaticPath(path string) bool {
	return path == "/wizard-ui-shared.js"
}
