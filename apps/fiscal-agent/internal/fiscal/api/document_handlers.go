package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func handleListFiscalDocuments(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	page := 1
	if q := r.URL.Query().Get("page"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			page = n
		}
	}
	pageSize := 10
	if q := r.URL.Query().Get("page_size"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			pageSize = n
		}
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from != "" && !validDateYMD(from) {
		writeErr(w, http.StatusBadRequest, "invalid_from", "from must be YYYY-MM-DD")
		return
	}
	if to != "" && !validDateYMD(to) {
		writeErr(w, http.StatusBadRequest, "invalid_to", "to must be YYYY-MM-DD")
		return
	}
	result, err := deps.Fiscal.ListInvoices(store.InvoiceListQuery{
		Page:     page,
		PageSize: pageSize,
		From:     from,
		To:       to,
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	invoices := result.Items
	if invoices == nil {
		invoices = []store.InvoiceListItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"invoices":        invoices,
		"page":            result.Page,
		"page_size":       result.PageSize,
		"total":           result.Total,
		"gross_total_sum": result.GrossTotalSum,
		"from":            from,
		"to":              to,
		"q":               strings.TrimSpace(r.URL.Query().Get("q")),
	})
}

func validDateYMD(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
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
	out := map[string]any{
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
	}
	if store.IsCreditableOriginalDocumentType(detail.DocumentType) {
		if detail.CreditedGrossTotal != "" {
			out["credited_gross_total"] = detail.CreditedGrossTotal
			out["remaining_gross_total"] = detail.RemainingGrossTotal
		}
		if len(detail.Lines) > 0 {
			out["lines"] = detail.Lines
		}
		if detail.DebitedGrossTotal != "" {
			out["debited_gross_total"] = detail.DebitedGrossTotal
			out["remaining_debit_gross_total"] = detail.RemainingDebitGrossTotal
		}
		if len(detail.DebitLines) > 0 {
			out["debit_lines"] = detail.DebitLines
		}
	}
	if detail.OriginalInvoiceID != "" {
		out["original_invoice_id"] = detail.OriginalInvoiceID
		out["original_invoice_no"] = detail.OriginalInvoiceNo
		out["credit_reason"] = detail.CreditReason
	}
	writeJSON(w, http.StatusOK, out)
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

func handleCreditNote(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	documentID := r.PathValue("documentId")
	var body struct {
		RequestID  string                    `json:"request_id"`
		OperatorID string                    `json:"operator_id"`
		StationID  string                    `json:"station_id"`
		Reason     string                    `json:"reason"`
		CreditFull *bool                     `json:"credit_full"`
		Lines      []domain.CreditLineRequest `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.OperatorID == "" {
		body.OperatorID = "op-demo-cashier"
	}
	creditFull := false
	if body.CreditFull != nil {
		creditFull = *body.CreditFull
	}
	res, err := deps.Fiscal.IssueCreditNote(r.Context(), domain.CreditNoteRequest{
		StoreID:           deps.StoreID,
		RequestID:         body.RequestID,
		OriginalInvoiceID: documentID,
		OperatorID:        body.OperatorID,
		StationID:         body.StationID,
		Reason:            body.Reason,
		CreditFull:        creditFull,
		Lines:             body.Lines,
	})
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) && ce.Code == service.ErrCodeNotFound {
			writeErr(w, http.StatusNotFound, "not_found", ce.Msg)
			return
		}
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

func handleDebitNote(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	documentID := r.PathValue("documentId")
	var body struct {
		RequestID  string                    `json:"request_id"`
		OperatorID string                    `json:"operator_id"`
		StationID  string                    `json:"station_id"`
		Reason     string                    `json:"reason"`
		DebitFull  *bool                     `json:"debit_full"`
		Lines      []domain.CreditLineRequest `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.OperatorID == "" {
		body.OperatorID = "op-demo-cashier"
	}
	debitFull := false
	if body.DebitFull != nil {
		debitFull = *body.DebitFull
	}
	res, err := deps.Fiscal.IssueDebitNote(r.Context(), domain.DebitNoteRequest{
		StoreID:           deps.StoreID,
		RequestID:         body.RequestID,
		OriginalInvoiceID: documentID,
		OperatorID:        body.OperatorID,
		StationID:         body.StationID,
		Reason:            body.Reason,
		DebitFull:         debitFull,
		Lines:             body.Lines,
	})
	if err != nil {
		var ce *service.CodedError
		if errors.As(err, &ce) && ce.Code == service.ErrCodeNotFound {
			writeErr(w, http.StatusNotFound, "not_found", ce.Msg)
			return
		}
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
