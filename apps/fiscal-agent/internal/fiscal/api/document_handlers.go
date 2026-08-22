package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/service"
)

func handleListFiscalDocuments(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	list, err := deps.Fiscal.ListInvoices(limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": list})
}

func handleGetFiscalDocument(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	detail, err := deps.Fiscal.GetInvoiceDetail(r.PathValue("documentId"))
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) && ce.Code == "not_found" {
			writeErr(w, http.StatusNotFound, "not_found", ce.Msg)
			return
		}
		writeCoded(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":     detail.DocumentID,
		"invoice_no":      detail.InvoiceNo,
		"atcud":           detail.ATCUD,
		"document_type":   detail.DocumentType,
		"document_status": detail.DocumentStatus,
		"print_status":    detail.PrintStatus,
		"print_job_id":    detail.PrintJobID,
		"gross_total":     detail.GrossTotal,
		"net_total":       detail.NetTotal,
		"tax_payable":     detail.TaxPayable,
		"source_sale_id":  detail.SourceSaleID,
		"order_label":     detail.OrderLabel,
		"issued_at":       detail.IssuedAt.UTC().Format(time.RFC3339),
		"hash":            detail.Hash,
	})
}

func handleReprint(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	documentID := r.PathValue("documentId")
	var body struct {
		OperatorID string `json:"operator_id"`
		StationID  string `json:"station_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.OperatorID == "" {
		body.OperatorID = "op-demo-cashier"
	}
	res, err := deps.Fiscal.ReprintDocument(r.Context(), documentID, body.OperatorID, body.StationID)
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) && ce.Code == "not_found" {
			writeErr(w, http.StatusNotFound, "not_found", ce.Msg)
			return
		}
		writeCoded(w, err)
		return
	}
	job, _ := deps.Fiscal.GetPrintJob(res.PrintJobID)
	out := map[string]any{
		"document_id":   res.InvoiceID,
		"print_job_id":  res.PrintJobID,
		"print_status":  res.PrintStatus,
		"print_purpose": "REPRINT",
	}
	if job != nil {
		out["job_status"] = job.JobStatus
		out["print_purpose"] = job.PrintPurpose
	}
	writeJSON(w, http.StatusOK, out)
}
