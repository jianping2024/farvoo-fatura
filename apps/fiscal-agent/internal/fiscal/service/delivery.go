package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/catalog"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeDeliveryNotAllowed     = "delivery_not_allowed"
	ErrCodeATTransportUnavailable = "at_transport_unavailable"
	ErrCodeATTransportRejected    = "at_transport_rejected"
)

// IssueDeliveryNote is the ONLY orchestration entry for GR issuance.
// Order: validate → TransportClient.RegisterDocument → store.IssueGR.
func (s *FiscalService) IssueDeliveryNote(ctx context.Context, req domain.DeliveryNoteRequest) (*domain.IssueResult, error) {
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
	shipFrom := strings.TrimSpace(req.ShipFromAddress)
	shipTo := strings.TrimSpace(req.ShipToAddress)
	moveAt := strings.TrimSpace(req.MovementStartAt)
	if shipFrom == "" || shipTo == "" || moveAt == "" {
		return nil, coded(ErrCodeValidationFailed, "ship_from_address, ship_to_address, movement_start_at required")
	}
	if err := validateGRCustomer(req.Snapshot.Customer, req.RecipientIsVATPayer); err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}

	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}
	tc := s.transport
	if tc == nil {
		tc = at.NewTransportFromEnv()
	}

	// Provisional document number for AT communication (series seq reserved only after AT OK).
	provisionalNo := "GR/" + req.RequestID
	if len(provisionalNo) > 60 {
		provisionalNo = provisionalNo[:60]
	}
	moveDate := moveAt
	if len(moveDate) >= 10 {
		moveDate = moveDate[:10]
	}
	atRes, err := tc.RegisterDocument(ctx, at.TransportDocument{
		DocumentNumber:    provisionalNo,
		MovementType:      "GR",
		MovementStatus:    "N",
		MovementDate:      moveDate,
		MovementStartTime: moveAt,
		CustomerTaxID:     strings.TrimSpace(req.Snapshot.Customer.TaxID),
		CustomerName:      strings.TrimSpace(req.Snapshot.Customer.CompanyName),
		ShipFromAddress:   shipFrom,
		ShipToAddress:     shipTo,
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not wired") || strings.Contains(msg, "unavailable") {
			return nil, coded(ErrCodeATTransportUnavailable, msg)
		}
		return nil, coded(ErrCodeATTransportRejected, msg)
	}
	if atRes == nil || strings.TrimSpace(atRes.ATDocCodeID) == "" {
		return nil, coded(ErrCodeATTransportRejected, "empty ATDocCodeID")
	}

	rec, err := s.db.IssueGR(ctx, sig, store.IssueGRParams{
		StoreID:             req.StoreID,
		RequestID:           req.RequestID,
		Snapshot:            req.Snapshot,
		OperatorID:          req.OperatorID,
		StationID:           req.StationID,
		FiscalTerminalID:    req.FiscalTerminalID,
		FiscalTerminalLabel: req.FiscalTerminalLabel,
		InvoiceLocale:       s.invoiceLocale(),
		ShipFromAddress:     shipFrom,
		ShipToAddress:       shipTo,
		MovementStartAt:     moveAt,
		RecipientIsVATPayer: req.RecipientIsVATPayer,
		ATDocCodeID:         strings.TrimSpace(atRes.ATDocCodeID),
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrGRSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE GR series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "NIF") || strings.Contains(err.Error(), "address") {
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

func validateGRCustomer(c domain.CustomerInput, vatPayer bool) error {
	name := strings.TrimSpace(c.CompanyName)
	addr := strings.TrimSpace(c.AddressDetail)
	city := strings.TrimSpace(c.City)
	postal := strings.TrimSpace(c.PostalCode)
	nif := strings.TrimSpace(c.TaxID)
	if name == "" {
		return fmt.Errorf("customer name required")
	}
	if addr == "" || city == "" || postal == "" {
		return fmt.Errorf("customer address (detail/city/postal) required")
	}
	if vatPayer {
		if nif == "" || nif == "999999990" {
			return fmt.Errorf("VAT payer requires real NIF")
		}
	}
	return nil
}

// IssueManualDeliveryNote builds a manual snapshot then IssueDeliveryNote — ONLY manual GR entry.
func (s *FiscalService) IssueManualDeliveryNote(ctx context.Context, in catalog.ManualIssueInput, operatorID, stationID, fiscalTerminalID, fiscalTerminalLabel string) (*domain.IssueResult, error) {
	if strings.TrimSpace(in.RequestID) == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if strings.TrimSpace(operatorID) == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if strings.TrimSpace(in.PaymentMethod) == "" {
		in.PaymentMethod = "CASH"
	}
	vat := true
	if in.RecipientIsVATPayer != nil {
		vat = *in.RecipientIsVATPayer
	}
	snap, err := catalog.BuildManualSaleSnapshot(s.db, in)
	if err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}
	if a := strings.TrimSpace(in.CustomerAddress); a != "" {
		snap.Customer.AddressDetail = a
	}
	if c := strings.TrimSpace(in.CustomerCity); c != "" {
		snap.Customer.City = c
	}
	if p := strings.TrimSpace(in.CustomerPostalCode); p != "" {
		snap.Customer.PostalCode = p
	}
	if !vat {
		// Do not keep ApplyCustomerOverride's consumidor-final promotion for non-VAT GR.
		snap.Customer.TaxID = ""
	}
	return s.IssueDeliveryNote(ctx, domain.DeliveryNoteRequest{
		StoreID:             s.storeID,
		RequestID:           in.RequestID,
		OperatorID:          operatorID,
		StationID:           stationID,
		FiscalTerminalID:    fiscalTerminalID,
		FiscalTerminalLabel: fiscalTerminalLabel,
		Snapshot:            snap,
		ShipFromAddress:     in.ShipFromAddress,
		ShipToAddress:       in.ShipToAddress,
		MovementStartAt:     in.MovementStartAt,
		RecipientIsVATPayer: vat,
	})
}

// AnnulDeliveryNote is the ONLY orchestration entry for GR annulment.
func (s *FiscalService) AnnulDeliveryNote(ctx context.Context, req domain.AnnulDeliveryNoteRequest) (*domain.IssueResult, error) {
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
	rec, err := s.db.AnnulGR(ctx, store.AnnulGRParams{
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
		return nil, coded(ErrCodeNotFound, "delivery note not found")
	}
	if errors.Is(err, store.ErrDeliveryNotAllowed) {
		return nil, coded(ErrCodeAnnulNotAllowed, "delivery note cannot be annulled")
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
