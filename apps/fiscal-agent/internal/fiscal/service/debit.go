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
	ErrCodeDebitNotAllowed     = "debit_not_allowed"
	ErrCodeDebitAmountExceeded = "debit_amount_exceeded"
)

// IssueDebitNote is the ONLY orchestration entry for ND debit notes.
func (s *FiscalService) IssueDebitNote(ctx context.Context, req domain.DebitNoteRequest) (*domain.IssueResult, error) {
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
	canND, err := s.db.OperatorCanIssueNC(req.StoreID, req.OperatorID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if !canND {
		return nil, coded(ErrCodeDebitNotAllowed, "operator cannot issue debit notes")
	}
	if !req.DebitFull && len(req.Lines) == 0 {
		return nil, coded(ErrCodeValidationFailed, "lines required for partial debit")
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

	rec, err := s.db.IssueND(ctx, sig, store.IssueNDParams{
		StoreID:           req.StoreID,
		RequestID:         req.RequestID,
		OriginalInvoiceID: req.OriginalInvoiceID,
		OperatorID:        req.OperatorID,
		StationID:         req.StationID,
		Reason:            reason,
		DebitFull:         req.DebitFull,
		Lines:             lines,
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "original invoice not found")
	}
	if errors.Is(err, store.ErrDebitNotAllowed) {
		return nil, coded(ErrCodeDebitNotAllowed, "original invoice cannot be debited")
	}
	if errors.Is(err, store.ErrDebitAmountExceeded) {
		return nil, coded(ErrCodeDebitAmountExceeded, "debit amount must be positive")
	}
	if errors.Is(err, store.ErrNDSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE ND series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "no debit lines") {
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
