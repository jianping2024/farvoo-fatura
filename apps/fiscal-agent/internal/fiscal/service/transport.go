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
	ErrCodeTransportNotAllowed = "transport_not_allowed"
)

// IssueTransportNote is the ONLY orchestration entry for GT issuance.
// Order: validate purpose/recipient → TransportClient.RegisterDocument(MovementType=GT) → store.IssueGT.
func (s *FiscalService) IssueTransportNote(ctx context.Context, req domain.TransportNoteRequest) (*domain.IssueResult, error) {
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
	purpose := strings.TrimSpace(req.MovementPurpose)
	if purpose != domain.MovementPurposeTransfer && purpose != domain.MovementPurposeCustomer {
		return nil, coded(ErrCodeValidationFailed, "movement_purpose must be transfer or customer")
	}
	shipFrom := strings.TrimSpace(req.ShipFromAddress)
	shipTo := strings.TrimSpace(req.ShipToAddress)
	moveAt := strings.TrimSpace(req.MovementStartAt)
	if shipFrom == "" || shipTo == "" || moveAt == "" {
		return nil, coded(ErrCodeValidationFailed, "ship_from_address, ship_to_address, movement_start_at required")
	}

	storeNIF, err := s.storeTaxpayerNIF(req.StoreID)
	if err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}
	vat := req.RecipientIsVATPayer
	if purpose == domain.MovementPurposeTransfer {
		vat = true
		nif := strings.TrimSpace(req.Snapshot.Customer.TaxID)
		if nif != storeNIF {
			return nil, coded(ErrCodeValidationFailed, "transfer requires customer NIF equal to store tax_registration_number")
		}
	} else {
		custNIF := strings.TrimSpace(req.Snapshot.Customer.TaxID)
		if custNIF == storeNIF {
			return nil, coded(ErrCodeValidationFailed, "customer purpose must not use store NIF")
		}
	}
	if err := validateGRCustomer(req.Snapshot.Customer, vat); err != nil {
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

	provisionalNo := "GT/" + req.RequestID
	if len(provisionalNo) > 60 {
		provisionalNo = provisionalNo[:60]
	}
	moveDate := moveAt
	if len(moveDate) >= 10 {
		moveDate = moveDate[:10]
	}
	atRes, err := tc.RegisterDocument(ctx, at.TransportDocument{
		DocumentNumber:    provisionalNo,
		MovementType:      "GT",
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

	rec, err := s.db.IssueGT(ctx, sig, store.IssueGTParams{
		StoreID:             req.StoreID,
		RequestID:           req.RequestID,
		Snapshot:            req.Snapshot,
		OperatorID:          req.OperatorID,
		StationID:           req.StationID,
		FiscalTerminalID:    req.FiscalTerminalID,
		FiscalTerminalLabel: req.FiscalTerminalLabel,
		InvoiceLocale:       s.invoiceLocale(),
		MovementPurpose:     purpose,
		ShipFromAddress:     shipFrom,
		ShipToAddress:       shipTo,
		MovementStartAt:     moveAt,
		RecipientIsVATPayer: vat,
		ATDocCodeID:         strings.TrimSpace(atRes.ATDocCodeID),
	})
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrGTSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE GT series with validation_code")
	}
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "NIF") || strings.Contains(err.Error(), "address") || strings.Contains(err.Error(), "movement_purpose") {
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

func (s *FiscalService) storeTaxpayerNIF(storeID string) (string, error) {
	var nif string
	err := s.db.SQL.QueryRow(`SELECT tax_registration_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&nif)
	if err != nil {
		return "", fmt.Errorf("taxpayer settings required")
	}
	nif = strings.TrimSpace(nif)
	if nif == "" {
		return "", fmt.Errorf("taxpayer NIF required")
	}
	return nif, nil
}

// IssueManualTransportNote builds a manual snapshot then IssueTransportNote — ONLY manual GT entry.
func (s *FiscalService) IssueManualTransportNote(ctx context.Context, in catalog.ManualIssueInput, operatorID, stationID, fiscalTerminalID, fiscalTerminalLabel string) (*domain.IssueResult, error) {
	if strings.TrimSpace(in.RequestID) == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if strings.TrimSpace(operatorID) == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	purpose := strings.TrimSpace(in.MovementPurpose)
	if purpose != domain.MovementPurposeTransfer && purpose != domain.MovementPurposeCustomer {
		return nil, coded(ErrCodeValidationFailed, "movement_purpose must be transfer or customer")
	}
	if strings.TrimSpace(in.PaymentMethod) == "" {
		in.PaymentMethod = "CASH"
	}
	vat := true
	if purpose == domain.MovementPurposeCustomer {
		if in.RecipientIsVATPayer != nil {
			vat = *in.RecipientIsVATPayer
		}
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
	if purpose == domain.MovementPurposeCustomer && !vat {
		snap.Customer.TaxID = ""
	}
	return s.IssueTransportNote(ctx, domain.TransportNoteRequest{
		StoreID:             s.storeID,
		RequestID:           in.RequestID,
		OperatorID:          operatorID,
		StationID:           stationID,
		FiscalTerminalID:    fiscalTerminalID,
		FiscalTerminalLabel: fiscalTerminalLabel,
		Snapshot:            snap,
		MovementPurpose:     purpose,
		ShipFromAddress:     in.ShipFromAddress,
		ShipToAddress:       in.ShipToAddress,
		MovementStartAt:     in.MovementStartAt,
		RecipientIsVATPayer: vat,
	})
}

// AnnulTransportNote is the ONLY orchestration entry for GT annulment.
func (s *FiscalService) AnnulTransportNote(ctx context.Context, req domain.AnnulTransportNoteRequest) (*domain.IssueResult, error) {
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
	rec, err := s.db.AnnulGT(ctx, store.AnnulGTParams{
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
		return nil, coded(ErrCodeNotFound, "transport note not found")
	}
	if errors.Is(err, store.ErrTransportNotAllowed) {
		return nil, coded(ErrCodeAnnulNotAllowed, "transport note cannot be annulled")
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
