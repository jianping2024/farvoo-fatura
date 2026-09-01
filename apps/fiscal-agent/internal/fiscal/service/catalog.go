package service

import (
	"context"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// IssueManualFT builds snapshot via catalog.BuildManualSaleSnapshot then IssueDocument — manual FT/FS/FR entry.
func (s *FiscalService) IssueManualFT(ctx context.Context, in catalog.ManualIssueInput, operatorID, stationID string) (*domain.IssueResult, error) {
	if strings.TrimSpace(in.RequestID) == "" {
		return nil, fmt.Errorf("fiscal: request_id required")
	}
	if strings.TrimSpace(operatorID) == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	snap, err := catalog.BuildManualSaleSnapshot(s.db, in)
	if err != nil {
		return nil, err
	}
	docType, err := ResolveSaleDocumentType(in.DocumentType)
	if err != nil {
		return nil, err
	}
	return s.IssueDocument(ctx, domain.IssueRequest{
		StoreID:    s.storeID,
		RequestID:  in.RequestID,
		OperatorID: operatorID,
		StationID:  stationID,
		Snapshot:   snap,
	}, docType)
}

// ListFiscalProducts proxies store (legacy full list; prefer ListFiscalProductsPaged).
func (s *FiscalService) ListFiscalProducts(limit int) ([]store.FiscalProductRow, error) {
	return s.db.ListFiscalProducts(limit)
}

// ListFiscalProductsPaged proxies store.
func (s *FiscalService) ListFiscalProductsPaged(q store.CatalogListQuery) (*store.ProductListResult, error) {
	return s.db.ListFiscalProductsPaged(q)
}

// UpsertLocalProduct proxies store.
func (s *FiscalService) UpsertLocalProduct(in store.LocalProductInput) (*store.FiscalProductRow, error) {
	return s.db.UpsertLocalFiscalProduct(in)
}

// ListCustomers proxies store (legacy full list; prefer ListCustomersPaged).
func (s *FiscalService) ListCustomers(limit int) ([]store.CustomerRow, error) {
	return s.db.ListCustomers(limit)
}

// ListCustomersPaged proxies store.
func (s *FiscalService) ListCustomersPaged(q store.CatalogListQuery) (*store.CustomerListResult, error) {
	return s.db.ListCustomersPaged(q)
}

// UpsertLocalCustomer proxies store.
func (s *FiscalService) UpsertLocalCustomer(in store.LocalCustomerInput) (*store.CustomerRow, error) {
	return s.db.UpsertLocalCustomer(in)
}
