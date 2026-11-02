package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/protect"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

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

// communicateTransport is the ONLY GR/GT → AT step: reserve the real next number
// (PrepareTransport), then envioDocumentoTransporte with it. Caller must hold
// s.transportMu until store.IssueGR/IssueGT (ExpectedSeq) commits.
func (s *FiscalService) communicateTransport(ctx context.Context, storeID, docType string,
	snap domain.SaleSnapshot, shipFrom, shipTo, moveAt string) (*transportComm, error) {
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
		CustomerAddress: at.TransportAddress{
			Detail: strings.TrimSpace(snap.Customer.AddressDetail), City: strings.TrimSpace(snap.Customer.City),
			PostalCode: strings.TrimSpace(snap.Customer.PostalCode),
		},
		ShipFromAddress: shipFrom,
		ShipToAddress:   shipTo,
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
		msg := err.Error()
		if strings.Contains(msg, "not wired") || strings.Contains(msg, "unavailable") {
			return nil, coded(ErrCodeATTransportUnavailable, msg)
		}
		return nil, coded(ErrCodeATTransportRejected, msg)
	}
	if atRes == nil || strings.TrimSpace(atRes.ATDocCodeID) == "" {
		return nil, coded(ErrCodeATTransportRejected, "empty ATDocCodeID")
	}
	return &transportComm{prep: prep, atRes: atRes}, nil
}

// transportCommitFailed: AT already holds ATDocCodeID for invoiceNo but no local document
// was written — surface both so the operator can reconcile (never silent).
func transportCommitFailed(comm *transportComm, err error) error {
	return coded("issue_failed", "AT registered "+comm.prep.InvoiceNo+" (ATDocCodeID "+
		comm.atRes.ATDocCodeID+") but local save failed: "+err.Error())
}
