package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeReceiptNotAllowed     = "receipt_not_allowed"
	ErrCodeReceiptAmountExceeded = "receipt_amount_exceeded"
)

// IssueReceipt is the ONLY orchestration entry for RG receipts.
func (s *FiscalService) IssueReceipt(ctx context.Context, req domain.ReceiptRequest) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if req.OperatorID == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if req.OriginalInvoiceID == "" {
		return nil, coded(ErrCodeValidationFailed, "original invoice id required")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	if !req.ReceiveFull {
		amt := strings.TrimSpace(req.Amount)
		if amt == "" {
			return nil, coded(ErrCodeValidationFailed, "amount required unless receive_full")
		}
		d, err := compliance.ParseDecimal(amt)
		if err != nil || !d.IsPositive() {
			return nil, coded(ErrCodeValidationFailed, "amount must be a positive money value")
		}
	}
	payMethod := domain.NormalizePaymentMethod(req.PaymentMethod)
	if payMethod == domain.PaymentAccount {
		return nil, coded(ErrCodeValidationFailed, "payment_method cannot be ACCOUNT on receipt")
	}
	if !domain.IsKnownPaymentMethod(payMethod) {
		return nil, coded(ErrCodeValidationFailed, fmt.Sprintf("unknown payment_method %q", req.PaymentMethod))
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) > 200 {
		return nil, coded(ErrCodeValidationFailed, "reason length must be 1-200")
	}

	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}

	rec, err := s.db.IssueRG(ctx, sig, store.IssueRGParams{
		StoreID:             req.StoreID,
		RequestID:           req.RequestID,
		OriginalInvoiceID:   req.OriginalInvoiceID,
		OperatorID:          req.OperatorID,
		StationID:           req.StationID,
		FiscalTerminalID:    req.FiscalTerminalID,
		FiscalTerminalLabel: req.FiscalTerminalLabel,
		Amount:              req.Amount,
		ReceiveFull:         req.ReceiveFull,
		PaymentMethod:       payMethod,
		Reason:              reason,
		InvoiceLocale:       s.invoiceLocale(),
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "original invoice not found")
	}
	if errors.Is(err, store.ErrReceiptNotAllowed) {
		return nil, coded(ErrCodeReceiptNotAllowed, "original invoice cannot receive a receipt")
	}
	if errors.Is(err, store.ErrReceiptAmountExceeded) {
		return nil, coded(ErrCodeReceiptAmountExceeded, "receipt amount exceeds remaining receivable")
	}
	if errors.Is(err, store.ErrRGSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE RG series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "payment_method") || strings.Contains(err.Error(), "amount") {
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
