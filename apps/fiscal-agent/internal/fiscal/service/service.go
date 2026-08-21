package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/at"
	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/protect"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/google/uuid"
)

// API error codes (stable).
const (
	ErrCodeTaxpayerMissing = "taxpayer_missing"
	ErrCodeATCredsMissing  = "at_credentials_missing"
	ErrCodeSeriesMissing   = "series_missing"
	ErrCodeSignerNotReady  = "signer_not_ready"
	ErrCodeATSOAPFailed    = "at_soap_failed"
)

// CodedError carries a stable error code for HTTP.
type CodedError struct {
	Code string
	Msg  string
}

func (e *CodedError) Error() string { return e.Msg }

func coded(code, msg string) error { return &CodedError{Code: code, Msg: msg} }

// FiscalService orchestrates setup + issuance.
type FiscalService struct {
	db      *store.DB
	signer  store.Signer
	at      at.Client
	dataDir string
	storeID string
}

// New returns a FiscalService. signer may be nil until Activate; IssueDocument loads from DB if nil.
func New(db *store.DB, sig store.Signer, atClient at.Client, dataDir, storeID string) *FiscalService {
	if atClient == nil {
		atClient = at.NewFromEnv()
	}
	return &FiscalService{db: db, signer: sig, at: atClient, dataDir: dataDir, storeID: storeID}
}

// SetSigner replaces the in-memory signer after activation.
func (s *FiscalService) SetSigner(sig store.Signer) { s.signer = sig }

func (s *FiscalService) ensureSigner() (store.Signer, error) {
	if s.signer != nil {
		return s.signer, nil
	}
	wrapped, _, ver, err := s.db.ActiveSigningKey()
	if err != nil {
		return nil, coded(ErrCodeSignerNotReady, "signing key not activated")
	}
	sig, err := signer.NewUnwrappingSigner(s.dataDir, wrapped, ver)
	if err != nil {
		return nil, coded(ErrCodeSignerNotReady, err.Error())
	}
	s.signer = sig
	return sig, nil
}

// UpsertTaxpayer is the ONLY taxpayer setup entry.
func (s *FiscalService) UpsertTaxpayer(p store.TaxpayerInput) error {
	if p.StoreID == "" {
		p.StoreID = s.storeID
	}
	if p.Country == "" {
		p.Country = "PT"
	}
	if p.Timezone == "" {
		p.Timezone = "Europe/Lisbon"
	}
	if p.SoftwareCertificateNumber == "" {
		p.SoftwareCertificateNumber = "0"
	}
	if p.TaxRegistrationNumber == "" || p.LegalName == "" || p.AddressDetail == "" || p.City == "" || p.PostalCode == "" {
		return fmt.Errorf("fiscal: taxpayer required fields missing")
	}
	if err := s.db.EnsureConsumidorFinal(); err != nil {
		return err
	}
	return s.db.UpsertTaxpayer(p)
}

// UpsertATCredentials seals and stores AT password — ONLY credentials write via service.
func (s *FiscalService) UpsertATCredentials(storeID, username, password string) error {
	if storeID == "" {
		storeID = s.storeID
	}
	if username == "" || password == "" {
		return fmt.Errorf("fiscal: username and password required")
	}
	ct, meta, err := protect.Seal(s.dataDir, []byte(password))
	if err != nil {
		return err
	}
	return s.db.UpsertATCredentials(storeID, username, ct, meta)
}

// RegisterSeries calls AT (or mock) and persists ACTIVE series — ONLY series registration entry.
func (s *FiscalService) RegisterSeries(ctx context.Context, storeID, seriesCode, docType string, fiscalYear int) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if docType == "" {
		docType = "FT"
	}
	if fiscalYear == 0 {
		fiscalYear = time.Now().Year()
	}
	if seriesCode == "" {
		seriesCode = fmt.Sprintf("%s%dDEMO01", docType, fiscalYear)
	}
	st, err := s.db.GetSetupStatus(storeID)
	if err != nil {
		return nil, err
	}
	if !st.TaxpayerOK {
		return nil, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
	}
	cred, err := s.db.GetATCredentials(storeID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeATCredsMissing, "configure AT credentials first")
	}
	if err != nil {
		return nil, err
	}
	pass, err := protect.Open(s.dataDir, cred.PasswordCiphertext, cred.WrapMeta)
	if err != nil {
		return nil, err
	}
	var certStr string
	_ = s.db.SQL.QueryRow(`SELECT software_certificate_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&certStr)
	certN, _ := strconv.Atoi(certStr)

	res, err := s.at.RegisterSeries(ctx, at.RegisterRequest{
		Username: cred.Username, Password: string(pass),
		SeriesCode: seriesCode, DocumentType: docType, FiscalYear: fiscalYear, SoftwareCertNumber: certN,
	})
	if err != nil {
		_ = s.db.MarkATCredentialsError(storeID, err.Error())
		return nil, coded(ErrCodeATSOAPFailed, err.Error())
	}
	if err := s.db.UpsertActiveSeries(storeID, docType, seriesCode, res.ValidationCode, fiscalYear); err != nil {
		return nil, err
	}
	_ = s.db.MarkATCredentialsOK(storeID)
	return s.db.GetSetupStatus(storeID)
}

// ActivateFiscal provisions device key + wraps product PEM — ONLY activation entry.
func (s *FiscalService) ActivateFiscal(storeID, productPEM string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "1" {
		return nil, fmt.Errorf("fiscal: local provision disabled (set FISCAL_ALLOW_LOCAL_PROVISION=1 for M1)")
	}
	productPEM = strings.TrimSpace(productPEM)
	if productPEM == "" {
		return nil, fmt.Errorf("fiscal: product_private_key_pem required")
	}
	var nif string
	err := s.db.SQL.QueryRow(`SELECT tax_registration_number FROM taxpayer_settings WHERE store_id=?`, storeID).Scan(&nif)
	if err != nil {
		return nil, coded(ErrCodeTaxpayerMissing, "configure taxpayer first")
	}

	dev, err := signer.GenerateDeviceKey()
	if err != nil {
		return nil, err
	}
	if err := signer.SaveDeviceKey(s.dataDir, dev); err != nil {
		return nil, err
	}
	wrapped, err := signer.WrapProductPEM(&dev.Private.PublicKey, []byte(productPEM))
	if err != nil {
		return nil, err
	}
	inner, err := signer.LoadPEM([]byte(productPEM), 1)
	if err != nil {
		return nil, fmt.Errorf("fiscal: product PEM: %w", err)
	}
	pub, err := inner.PublicKeyPEM()
	if err != nil {
		return nil, err
	}
	if err := s.db.SaveActivation(store.ActivationInput{
		InstallationID:     "inst-" + uuid.NewString(),
		StoreID:            storeID,
		TaxpayerNIF:        nif,
		DeviceID:           "device-local",
		DevicePublicKey:    dev.PublicPEM,
		KeyProtectionLevel: "SOFTWARE",
		KeyVersion:         1,
		PublicKeyPEM:       pub,
		WrappedPrivateKey:  wrapped,
	}); err != nil {
		return nil, err
	}
	sig, err := signer.NewUnwrappingSigner(s.dataDir, wrapped, 1)
	if err != nil {
		return nil, err
	}
	s.signer = sig
	return s.db.GetSetupStatus(storeID)
}

// UpsertOperator is the ONLY operator setup entry for M1.
func (s *FiscalService) UpsertOperator(id, storeID, role, name string) error {
	if storeID == "" {
		storeID = s.storeID
	}
	if id == "" {
		id = "op-demo-cashier"
	}
	if role == "" {
		role = "cashier"
	}
	if name == "" {
		name = "Cashier"
	}
	return s.db.UpsertOperator(id, storeID, role, name, "")
}

// SetupStatus proxies store.
func (s *FiscalService) SetupStatus(storeID string) (*store.SetupStatus, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	return s.db.GetSetupStatus(storeID)
}

// IssueDocument signs a sale snapshot as FT (P0).
func (s *FiscalService) IssueDocument(ctx context.Context, req domain.IssueRequest, docType domain.DocumentType) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	if req.RequestID == "" {
		return nil, fmt.Errorf("fiscal: request_id required")
	}
	if req.StoreID == "" {
		req.StoreID = s.storeID
	}
	if req.OperatorID == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	if len(req.Snapshot.Lines) == 0 {
		return nil, fmt.Errorf("fiscal: lines required")
	}
	st, err := s.db.GetSetupStatus(req.StoreID)
	if err != nil {
		return nil, err
	}
	if !st.SeriesOK {
		return nil, coded(ErrCodeSeriesMissing, "no ACTIVE FT series with validation_code")
	}
	sig, err := s.ensureSigner()
	if err != nil {
		return nil, err
	}
	rec, err := s.db.IssueFT(ctx, sig, store.IssueParams{
		StoreID: req.StoreID, RequestID: req.RequestID, DocType: docType,
		Snapshot: req.Snapshot, OperatorID: req.OperatorID, StationID: req.StationID,
	})
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: rec.IdempotentHit,
	}, nil
}

// GetByRequestID looks up a prior issue.
func (s *FiscalService) GetByRequestID(storeID, requestID string) (*domain.IssueResult, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	rec, err := s.db.GetByRequestID(storeID, requestID)
	if err != nil {
		return nil, err
	}
	return &domain.IssueResult{
		DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
		DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
		PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
		IssuedAt: rec.IssuedAt, IdempotentHit: true,
	}, nil
}

// GetPrintJob proxies store.
func (s *FiscalService) GetPrintJob(id string) (*store.PrintJobView, error) {
	return s.db.GetPrintJob(id)
}

// ListBillDrafts returns local bill sync drafts (read model).
func (s *FiscalService) ListBillDrafts(limit int) ([]store.BillSyncDraft, error) {
	return s.db.ListBillDrafts(limit)
}

// IssueBillDraftInput is the ONLY input shape for IssueFromBillDraft.
type IssueBillDraftInput struct {
	DraftID      string
	OperatorID   string
	Mode         string // whole_table | person; empty → whole_table
	ScopeID      string
	StationID    string // required: station_printers key for ORIGINAL print
	CustomerNIF  string
	CustomerName string
}

// BillDraftDetail is GET /bill-drafts/{id} read model.
type BillDraftDetail struct {
	Draft        store.BillSyncDraft `json:"draft"`
	Payload      billsync.Snapshot   `json:"payload"`
	IssuedScopes []store.SignedFTScope `json:"issued_scopes"`
}

// GetBillDraftDetail loads one draft + payload + issued scopes (tax DB).
func (s *FiscalService) GetBillDraftDetail(draftID string) (*BillDraftDetail, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	draft, err := s.db.GetBillDraftByID(strings.TrimSpace(draftID))
	if err != nil {
		return nil, err
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &snap); err != nil {
		return nil, fmt.Errorf("fiscal: draft payload: %w", err)
	}
	if strings.TrimSpace(snap.SourceSaleID) == "" {
		snap.SourceSaleID = draft.SourceSaleID
	}
	scopes, err := s.db.ListSignedFTScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
	if err != nil {
		return nil, err
	}
	return &BillDraftDetail{Draft: *draft, Payload: snap, IssuedScopes: scopes}, nil
}

// DiscardBillDrafts is the ONLY user discard entry: hard-delete drafts for the sale; never deletes invoices.
func (s *FiscalService) DiscardBillDrafts(draftID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service not configured")
	}
	draft, err := s.db.GetBillDraftByID(strings.TrimSpace(draftID))
	if err != nil {
		return err
	}
	return s.db.DeleteBillDraftsBySale(draft.SourceSaleID)
}

// IssueFromBillDraft is the ONLY entry that issues FT from a bill_sync_drafts row.
// Maps via billsync.DraftToSaleSnapshot / DraftPartToSaleSnapshot → IssueDocument → DeleteBillDraftsBySale (when due).
func (s *FiscalService) IssueFromBillDraft(ctx context.Context, in IssueBillDraftInput) (*domain.IssueResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service not configured")
	}
	draftID := strings.TrimSpace(in.DraftID)
	operatorID := strings.TrimSpace(in.OperatorID)
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = "whole_table"
	}
	if draftID == "" {
		return nil, fmt.Errorf("fiscal: draft id required")
	}
	if operatorID == "" {
		return nil, fmt.Errorf("fiscal: operator_id required")
	}
	stationID := strings.TrimSpace(in.StationID)
	if stationID == "" {
		return nil, coded("validation_failed", "station_id required")
	}
	if mode != "whole_table" && mode != "person" {
		return nil, coded("validation_failed", fmt.Sprintf("mode %q invalid", mode))
	}

	draft, err := s.db.GetBillDraftByID(draftID)
	if err != nil {
		return nil, err
	}
	if draft.Status != store.BillDraftOpen {
		return nil, coded("draft_not_open", fmt.Sprintf("draft status %q is not open", draft.Status))
	}
	var snap billsync.Snapshot
	if err := json.Unmarshal([]byte(draft.PayloadJSON), &snap); err != nil {
		return nil, fmt.Errorf("fiscal: draft payload: %w", err)
	}
	if strings.TrimSpace(snap.SourceSaleID) == "" {
		snap.SourceSaleID = draft.SourceSaleID
	}
	if strings.TrimSpace(snap.RequestID) == "" {
		snap.RequestID = draft.RequestID
	}
	if strings.TrimSpace(snap.SourceSystem) == "" {
		snap.SourceSystem = "farvoo"
	}

	payloadType := strings.TrimSpace(snap.ScopeType)

	existing, err := s.db.ListSignedFTScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
	if err != nil {
		return nil, err
	}
	if err := checkScopeMutex(mode, existing); err != nil {
		return nil, err
	}

	// Already-issued scope → return original (NIF changes must not conflict).
	if hit := findIssuedScope(mode, strings.TrimSpace(in.ScopeID), draft.SourceSaleID, existing); hit != nil {
		rec, err := s.db.GetIssueRecordByID(hit.DocumentID)
		if err != nil {
			return nil, err
		}
		return &domain.IssueResult{
			DocumentID: rec.DocumentID, InvoiceNo: rec.InvoiceNo, ATCUD: rec.ATCUD,
			DocumentType: rec.DocumentType, DocumentStatus: rec.DocumentStatus,
			PrintJobID: rec.PrintJobID, PrintStatus: rec.PrintStatus,
			IssuedAt: rec.IssuedAt, IdempotentHit: true,
		}, nil
	}

	switch mode {
	case "whole_table":
		if payloadType != "whole_table" {
			return nil, coded("validation_failed", fmt.Sprintf("mode=whole_table requires payload whole_table (got %q)", payloadType))
		}
	case "person":
		if payloadType != "split" {
			return nil, coded("validation_failed", fmt.Sprintf("mode=person requires payload split (got %q)", payloadType))
		}
	}

	var sale domain.SaleSnapshot
	var reqID string
	switch mode {
	case "whole_table":
		sale, err = billsync.DraftToSaleSnapshot(snap)
		reqID = "draft-issue:" + draft.ID + ":whole_table"
	case "person":
		sale, err = billsync.DraftPartToSaleSnapshot(snap, in.ScopeID)
		reqID = "draft-issue:" + draft.ID + ":person:" + strings.TrimSpace(in.ScopeID)
	}
	if err != nil {
		return nil, err
	}
	if err := billsync.ApplyCustomerOverride(&sale, in.CustomerNIF, in.CustomerName); err != nil {
		return nil, err
	}

	res, err := s.IssueDocument(ctx, domain.IssueRequest{
		StoreID: s.storeID, RequestID: reqID, OperatorID: operatorID, StationID: stationID, Snapshot: sale,
	}, domain.DocumentFT)
	if err != nil {
		return nil, err
	}

	shouldDelete := mode == "whole_table"
	if mode == "person" {
		after, err := s.db.ListSignedFTScopesForSale(s.storeID, snap.SourceSystem, draft.SourceSaleID)
		if err != nil {
			return nil, err
		}
		shouldDelete = allSplitScopesIssued(snap, after)
	}
	if shouldDelete {
		if err := s.db.DeleteBillDraftsBySale(draft.SourceSaleID); err != nil {
			res.CleanupPending = true
			// Ticket already signed — do not fail the client.
			return res, nil
		}
	}
	return res, nil
}

func checkScopeMutex(mode string, existing []store.SignedFTScope) error {
	hasWhole, hasPerson := false, false
	for _, sc := range existing {
		switch strings.TrimSpace(sc.ScopeType) {
		case "whole_table":
			hasWhole = true
		case "person":
			hasPerson = true
		}
	}
	if mode == "person" && hasWhole {
		return coded("scope_mutex", "whole_table FT already exists for this sale")
	}
	if mode == "whole_table" && hasPerson {
		return coded("scope_mutex", "person FT already exists for this sale")
	}
	return nil
}

func findIssuedScope(mode, scopeID, saleID string, existing []store.SignedFTScope) *store.SignedFTScope {
	wantType := "whole_table"
	wantID := saleID
	if mode == "person" {
		wantType = "person"
		wantID = scopeID
	}
	for i := range existing {
		if strings.TrimSpace(existing[i].ScopeType) == wantType && strings.TrimSpace(existing[i].ScopeID) == wantID {
			return &existing[i]
		}
	}
	return nil
}

func allSplitScopesIssued(snap billsync.Snapshot, scopes []store.SignedFTScope) bool {
	if len(snap.Splits) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, sc := range scopes {
		if strings.TrimSpace(sc.ScopeType) == "person" {
			have[strings.TrimSpace(sc.ScopeID)] = true
		}
	}
	for _, sp := range snap.Splits {
		id := strings.TrimSpace(sp.ScopeID)
		if id == "" || !have[id] {
			return false
		}
	}
	return true
}

// DB exposes store for bill-sync puller wiring (read/ingest only via billsync package).
func (s *FiscalService) DB() *store.DB { return s.db }
