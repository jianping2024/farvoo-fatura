package service

import (
	"context"
	"errors"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/domain"
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

// ListInvoices returns paginated invoices for Admin with optional date/search filters.
func (s *FiscalService) ListInvoices(q store.InvoiceListQuery) (*store.InvoiceListResult, error) {
	if q.StoreID == "" {
		q.StoreID = s.storeID
	}
	return s.db.ListInvoices(q)
}

// GetInvoiceDetail returns one invoice with credit/debit remaining (FT/FS/FR) or NC/ND original ref.
func (s *FiscalService) GetInvoiceDetail(documentID string) (*store.InvoiceDetail, error) {
	d, err := s.db.GetInvoiceDetail(documentID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded("not_found", "invoice not found")
	}
	if err != nil {
		return nil, err
	}
	switch {
	case store.IsCreditableOriginalDocumentType(d.DocumentType):
		rem, err := s.db.CreditRemainingForInvoice(documentID)
		if err != nil {
			return d, err
		}
		if rem != nil {
			d.CreditedGrossTotal = rem.CreditedGrossTotal
			d.RemainingGrossTotal = rem.RemainingGrossTotal
			d.Lines = rem.Lines
		}
		drem, err := s.db.DebitRemainingForInvoice(documentID)
		if err != nil {
			return d, err
		}
		if drem != nil {
			d.DebitedGrossTotal = drem.DebitedGrossTotal
			d.RemainingDebitGrossTotal = drem.RemainingGrossTotal
			d.DebitLines = drem.Lines
		}
	case d.DocumentType == domain.DocumentNC, d.DocumentType == domain.DocumentND:
		orig, err := s.db.CorrectiveOriginalForDocument(documentID)
		if err != nil {
			return d, err
		}
		if orig != nil {
			d.OriginalInvoiceID = orig.OriginalInvoiceID
			d.OriginalInvoiceNo = orig.OriginalInvoiceNo
			d.CreditReason = orig.CreditReason
		}
	}
	return d, nil
}
