package service

import (
	"context"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// FiscalService is the application entry for issuance. ONLY orchestration — no second write path.
type FiscalService struct {
	db     *store.DB
	signer store.Signer
}

// New returns a FiscalService.
func New(db *store.DB, signer store.Signer) *FiscalService {
	return &FiscalService{db: db, signer: signer}
}

// IssueDocument signs a sale snapshot as FT (P0).
func (s *FiscalService) IssueDocument(ctx context.Context, req domain.IssueRequest, docType domain.DocumentType) (*domain.IssueResult, error) {
	if s == nil || s.db == nil || s.signer == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, fmt.Errorf("fiscal: request_id required")
	}
	if req.StoreID == "" {
		return nil, fmt.Errorf("fiscal: store_id required")
	}
	if req.OperatorID == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	if len(req.Snapshot.Lines) == 0 {
		return nil, fmt.Errorf("fiscal: lines required")
	}
	rec, err := s.db.IssueFT(ctx, s.signer, store.IssueParams{
		StoreID:    req.StoreID,
		RequestID:  req.RequestID,
		DocType:    docType,
		Snapshot:   req.Snapshot,
		OperatorID: req.OperatorID,
	})
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID:     rec.DocumentID,
		InvoiceNo:      rec.InvoiceNo,
		ATCUD:          rec.ATCUD,
		DocumentType:   rec.DocumentType,
		DocumentStatus: rec.DocumentStatus,
		PrintJobID:     rec.PrintJobID,
		PrintStatus:    rec.PrintStatus,
		IssuedAt:       rec.IssuedAt,
		IdempotentHit:  rec.IdempotentHit,
	}, nil
}

// GetByRequestID looks up a prior issue.
func (s *FiscalService) GetByRequestID(storeID, requestID string) (*domain.IssueResult, error) {
	rec, err := s.db.GetByRequestID(storeID, requestID)
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID:     rec.DocumentID,
		InvoiceNo:      rec.InvoiceNo,
		ATCUD:          rec.ATCUD,
		DocumentType:   rec.DocumentType,
		DocumentStatus: rec.DocumentStatus,
		PrintJobID:     rec.PrintJobID,
		PrintStatus:    rec.PrintStatus,
		IssuedAt:       rec.IssuedAt,
		IdempotentHit:  true,
	}, nil
}

// GetPrintJob proxies store.
func (s *FiscalService) GetPrintJob(id string) (*store.PrintJobView, error) {
	return s.db.GetPrintJob(id)
}
