package service

import (
	"context"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// IssueManualFT builds snapshot via catalog.BuildManualSaleSnapshot then IssueDocument — ONLY manual FT entry.
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
	return s.IssueDocument(ctx, domain.IssueRequest{
		StoreID:    s.storeID,
		RequestID:  in.RequestID,
		OperatorID: operatorID,
		StationID:  stationID,
		Snapshot:   snap,
	}, domain.DocumentFT)
}

// ListFiscalProducts proxies store.
func (s *FiscalService) ListFiscalProducts(limit int) ([]store.FiscalProductRow, error) {
	return s.db.ListFiscalProducts(limit)
}

// UpsertLocalProduct proxies store.
func (s *FiscalService) UpsertLocalProduct(in store.LocalProductInput) (*store.FiscalProductRow, error) {
	return s.db.UpsertLocalFiscalProduct(in)
}

// ListCustomers proxies store.
func (s *FiscalService) ListCustomers(limit int) ([]store.CustomerRow, error) {
	return s.db.ListCustomers(limit)
}

// UpsertLocalCustomer proxies store.
func (s *FiscalService) UpsertLocalCustomer(in store.LocalCustomerInput) (*store.CustomerRow, error) {
	return s.db.UpsertLocalCustomer(in)
}
