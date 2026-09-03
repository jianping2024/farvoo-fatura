package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"farvoo-fiscal-agent/internal/fiscal/service"
)

func handleExportSAFT(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		StoreID string `json:"store_id"`
		Year    int    `json:"year"`
		Month   int    `json:"month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	storeID, ok := writeResolvedStoreID(w, deps, body.StoreID)
	if !ok {
		return
	}
	opID, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	res, err := deps.Fiscal.ExportSAFT(r.Context(), service.ExportSAFTInput{
		StoreID: storeID, Year: body.Year, Month: body.Month, OperatorID: opID,
	})
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func handleListSAFTExports(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	storeID, ok := writeResolvedStoreID(w, deps, r.URL.Query().Get("store_id"))
	if !ok {
		return
	}
	year, month := 0, 0
	if q := r.URL.Query().Get("year"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			year = n
		}
	}
	if q := r.URL.Query().Get("month"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			month = n
		}
	}
	rows, err := deps.Fiscal.ListSAFTExports(storeID, year, month)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exports": rows})
}

func handleDownloadSAFTExport(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	row, data, err := deps.Fiscal.GetSAFTExportFile(r.PathValue("exportId"))
	if err != nil {
		writeCoded(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=windows-1252")
	w.Header().Set("Content-Disposition", `attachment; filename="`+row.FileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func handleGetSAFTExport(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	row, err := deps.Fiscal.GetSAFTExport(r.PathValue("exportId"))
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}
