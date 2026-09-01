package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func parseCatalogListQuery(r *http.Request) store.CatalogListQuery {
	page := 1
	if q := r.URL.Query().Get("page"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			page = n
		}
	}
	pageSize := 200
	if q := r.URL.Query().Get("page_size"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			pageSize = n
		}
	}
	return store.CatalogListQuery{
		Page:     page,
		PageSize: pageSize,
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
	}
}

func handleListProducts(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	q := parseCatalogListQuery(r)
	result, err := deps.Fiscal.ListFiscalProductsPaged(q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	items := result.Items
	if items == nil {
		items = []store.FiscalProductRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"products":  items,
		"page":      result.Page,
		"page_size": result.PageSize,
		"total":     result.Total,
	})
}

func handleUpsertProduct(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		ProductCode    string `json:"product_code"`
		DisplayName    string `json:"display_name"`
		SaftName       string `json:"saft_name"`
		UnitPriceGross string `json:"unit_price_gross"`
		VATRate        string `json:"vat_rate"`
		TaxCode        string `json:"tax_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	row, err := deps.Fiscal.UpsertLocalProduct(store.LocalProductInput{
		ProductCode: body.ProductCode, DisplayName: body.DisplayName, SaftName: body.SaftName,
		UnitPriceGross: body.UnitPriceGross, VATRate: body.VATRate, TaxCode: body.TaxCode,
	})
	if err != nil {
		if strings.Contains(err.Error(), "REMOTE_SYNC") {
			writeErr(w, http.StatusConflict, "product_conflict", err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func handleListCustomers(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	q := parseCatalogListQuery(r)
	result, err := deps.Fiscal.ListCustomersPaged(q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	items := result.Items
	if items == nil {
		items = []store.CustomerRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"customers": items,
		"page":      result.Page,
		"page_size": result.PageSize,
		"total":     result.Total,
	})
}

func handleUpsertCustomer(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		CustomerTaxID string `json:"customer_tax_id"`
		CompanyName   string `json:"company_name"`
		AddressDetail string `json:"address_detail"`
		City          string `json:"city"`
		PostalCode    string `json:"postal_code"`
		Country       string `json:"country"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	row, err := deps.Fiscal.UpsertLocalCustomer(store.LocalCustomerInput{
		CustomerTaxID: body.CustomerTaxID, CompanyName: body.CompanyName,
		AddressDetail: body.AddressDetail, City: body.City, PostalCode: body.PostalCode, Country: body.Country,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func handleIssueManualFT(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	var body struct {
		RequestID          string                    `json:"request_id"`
		OperatorID         string                    `json:"operator_id"`
		StationID          string                    `json:"station_id"`
		DocumentType       string                    `json:"document_type"`
		CustomerNIF        string                    `json:"customer_nif"`
		CustomerName       string                    `json:"customer_name"`
		PaymentMethod      string                    `json:"payment_method"`
		TableDisplayName   string                    `json:"table_display_name"`
		Lines              []catalog.ManualLineInput `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	id, ok := RequireOperatorID(w, r)
	if !ok {
		return
	}
	body.OperatorID = id
	res, err := deps.Fiscal.IssueManualFT(r.Context(), catalog.ManualIssueInput{
		RequestID: body.RequestID, DocumentType: body.DocumentType,
		CustomerNIF: body.CustomerNIF, CustomerName: body.CustomerName,
		PaymentMethod: body.PaymentMethod, TableDisplayName: body.TableDisplayName, Lines: body.Lines,
	}, body.OperatorID, body.StationID)
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
