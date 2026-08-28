package service

import (
	"context"
	"errors"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

// ReprintDocument enqueues a REPRINT job — ONLY reprint orchestration entry (no re-sign).
func (s *FiscalService) ReprintDocument(_ context.Context, documentID, operatorID, stationID string) (*store.ReprintResult, error) {
	if documentID == "" {
		return nil, fmt.Errorf("fiscal: document id required")
	}
	if operatorID == "" {
		operatorID = "op-demo-cashier"
	}
	res, err := s.db.CreateReprintPrintJob(documentID, operatorID, stationID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded("not_found", "invoice not found")
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListInvoices returns invoices for Admin with optional date/search filters.
func (s *FiscalService) ListInvoices(q store.InvoiceListQuery) ([]store.InvoiceListItem, error) {
	if q.StoreID == "" {
		q.StoreID = s.storeID
	}
	return s.db.ListInvoices(q)
}

// GetInvoiceDetail returns one invoice.
func (s *FiscalService) GetInvoiceDetail(documentID string) (*store.InvoiceDetail, error) {
	d, err := s.db.GetInvoiceDetail(documentID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded("not_found", "invoice not found")
	}
	return d, err
}
