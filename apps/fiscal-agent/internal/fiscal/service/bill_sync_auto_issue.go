package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// BillSyncIngestResult is the Puller/ack outcome for one cloud bill_sync job.
type BillSyncIngestResult struct {
	Draft *store.BillSyncDraft
	Issue *domain.IssueResult // set when auto_issue succeeded (or idempotent hit)
}

// AutoIssueContext is Agent-local issuer identity for background Farvoo auto_issue (no Admin session).
type AutoIssueContext struct {
	OperatorID          string
	StationID           string
	FiscalTerminalID    string
	FiscalTerminalLabel string
}

// ResolveAutoIssueContext is the ONLY resolver for background auto_issue operator/station/terminal.
// Station = local_default_station_id; operator = first active login operator (role order).
func (s *FiscalService) ResolveAutoIssueContext() (AutoIssueContext, error) {
	if s == nil || s.db == nil {
		return AutoIssueContext{}, fmt.Errorf("fiscal: service not configured")
	}
	station, err := s.db.GetLocalDefaultStation(s.storeID)
	if err != nil {
		return AutoIssueContext{}, err
	}
	station = strings.TrimSpace(station)
	if station == "" {
		return AutoIssueContext{}, coded("station_required", "set a default print station first")
	}
	ops, err := s.db.ListOperatorsForLogin(s.storeID)
	if err != nil {
		return AutoIssueContext{}, err
	}
	if len(ops) == 0 {
		return AutoIssueContext{}, coded("operator_required", "no active operator for auto_issue")
	}
	return AutoIssueContext{
		OperatorID:          ops[0].ID,
		StationID:           station,
		FiscalTerminalID:    domain.LoopbackFiscalTerminalID,
		FiscalTerminalLabel: "127.0.0.1",
	}, nil
}

// IngestBillSyncJob is the ONLY service entry for PullAndIngest: persist draft, optionally auto-issue, or reprint.
// When reprint_document_id is set: only ReprintDocument (existing path; no re-sign).
// When auto_issue=true: uses Farvoo document_type as-is (required FT|FS); empty customer_nif → scatter via ApplyCustomerOverride.
// Person auto_issue after partial issue reuses open draft (does not re-Upsert / overwrite allocation).
func (s *FiscalService) IngestBillSyncJob(ctx context.Context, job billsync.CloudJob) (*BillSyncIngestResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal(job.Payload, &snap); err != nil {
		return nil, billsync.NewIngestError(billsync.CodeValidationFailed, "payload json: "+err.Error())
	}

	if reprintID := strings.TrimSpace(snap.ReprintDocumentID); reprintID != "" {
		aic, err := s.ResolveAutoIssueContext()
		if err != nil {
			return nil, mapAutoIssueErr(err)
		}
		if _, err := s.ReprintDocument(ctx, reprintID, aic.OperatorID, aic.StationID); err != nil {
			return nil, mapAutoIssueErr(err)
		}
		rec, err := s.db.GetIssueRecordByID(reprintID)
		if err != nil {
			return nil, billsync.NewIngestError(billsync.CodePersistFailed, err.Error())
		}
		return &BillSyncIngestResult{
			Issue: &domain.IssueResult{
				DocumentID:     rec.DocumentID,
				InvoiceNo:      rec.InvoiceNo,
				ATCUD:          rec.ATCUD,
				DocumentType:   rec.DocumentType,
				DocumentStatus: rec.DocumentStatus,
				PrintJobID:     rec.PrintJobID,
				PrintStatus:    rec.PrintStatus,
				IssuedAt:       rec.IssuedAt,
			},
		}, nil
	}

	if !snap.AutoIssue {
		draft, err := billsync.IngestCloudJob(s.db, job)
		if err != nil {
			return nil, err
		}
		return &BillSyncIngestResult{Draft: draft}, nil
	}

	mode, scopeID, err := billsync.ResolveAutoIssueMode(snap)
	if err != nil {
		return nil, err
	}
	docType := strings.TrimSpace(snap.DocumentType)
	if docType == "" {
		return nil, billsync.NewIngestError(billsync.CodeValidationFailed, "document_type required for auto_issue")
	}
	if _, err := ResolveBillSyncDocumentType(docType); err != nil {
		return nil, billsync.NewIngestError(billsync.CodeValidationFailed, err.Error())
	}

	aic, err := s.ResolveAutoIssueContext()
	if err != nil {
		return nil, mapAutoIssueErr(err)
	}

	draft, prior, err := s.resolveDraftForAutoIssue(job, snap, mode, scopeID)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		return &BillSyncIngestResult{Issue: prior}, nil
	}

	rev := draft.AllocationRevision
	in := IssueBillDraftInput{
		DraftID:             draft.ID,
		DocumentType:        docType,
		OperatorID:          aic.OperatorID,
		Mode:                mode,
		ScopeID:             scopeID,
		StationID:           aic.StationID,
		FiscalTerminalID:    aic.FiscalTerminalID,
		FiscalTerminalLabel: aic.FiscalTerminalLabel,
		CustomerNIF:         snap.CustomerNIF,
		CustomerName:        snap.CustomerName,
		PaymentMethod:       snap.PaymentMethod,
	}
	if mode == "person" {
		in.AllocationRevision = &rev
	}
	res, err := s.IssueFromBillDraft(ctx, in)
	if err != nil {
		return nil, mapAutoIssueErr(err)
	}
	return &BillSyncIngestResult{Draft: draft, Issue: res}, nil
}

// ProcessBillSyncJob adapts IngestBillSyncJob for billsync.Puller.ProcessJob (ack invoice_no + document_id).
func (s *FiscalService) ProcessBillSyncJob(ctx context.Context, job billsync.CloudJob) (invoiceNo, documentID string, err error) {
	out, err := s.IngestBillSyncJob(ctx, job)
	if err != nil {
		return "", "", err
	}
	if out != nil && out.Issue != nil {
		return out.Issue.InvoiceNo, out.Issue.DocumentID, nil
	}
	return "", "", nil
}

func (s *FiscalService) resolveDraftForAutoIssue(job billsync.CloudJob, snap billsync.Snapshot, mode, scopeID string) (*store.BillSyncDraft, *domain.IssueResult, error) {
	saleID := strings.TrimSpace(snap.SourceSaleID)
	if open, err := s.db.GetOpenBillDraftBySale(saleID); err == nil && open != nil {
		// Partial person path: keep allocation; do not UpsertBillDraftOpen (would hit already_invoiced).
		return open, nil, nil
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, nil, billsync.NewIngestError(billsync.CodePersistFailed, err.Error())
	}

	scopes, err := s.db.ListSignedSaleScopesForSale(s.storeID, "farvoo", saleID)
	if err != nil {
		return nil, nil, billsync.NewIngestError(billsync.CodePersistFailed, err.Error())
	}
	if len(scopes) > 0 {
		if hit := findIssuedScope(mode, scopeID, saleID, scopes); hit != nil {
			rec, err := s.db.GetIssueRecordByID(hit.DocumentID)
			if err != nil {
				return nil, nil, billsync.NewIngestError(billsync.CodePersistFailed, err.Error())
			}
			return nil, &domain.IssueResult{
				DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
				DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
				PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
				IssuedAt: rec.IssuedAt, IdempotentHit: true,
			}, nil
		}
		return nil, nil, billsync.NewIngestError(billsync.CodeAlreadyInvoiced, "bill already invoiced; use Agent reprint/NC")
	}

	draft, err := billsync.IngestCloudJob(s.db, job)
	if err != nil {
		return nil, nil, err
	}
	return draft, nil, nil
}

func mapAutoIssueErr(err error) error {
	if err == nil {
		return nil
	}
	if ie := billsync.AsIngestError(err); ie != nil {
		return err
	}
	var ce *CodedError
	if errors.As(err, &ce) {
		return billsync.NewIngestError(ce.Code, ce.Msg)
	}
	if errors.Is(err, store.ErrNotFound) {
		return billsync.NewIngestError("draft_not_found", err.Error())
	}
	return billsync.NewIngestError(billsync.CodePersistFailed, err.Error())
}
