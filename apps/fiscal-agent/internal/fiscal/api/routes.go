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
	StoreID string
}

// Mount registers fiscal local routes. Prefix: /local/v1
func Mount(mux *http.ServeMux, deps HandlerDeps) {
	mux.HandleFunc("GET /local/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "module": "fiscal"})
	})
	mux.HandleFunc("GET /local/v1/setup/status", func(w http.ResponseWriter, r *http.Request) {
		handleSetupStatus(w, r, deps)
	})
	mux.HandleFunc("PUT /local/v1/setup/taxpayer", func(w http.ResponseWriter, r *http.Request) {
		handleUpsertTaxpayer(w, r, deps)
	})
	mux.HandleFunc("PUT /local/v1/setup/at-credentials", func(w http.ResponseWriter, r *http.Request) {
		handleUpsertAT(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/setup/series/register", func(w http.ResponseWriter, r *http.Request) {
		handleRegisterSeries(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/setup/activate", func(w http.ResponseWriter, r *http.Request) {
		handleActivate(w, r, deps)
	})
	mux.HandleFunc("PUT /local/v1/setup/operator", func(w http.ResponseWriter, r *http.Request) {
		handleOperator(w, r, deps)
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
	mux.HandleFunc("GET /local/v1/bill-drafts", func(w http.ResponseWriter, r *http.Request) {
		handleListBillDrafts(w, r, deps)
	})
}

func handleSetupStatus(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	st, err := deps.Fiscal.SetupStatus(r.URL.Query().Get("store_id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "status_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func handleUpsertTaxpayer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body store.TaxpayerInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	if err := deps.Fiscal.UpsertTaxpayer(body); err != nil {
		writeCoded(w, err)
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
}

func handleUpsertAT(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID  string `json:"store_id"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.StoreID == "" {
		body.StoreID = deps.StoreID
	}
	if err := deps.Fiscal.UpsertATCredentials(body.StoreID, body.Username, body.Password); err != nil {
		writeCoded(w, err)
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
}

func handleRegisterSeries(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID    string `json:"store_id"`
		SeriesCode string `json:"series_code"`
		DocType    string `json:"document_type"`
		FiscalYear int    `json:"fiscal_year"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	st, err := deps.Fiscal.RegisterSeries(r.Context(), body.StoreID, body.SeriesCode, body.DocType, body.FiscalYear)
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func handleActivate(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		StoreID              string `json:"store_id"`
		ProductPrivateKeyPEM string `json:"product_private_key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	st, err := deps.Fiscal.ActivateFiscal(body.StoreID, body.ProductPrivateKeyPEM)
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func handleOperator(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	var body struct {
		ID          string `json:"id"`
		StoreID     string `json:"store_id"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if err := deps.Fiscal.UpsertOperator(body.ID, body.StoreID, body.Role, body.DisplayName); err != nil {
		writeCoded(w, err)
		return
	}
	st, _ := deps.Fiscal.SetupStatus(body.StoreID)
	writeJSON(w, http.StatusOK, st)
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
		writeCoded(w, err)
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
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id": res.DocumentID, "invoice_no": res.InvoiceNo, "atcud": res.ATCUD,
		"document_type": res.DocumentType, "document_status": res.DocumentStatus,
		"print_job_id": res.PrintJobID, "print_status": res.PrintStatus,
		"issued_at": res.IssuedAt.UTC().Format(time.RFC3339), "idempotent_hit": true,
	})
}

func handleGetPrintJob(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	job, err := deps.Fiscal.GetPrintJob(r.PathValue("printJobId"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "print job not found")
		return
	}
	if err != nil {
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func handleListBillDrafts(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	list, err := deps.Fiscal.ListBillDrafts(50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	type row struct {
		ID               string `json:"id"`
		RequestID        string `json:"request_id"`
		SourceSaleID     string `json:"source_sale_id"`
		Status           string `json:"status"`
		CloudJobID       string `json:"cloud_job_id,omitempty"`
		UpdatedAt        string `json:"updated_at"`
		TableDisplayName string `json:"table_display_name,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, d := range list {
		var meta struct {
			TableDisplayName string `json:"table_display_name"`
		}
		_ = json.Unmarshal([]byte(d.PayloadJSON), &meta)
		out = append(out, row{
			ID: d.ID, RequestID: d.RequestID, SourceSaleID: d.SourceSaleID,
			Status: d.Status, CloudJobID: d.CloudJobID, UpdatedAt: d.UpdatedAt,
			TableDisplayName: meta.TableDisplayName,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": out})
}

func writeCoded(w http.ResponseWriter, err error) {
	var ce *service.CodedError
	if errors.As(err, &ce) {
		status := http.StatusBadRequest
		switch ce.Code {
		case service.ErrCodeATSOAPFailed:
			status = http.StatusBadGateway
		case service.ErrCodeSignerNotReady, service.ErrCodeSeriesMissing, service.ErrCodeTaxpayerMissing, service.ErrCodeATCredsMissing:
			status = http.StatusConflict
		}
		writeErr(w, status, ce.Code, ce.Msg)
		return
	}
	writeErr(w, http.StatusBadRequest, "issue_failed", err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": code, "message": msg})
}
