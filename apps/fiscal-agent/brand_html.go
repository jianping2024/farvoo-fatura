package main

import "net/http"

func writeBrandHTML(w http.ResponseWriter, raw []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(fillBrandTokens(string(raw))))
}
