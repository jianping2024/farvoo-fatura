package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func handleListProducts(w http.ResponseWriter, r *http.Request, deps HandlerDeps) {
	if deps.Fiscal == nil {
		writeErr(w, http.StatusServiceUnavailable, "fiscal_unavailable", "fiscal service not configured")
		return
	}
	list, err := deps.Fiscal.ListFiscalProducts(200)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": list})
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
	list, err := deps.Fiscal.ListCustomers(200)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"customers": list})
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
		RequestID     string                  `json:"request_id"`
		OperatorID    string                  `json:"operator_id"`
		StationID     string                  `json:"station_id"`
		CustomerNIF   string                  `json:"customer_nif"`
		CustomerName  string                  `json:"customer_name"`
		PaymentMethod string                  `json:"payment_method"`
		Lines         []catalog.ManualLineInput `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.OperatorID == "" {
		body.OperatorID = "op-demo-cashier"
	}
	res, err := deps.Fiscal.IssueManualFT(r.Context(), catalog.ManualIssueInput{
		RequestID: body.RequestID, CustomerNIF: body.CustomerNIF, CustomerName: body.CustomerName,
		PaymentMethod: body.PaymentMethod, Lines: body.Lines,
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
