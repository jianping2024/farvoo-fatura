package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeProformaNotAllowed = "proforma_not_allowed"
	ErrCodeAnnulNotAllowed    = "annul_not_allowed"
)

// IssueProforma is the ONLY orchestration entry for PF issuance.
func (s *FiscalService) IssueProforma(ctx context.Context, req domain.ProformaRequest) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if req.OperatorID == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if len(req.Snapshot.Lines) == 0 {
		return nil, coded(ErrCodeValidationFailed, "lines required")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	validUntil := strings.TrimSpace(req.ValidUntil)
	if validUntil != "" {
		if _, err := time.Parse("2006-01-02", validUntil); err != nil {
			return nil, coded(ErrCodeValidationFailed, "valid_until must be YYYY-MM-DD")
		}
	}
	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}
	rec, err := s.db.IssuePF(ctx, sig, store.IssuePFParams{
		StoreID:             req.StoreID,
		RequestID:           req.RequestID,
		Snapshot:            req.Snapshot,
		OperatorID:          req.OperatorID,
		StationID:           req.StationID,
		FiscalTerminalID:    req.FiscalTerminalID,
		FiscalTerminalLabel: req.FiscalTerminalLabel,
		InvoiceLocale:       s.invoiceLocale(),
		ValidUntil:          validUntil,
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrPFSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE PF series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "valid_until") || strings.Contains(err.Error(), "lines") {
			return nil, coded(ErrCodeValidationFailed, err.Error())
		}
		return nil, coded("issue_failed", err.Error())
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}, nil
}

// IssueManualProforma builds a manual snapshot then IssueProforma — ONLY manual PF entry.
func (s *FiscalService) IssueManualProforma(ctx context.Context, in catalog.ManualIssueInput, operatorID, stationID, fiscalTerminalID, fiscalTerminalLabel string) (*domain.IssueResult, error) {
	if strings.TrimSpace(in.RequestID) == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if strings.TrimSpace(operatorID) == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if strings.TrimSpace(in.PaymentMethod) == "" {
		in.PaymentMethod = "CASH" // PF ignores payments; snapshot builder still needs a known method
	}
	snap, err := catalog.BuildManualSaleSnapshot(s.db, in)
	if err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}
	return s.IssueProforma(ctx, domain.ProformaRequest{
		StoreID:             s.storeID,
		RequestID:           in.RequestID,
		OperatorID:          operatorID,
		StationID:           stationID,
		FiscalTerminalID:    fiscalTerminalID,
		FiscalTerminalLabel: fiscalTerminalLabel,
		Snapshot:            snap,
		ValidUntil:          in.ValidUntil,
	})
}

// AnnulProforma is the ONLY orchestration entry for PF annulment.
func (s *FiscalService) AnnulProforma(ctx context.Context, req domain.AnnulProformaRequest) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if req.OperatorID == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if req.DocumentID == "" {
		return nil, coded(ErrCodeValidationFailed, "document id required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, coded(ErrCodeValidationFailed, "reason required")
	}
	if utf8.RuneCountInString(reason) > 50 {
		return nil, coded(ErrCodeValidationFailed, "reason length must be 1-50")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	rec, err := s.db.AnnulPF(ctx, store.AnnulPFParams{
		StoreID:    req.StoreID,
		RequestID:  req.RequestID,
		DocumentID: req.DocumentID,
		OperatorID: req.OperatorID,
		Reason:     reason,
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "proforma not found")
	}
	if errors.Is(err, store.ErrProformaNotAllowed) {
		return nil, coded(ErrCodeAnnulNotAllowed, "proforma cannot be annulled")
	}
	if err != nil {
		return nil, coded("annul_failed", err.Error())
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}, nil
}
