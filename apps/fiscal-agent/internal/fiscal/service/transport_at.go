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
	return from.Normalize(), to.Normalize(), nil
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

// communicateTransport is the ONLY GR/GT → AT step: reserve the real next number
// (PrepareTransport), then envioDocumentoTransporte with it. Caller must hold
// s.transportMu until store.IssueGR/IssueGT (ExpectedSeq) commits.
func (s *FiscalService) communicateTransport(ctx context.Context, storeID, docType string,
	snap domain.SaleSnapshot, shipFrom, shipTo domain.Address, moveAt string) (*transportComm, error) {
	prep, err := s.db.PrepareTransport(storeID, docType, snap)
	if errors.Is(err, store.ErrGRSeriesMissing) || errors.Is(err, store.ErrGTSeriesMissing) {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE "+docType+" series with validation_code")
	}
	if err != nil {
		return nil, coded(ErrCodeValidationFailed, err.Error())
	}
	var user, pass string
	if cred, err := s.db.GetATCredentials(storeID); err == nil {
		p, err := protect.Open(s.dataDir, cred.PasswordCiphertext, cred.WrapMeta)
		if err != nil {
			return nil, err
		}
		user, pass = cred.Username, string(p)
	} else if !errors.Is(err, store.ErrNotFound) {
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
