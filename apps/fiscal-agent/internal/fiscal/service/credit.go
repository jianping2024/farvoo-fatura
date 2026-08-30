package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeCreditNotAllowed     = "credit_not_allowed"
	ErrCodeCreditAmountExceeded = "credit_amount_exceeded"
	ErrCodeValidationFailed     = "validation_failed"
	ErrCodeNotFound             = "not_found"
	ErrCodeIdempotencyConflict  = "idempotency_conflict"
)

// IssueCreditNote is the ONLY orchestration entry for NC credit notes.
func (s *FiscalService) IssueCreditNote(ctx context.Context, req domain.CreditNoteRequest) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if req.OperatorID == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, coded(ErrCodeValidationFailed, "reason required")
	}
	if len(reason) > 200 {
		return nil, coded(ErrCodeValidationFailed, "reason length must be 1-200")
	}
	if req.OriginalInvoiceID == "" {
		return nil, coded(ErrCodeValidationFailed, "original invoice id required")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	canNC, err := s.db.OperatorCanIssueNC(req.StoreID, req.OperatorID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if !canNC {
		return nil, coded(ErrCodeCreditNotAllowed, "operator cannot issue credit notes")
	}
	if !req.CreditFull && len(req.Lines) == 0 {
		return nil, coded(ErrCodeValidationFailed, "lines required for partial credit")
	}
	for _, ln := range req.Lines {
		if ln.OriginalLineNumber < 1 {
			return nil, coded(ErrCodeValidationFailed, "original_line_number required")
		}
		if q := strings.TrimSpace(ln.Quantity); q != "" {
			if _, err := billsync.ParseQtyString(q); err != nil {
				return nil, coded(ErrCodeValidationFailed, err.Error())
			}
		}
	}

	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}

	lines := make([]store.CreditLineInput, 0, len(req.Lines))
	for _, ln := range req.Lines {
		lines = append(lines, store.CreditLineInput{
			OriginalLineNumber: ln.OriginalLineNumber,
			Quantity:           ln.Quantity,
			LineGross:          ln.LineGross,
		})
	}

	rec, err := s.db.IssueNC(ctx, sig, store.IssueNCParams{
		StoreID:           req.StoreID,
		RequestID:         req.RequestID,
		OriginalInvoiceID: req.OriginalInvoiceID,
		OperatorID:        req.OperatorID,
		StationID:         req.StationID,
		Reason:            reason,
		CreditFull:        req.CreditFull,
		Lines:             lines,
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "original invoice not found")
	}
	if errors.Is(err, store.ErrCreditNotAllowed) {
		return nil, coded(ErrCodeCreditNotAllowed, "original invoice cannot be credited")
	}
	if errors.Is(err, store.ErrCreditAmountExceeded) {
		return nil, coded(ErrCodeCreditAmountExceeded, "credit amount exceeds remaining gross")
	}
	if errors.Is(err, store.ErrNCSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE NC series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "no credit lines") {
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
