package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/uievents"
)

// HandlerDeps groups dependencies for HTTP handlers.
type HandlerDeps struct {
	Fiscal            *service.FiscalService
	StoreID           string
	StationPrintersFn func() map[string]string // live Agent station_printers; may be nil
	UIEvents          *uievents.Hub            // Admin SSE; may be nil in unit tests
}

// PrinterStation is one mapped station for GET /local/v1/printers.
type PrinterStation struct {
	ID      string `json:"id"`
	Printer string `json:"printer"`
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
	mux.HandleFunc("POST /local/v1/fiscal-documents/manual", func(w http.ResponseWriter, r *http.Request) {
		handleIssueManualFT(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/products", func(w http.ResponseWriter, r *http.Request) {
		handleListProducts(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/products", func(w http.ResponseWriter, r *http.Request) {
		handleUpsertProduct(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/customers", func(w http.ResponseWriter, r *http.Request) {
		handleListCustomers(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/customers", func(w http.ResponseWriter, r *http.Request) {
		handleUpsertCustomer(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/fiscal-documents", func(w http.ResponseWriter, r *http.Request) {
		handleListFiscalDocuments(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/fiscal-documents/{documentId}", func(w http.ResponseWriter, r *http.Request) {
		handleGetFiscalDocument(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/fiscal-documents/{documentId}/reprints", func(w http.ResponseWriter, r *http.Request) {
		handleReprint(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/fiscal-documents/by-request/{requestId}", func(w http.ResponseWriter, r *http.Request) {
		handleGetByRequest(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/print-jobs/{printJobId}", func(w http.ResponseWriter, r *http.Request) {
		handleGetPrintJob(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/printers", func(w http.ResponseWriter, r *http.Request) {
		handleListPrinters(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/bill-drafts", func(w http.ResponseWriter, r *http.Request) {
		handleListBillDrafts(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/bill-drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleGetBillDraft(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/bill-drafts/{id}/issue", func(w http.ResponseWriter, r *http.Request) {
		handleIssueBillDraft(w, r, deps)
	})
	mux.HandleFunc("PUT /local/v1/bill-drafts/{id}/allocation", func(w http.ResponseWriter, r *http.Request) {
		handleSaveBillDraftAllocation(w, r, deps)
	})
	mux.HandleFunc("POST /local/v1/bill-drafts/{id}/discard", func(w http.ResponseWriter, r *http.Request) {
		handleDiscardBillDraft(w, r, deps)
	})
	mux.HandleFunc("GET /local/v1/events", func(w http.ResponseWriter, r *http.Request) {
		if deps.UIEvents == nil {
			writeErr(w, http.StatusServiceUnavailable, "events_unavailable", "UI events hub not configured")
			return
		}
		deps.UIEvents.ServeSSE(w, r)
	})
	// Dev/UAT only: run the SAME PullAndIngest path in-process so SSE Hub sees draft writers.
	mux.HandleFunc("POST /local/v1/dev/bill-sync/pull", func(w http.ResponseWriter, r *http.Request) {
		handleDevBillSyncPull(w, r, deps)
	})
}

// handleDevBillSyncPull is the ONLY HTTP trigger for billsync.PullAndIngest (fiscal-local UAT).
// Gated by FISCAL_ALLOW_DEV_KEY=1. Uses FARVOO_API + FARVOO_JWT — same as production doorbell.
func handleDevBillSyncPull(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if os.Getenv("FISCAL_ALLOW_DEV_KEY") != "1" {
		writeErr(w, http.StatusForbidden, "dev_forbidden", "set FISCAL_ALLOW_DEV_KEY=1 for local UAT pull")
		return
	}
	if deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	apiBase := strings.TrimSpace(os.Getenv("FARVOO_API"))
	jwt := strings.TrimSpace(os.Getenv("FARVOO_JWT"))
	if apiBase == "" || jwt == "" {
		writeErr(w, http.StatusBadRequest, "farvoo_env_missing", "FARVOO_API and FARVOO_JWT required")
		return
	}
	n, err := (&billsync.Puller{APIBase: apiBase, JWT: jwt, DB: deps.Fiscal.DB()}).PullAndIngest(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "bill_sync_pull_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processed": n})
}

func handleListPrinters(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	stations := []PrinterStation{}
	if deps.StationPrintersFn != nil {
		for id, raw := range deps.StationPrintersFn() {
			id = strings.TrimSpace(id)
			raw = strings.TrimSpace(raw)
			if id == "" || raw == "" {
				continue
			}
			stations = append(stations, PrinterStation{ID: id, Printer: raw})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": stations})
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
	StationID  string              `json:"station_id"`
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
		StoreID: body.StoreID, RequestID: body.RequestID, OperatorID: body.OperatorID,
		StationID: body.StationID, Snapshot: body.Snapshot,
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
		ScopeType        string `json:"scope_type,omitempty"`
		GrossTotal       string `json:"gross_total,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, d := range list {
		var meta struct {
			TableDisplayName string `json:"table_display_name"`
			ScopeType        string `json:"scope_type"`
		}
		_ = json.Unmarshal([]byte(d.PayloadJSON), &meta)
		out = append(out, row{
			ID: d.ID, RequestID: d.RequestID, SourceSaleID: d.SourceSaleID,
			Status: d.Status, CloudJobID: d.CloudJobID, UpdatedAt: d.UpdatedAt,
			TableDisplayName: meta.TableDisplayName, ScopeType: meta.ScopeType,
			GrossTotal:       billsync.ListGrossTotalFromPayload(d.PayloadJSON),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": out})
}

func handleGetBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	detail, err := deps.Fiscal.GetBillDraftDetail(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func handleDiscardBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	if err := deps.Fiscal.DiscardBillDrafts(r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "discard_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleIssueBillDraft(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	id := r.PathValue("id")
	var body struct {
		OperatorID           string `json:"operator_id"`
		Mode                 string `json:"mode"`
		ScopeID              string `json:"scope_id"`
		StationID            string `json:"station_id"`
		CustomerNIF          string `json:"customer_nif"`
		CustomerName         string `json:"customer_name"`
		AllocationRevision   *int64 `json:"allocation_revision"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.OperatorID == "" {
		body.OperatorID = "op-demo-cashier"
	}
	if body.Mode == "" {
		body.Mode = "whole_table"
	}
	res, err := deps.Fiscal.IssueFromBillDraft(r.Context(), service.IssueBillDraftInput{
		DraftID: id, OperatorID: body.OperatorID, Mode: body.Mode, ScopeID: body.ScopeID,
		StationID: body.StationID, CustomerNIF: body.CustomerNIF, CustomerName: body.CustomerName,
		AllocationRevision: body.AllocationRevision,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		if ie := billsync.AsIngestError(err); ie != nil {
			writeErr(w, http.StatusBadRequest, ie.Code, ie.Message)
			return
		}
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func handleSaveBillDraftAllocation(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		ExpectedRevision int64               `json:"expected_revision"`
		Allocation       billsync.Allocation `json:"allocation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "validation_failed", "invalid json")
		return
	}
	detail, err := deps.Fiscal.SaveBillDraftAllocation(service.SaveBillDraftAllocationInput{
		DraftID: r.PathValue("id"), ExpectedRevision: body.ExpectedRevision, Allocation: body.Allocation,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "draft_not_found", err.Error())
			return
		}
		if ie := billsync.AsIngestError(err); ie != nil {
			writeErr(w, http.StatusBadRequest, ie.Code, ie.Message)
			return
		}
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
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
		case "scope_mutex", "allocation_conflict", "draft_not_open", "already_invoiced":
			status = http.StatusConflict
		case "validation_failed":
			status = http.StatusBadRequest
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
