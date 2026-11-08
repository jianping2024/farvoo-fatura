package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/protect"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// ErrCodeTransportAddressIncomplete: GR/GT customer, load or unload address is not a
// complete Portuguese address (AT rule, enforced in every env so mock matches live).
const ErrCodeTransportAddressIncomplete = "transport_address_incomplete"

// customerAddress is the customer snapshot's address as a structured value.
func customerAddress(c domain.CustomerInput) domain.Address {
	return domain.Address{Detail: c.AddressDetail, City: c.City, PostalCode: c.PostalCode, Country: c.Country}
}

// resolveShipPoints is the ONLY GR/GT address rule: an omitted load point is the store
// address, an omitted unload point is the customer address; customer + both points must
// then be complete PT addresses.
func (s *FiscalService) resolveShipPoints(storeID string, cust domain.CustomerInput, from, to domain.Address) (domain.Address, domain.Address, error) {
	if from.IsZero() {
		t, err := s.db.GetTaxpayerSettings(storeID)
		if errors.Is(err, store.ErrNotFound) {
			return from, to, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
		}
		if err != nil {
			return from, to, err
		}
		from = domain.Address{Detail: t.AddressDetail, City: t.City, PostalCode: t.PostalCode, Country: t.Country}
	}
	if to.IsZero() {
		to = customerAddress(cust)
	}
	for _, p := range []struct {
		field string
		addr  domain.Address
	}{{"customer", customerAddress(cust)}, {"ship_from", from}, {"ship_to", to}} {
		if err := p.addr.ValidatePT(p.field); err != nil {
			var ia *domain.IncompleteAddressError
			if errors.As(err, &ia) {
				return from, to, codedField(ErrCodeTransportAddressIncomplete, err.Error(), ia.Field, ia.Part)
			}
			return from, to, coded(ErrCodeTransportAddressIncomplete, err.Error())
		}
	}
	// Customer text was checked with the snapshot (validateSaleText); the two points here.
	from, err := from.Normalize().ValidateSAFTText("ship_from")
	if err != nil {
		return from, to, err
	}
	to, err = to.Normalize().ValidateSAFTText("ship_to")
	if err != nil {
		return from, to, err
	}
	return from, to, nil
}

func toATAddress(a domain.Address) at.TransportAddress {
	n := a.Normalize()
	return at.TransportAddress{Detail: n.Detail, City: n.City, PostalCode: n.PostalCode}
}

// transportComm is the AT result plus the number it was registered under.
type transportComm struct {
	prep  *store.TransportPrep
	atRes *at.TransportResult
}

// issueResultFromRecord maps a store record (fresh or idempotent) to the API result.
func issueResultFromRecord(rec *store.IssueRecord) *domain.IssueResult {
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}
}

// normalizeMovementStart yields Lisbon wall time "YYYY-MM-DDTHH:MM:SS" (AT xsd:dateTime):
// pads "YYYY-MM-DDTHH:MM" and converts RFC3339 (DeliveryNoteRequest allows both).
func normalizeMovementStart(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 16 {
		return s + ":00"
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		loc, err := time.LoadLocation("Europe/Lisbon")
		if err != nil {
			loc = time.UTC
		}
		return t.In(loc).Format("2006-01-02T15:04:05")
	}
	return s
}

func lisbonLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		return time.UTC
	}
	return loc
}

// assertMovementStartNotPast is the ONLY GR/GT rule that movement_start_at must not be
// earlier than the current Lisbon minute (AT ReturnCode -100). Call after normalizeMovementStart.
func assertMovementStartNotPast(moveAt string) error {
	loc := lisbonLocation()
	t, err := time.ParseInLocation("2006-01-02T15:04:05", strings.TrimSpace(moveAt), loc)
	if err != nil {
		return coded(ErrCodeValidationFailed, "movement_start_at must be YYYY-MM-DDTHH:MM:SS")
	}
	now := time.Now().In(loc).Truncate(time.Minute)
	if t.Before(now) {
		return coded(ErrCodeMovementStartPast, "movement start is before current Lisbon time")
	}
	return nil
}

// mapTransportRegisterError maps AT RegisterDocument failures to stable service codes.
// ReturnCode -100 / hint movement_start_past → ErrCodeMovementStartPast (same as local assert).
func mapTransportRegisterError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, at.ErrTransportInvalid):
		return coded(ErrCodeValidationFailed, err.Error())
	case errors.Is(err, at.ErrTransportUnavailable):
		return coded(ErrCodeATTransportUnavailable, err.Error())
	default:
		var te *at.TransportError
		if !errors.As(err, &te) {
			return coded(ErrCodeATTransportRejected, err.Error())
		}
		switch te.Hint {
		case "movement_start_past":
			return coded(ErrCodeMovementStartPast, te.Message)
		case "number_taken":
			return coded(ErrCodeATNumberTaken, te.Message)
		}
		// AT's own ReturnMessage (e.g. -1 "O NIF do destinatário … não é válido"); Admin prefixes it.
		return coded(ErrCodeATTransportRejected, te.Message)
	}
}

// communicateTransport registers a new GR/GT: reserve the real next number
// (PrepareTransport), then envioDocumentoTransporte with MovementStatus=N.
// Caller must hold s.transportMu until store.IssueGR/IssueGT (ExpectedSeq) commits.
// Annulment uses communicateAnnul (same RegisterDocument, status A, no new number).
func (s *FiscalService) communicateTransport(ctx context.Context, storeID, docType string,
	snap domain.SaleSnapshot, shipFrom, shipTo domain.Address, moveAt string) (*transportComm, error) {
	prep, err := s.db.PrepareTransport(storeID, docType, snap)
	if errors.Is(err, store.ErrGRSeriesMissing) || errors.Is(err, store.ErrGTSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE "+docType+" series with validation_code")
	}
	if err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}
	user, pass, err := s.transportCredentials(storeID)
	if err != nil {
		return nil, err
	}
	doc := at.TransportDocument{
		Username: user, Password: pass,
		TaxRegistrationNumber: prep.IssuerNIF,
		CompanyName:           prep.IssuerName,
		CompanyAddress:        at.TransportAddress{Detail: prep.IssuerAddr, City: prep.IssuerCity, PostalCode: prep.IssuerPost},
		DocumentNumber:        prep.InvoiceNo,
		ATCUD:                 prep.ATCUD,
		MovementType:          docType,
		MovementStatus:        "N",
		MovementStartTime:     moveAt,
		CustomerTaxID:         strings.TrimSpace(snap.Customer.TaxID),
		CustomerName:          strings.TrimSpace(snap.Customer.CompanyName),
		CustomerAddress:       toATAddress(customerAddress(snap.Customer)),
		ShipFrom:              toATAddress(shipFrom),
		ShipTo:                toATAddress(shipTo),
	}
	if len(moveAt) >= 10 {
		doc.MovementDate = moveAt[:10]
	}
	for _, ln := range prep.Lines {
		doc.Lines = append(doc.Lines, at.TransportLine{
			Description: ln.Description, Quantity: ln.Quantity, UnitOfMeasure: ln.UnitOfMeasure, UnitPrice: ln.UnitPriceNet,
		})
	}
	tc := s.transport
	if tc == nil {
		tc = at.NewTransportFromEnv()
	}
	atRes, err := tc.RegisterDocument(ctx, doc)
	if err != nil {
		return nil, mapTransportRegisterError(err)
	}
	if atRes == nil || strings.TrimSpace(atRes.ATDocCodeID) == "" {
		return nil, coded(ErrCodeATTransportRejected, "empty ATDocCodeID")
	}
	return &transportComm{prep: prep, atRes: atRes}, nil
}

// transportCredentials reads the AT sub-user for storeID. Missing credentials yield empty
// user and password (mock ignores them). ONLY reader of at_credentials for transport SOAP.
func (s *FiscalService) transportCredentials(storeID string) (string, string, error) {
	cred, err := s.db.GetATCredentials(storeID)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	p, err := protect.Open(s.dataDir, cred.PasswordCiphertext, cred.WrapMeta)
	if err != nil {
		return "", "", err
	}
	return cred.Username, string(p), nil
}

// communicateAnnul resends an issued GR/GT with MovementStatus=A on the same
// RegisterDocument path. It does not allocate a number and does not replace
// at_transport_doc_code. Caller must hold s.transportMu.
func (s *FiscalService) communicateAnnul(ctx context.Context, storeID string, m *store.CommunicatedMovement) error {
	user, pass, err := s.transportCredentials(storeID)
	if err != nil {
		return err
	}
	moveAt := normalizeMovementStart(m.MovementStart)
	doc := at.TransportDocument{
		Username: user, Password: pass,
		TaxRegistrationNumber: m.IssuerNIF,
		CompanyName:           m.IssuerName,
		CompanyAddress:        at.TransportAddress{Detail: m.IssuerAddr, City: m.IssuerCity, PostalCode: m.IssuerPost},
		DocumentNumber:        m.InvoiceNo,
		ATCUD:                 m.ATCUD,
		MovementType:          m.DocumentType,
		MovementStatus:        "A",
		MovementStartTime:     moveAt,
		CustomerTaxID:         strings.TrimSpace(m.Customer.TaxID),
		CustomerName:          strings.TrimSpace(m.Customer.CompanyName),
		CustomerAddress:       toATAddress(customerAddress(m.Customer)),
		ShipFrom:              toATAddress(m.ShipFrom),
		ShipTo:                toATAddress(m.ShipTo),
	}
	if len(moveAt) >= 10 {
		doc.MovementDate = moveAt[:10]
	}
	for _, ln := range m.Lines {
		doc.Lines = append(doc.Lines, at.TransportLine{
			Description: ln.Description, Quantity: ln.Quantity, UnitOfMeasure: ln.UnitOfMeasure, UnitPrice: ln.UnitPriceNet,
		})
	}
	tc := s.transport
	if tc == nil {
		tc = at.NewTransportFromEnv()
	}
	if _, err := tc.RegisterDocument(ctx, doc); err != nil {
		return mapTransportRegisterError(err)
	}
	return nil
}

// annulMovement is the ONLY GR/GT annulment orchestration. Order: idempotent retry,
// status and movement-start gates, communicateAnnul, then the type's local N→A write.
func (s *FiscalService) annulMovement(ctx context.Context, docType, storeID, requestID, operatorID, documentID, reason string) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if requestID == "" {
		return nil, coded(ErrCodeValidationFailed, "request_id required")
	}
	if operatorID == "" {
		return nil, coded(ErrCodeValidationFailed, "operator_id required")
	}
	if documentID == "" {
		return nil, coded(ErrCodeValidationFailed, "document id required")
	}
	reason, err := domain.ValidateStatusReason(reason)
	if err != nil {
		return nil, err
	}
	if storeID == "" {
		storeID = s.storeID
	}

	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if rec, hit, err := s.db.IssuedByRequestID(storeID, requestID); err != nil {
		return nil, err
	} else if hit && rec.DocumentID == documentID && rec.DocumentStatus == domain.WorkStatusAnnulled {
		return annulResult(rec), nil
	}

	m, err := s.db.LoadCommunicatedMovement(storeID, documentID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "document not found")
	}
	if err != nil {
		return nil, err
	}
	if m.DocumentType != docType {
		return nil, coded(ErrCodeAnnulNotAllowed, docType+" cannot be annulled")
	}
	// Already annulled: the store business key returns the first result. Do not call AT again.
	if m.DocumentStatus == string(domain.WorkStatusAnnulled) {
		rec, err := s.annulMovementLocal(ctx, docType, storeID, requestID, operatorID, documentID, reason)
		if err != nil {
			return nil, err
		}
		return annulResult(rec), nil
	}
	if m.DocumentStatus != string(domain.WorkStatusNormal) {
		return nil, coded(ErrCodeAnnulNotAllowed, docType+" cannot be annulled")
	}
	moveAt := normalizeMovementStart(m.MovementStart)
	if err := assertMovementStartNotPast(moveAt); err != nil {
		return nil, err
	}
	m.MovementStart = moveAt
	if err := s.communicateAnnul(ctx, storeID, m); err != nil {
		return nil, err
	}

	rec, err := s.annulMovementLocal(ctx, docType, storeID, requestID, operatorID, documentID, reason)
	if err != nil {
		return nil, err
	}
	return annulResult(rec), nil
}

func (s *FiscalService) annulMovementLocal(ctx context.Context, docType, storeID, requestID, operatorID, documentID, reason string) (*store.IssueRecord, error) {
	var rec *store.IssueRecord
	var err error
	switch docType {
	case "GR":
		rec, err = s.db.AnnulGR(ctx, store.AnnulGRParams{
			StoreID: storeID, RequestID: requestID, DocumentID: documentID, OperatorID: operatorID, Reason: reason,
		})
	case "GT":
		rec, err = s.db.AnnulGT(ctx, store.AnnulGTParams{
			StoreID: storeID, RequestID: requestID, DocumentID: documentID, OperatorID: operatorID, Reason: reason,
		})
	default:
		return nil, coded(ErrCodeAnnulNotAllowed, "document cannot be annulled")
	}
	if errors.Is(err, store.ErrConflict) {
		return nil, coded(ErrCodeIdempotencyConflict, err.Error())
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "document not found")
	}
	if errors.Is(err, store.ErrDeliveryNotAllowed) || errors.Is(err, store.ErrTransportNotAllowed) {
		return nil, coded(ErrCodeAnnulNotAllowed, docType+" cannot be annulled")
	}
	if err != nil {
		return nil, coded("annul_failed", err.Error())
	}
	return rec, nil
}

func annulResult(rec *store.IssueRecord) *domain.IssueResult {
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}
}

// transportCommitFailed: AT already holds ATDocCodeID for the number but no local document
// was written. Recorded in audit_log (at_transport_unsaved) for reconciliation and surfaced.
func (s *FiscalService) transportCommitFailed(operatorID string, comm *transportComm, err error) error {
	detail, _ := json.Marshal(map[string]string{
		"invoice_no": comm.prep.InvoiceNo, "at_doc_code_id": comm.atRes.ATDocCodeID, "error": err.Error(),
	})
	_ = s.db.InsertAuditLog(operatorID, "at_transport_unsaved", "invoice", comm.prep.InvoiceNo, string(detail))
	return coded("issue_failed", fmt.Sprintf("AT registered %s (ATDocCodeID %s) but local save failed: %v",
		comm.prep.InvoiceNo, comm.atRes.ATDocCodeID, err))
}
