package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// HandlerDeps groups dependencies for HTTP handlers.
type HandlerDeps struct {
	Fiscal  *service.FiscalService
	StoreID string // default store when body omits store_id (dev)
}

// Mount registers fiscal local routes. Prefix: /local/v1
func Mount(mux *http.ServeMux, deps HandlerDeps) {
	mux.HandleFunc("GET /local/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "module": "fiscal"})
	})
	mux.HandleFunc("POST /local/v1/fiscal-documents", func(w http.ResponseWriter, r *http.Request) {
		handleIssue(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/fiscal-documents/by-request/{requestId}", func(w http.ResponseWriter, r *http.Request) {
		handleGetByRequest(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/print-jobs/{printJobId}", func(w http.ResponseWriter, r *http.Request) {
		handleGetPrintJob(w, r, deps)
	})
}

type issueBody struct {
	StoreID    string              `json:"store_id"`
	RequestID  string              `json:"request_id"`
	OperatorID string              `json:"operator_id"`
	DocType    string              `json:"document_type"`
	Snapshot   domain.SaleSnapshot `json:"snapshot"`
}

func handleIssue(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body issueBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	docType := domain.DocumentType(body.DocType)
	if docType == "" {
		docType = domain.DocumentFT
	}
	res, err := deps.Fiscal.IssueDocument(r.Context(), domain.IssueRequest{
		StoreID: body.StoreID, RequestID: body.RequestID, OperatorID: body.OperatorID, Snapshot: body.Snapshot,
	}, docType)
	if errors.Is(err, store.ErrConflict) {
		writeErr(w, http.StatusConflict, "idempotency_conflict", err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "issue_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":     res.DocumentID,
		"invoice_no":      res.InvoiceNo,
		"atcud":           res.ATCUD,
		"document_type":   res.DocumentType,
		"document_status": res.DocumentStatus,
		"print_job_id":    res.PrintJobID,
		"print_status":    res.PrintStatus,
		"issued_at":       res.IssuedAt.UTC().Format(time.RFC3339),
		"idempotent_hit":  res.IdempotentHit,
	})
}

func handleGetByRequest(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	requestID := r.PathValue("requestId")
	storeID := r.URL.Query().Get("store_id")
	if storeID == "" {
		storeID = deps.StoreID
	}
	res, err := deps.Fiscal.GetByRequestID(storeID, requestID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "request not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "lookup_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":     res.DocumentID,
		"invoice_no":      res.InvoiceNo,
		"atcud":           res.ATCUD,
		"document_type":   res.DocumentType,
		"document_status": res.DocumentStatus,
		"print_job_id":    res.PrintJobID,
		"print_status":    res.PrintStatus,
		"issued_at":       res.IssuedAt.UTC().Format(time.RFC3339),
		"idempotent_hit":  true,
	})
}

func handleGetPrintJob(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	id := r.PathValue("printJobId")
	job, err := deps.Fiscal.GetPrintJob(id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "print job not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "lookup_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": code, "message": msg})
}
